package interview

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

func testAnswerProfile(baseURL string) config.AnswerModelConfig {
	return config.AnswerModelConfig{
		APIKey: "test-key", Model: "test-model", BaseURL: baseURL,
		Protocol: "openai_chat_completions", ThinkingMode: "disabled",
	}
}

// hangServer returns a server whose SSE response opens and then never sends
// any data line, simulating a model queue or proxy that accepts the request
// and stalls forever.
func hangServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
}

func TestHTTPExecutorFirstTokenWatchdogReturns(t *testing.T) {
	server := hangServer()
	defer server.Close()
	executor := &HTTPAnswerExecutor{
		Client: server.Client(), FirstTokenTimeout: 250 * time.Millisecond,
		IdleTimeout: time.Second, TotalTimeout: 5 * time.Second,
	}
	start := time.Now()
	_, err := executor.Stream(context.Background(), testAnswerProfile(server.URL), "问题", nil)
	if err == nil {
		t.Fatal("hanging first token must produce an error")
	}
	if !strings.Contains(err.Error(), "首 Token 超时") {
		t.Fatalf("expected first-token timeout error, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("watchdog fired too late: %v", elapsed)
	}
}

func TestHTTPExecutorIdleWatchdogReturns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"先给结论\"}}]}\n\n"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	executor := &HTTPAnswerExecutor{
		Client: server.Client(), FirstTokenTimeout: 5 * time.Second,
		IdleTimeout: 250 * time.Millisecond, TotalTimeout: 5 * time.Second,
	}
	_, err := executor.Stream(context.Background(), testAnswerProfile(server.URL), "问题", nil)
	if err == nil || !strings.Contains(err.Error(), "空闲超时") {
		t.Fatalf("expected idle timeout error, got %v", err)
	}
}

func TestHTTPExecutorTotalTimeoutReturns(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(50 * time.Millisecond):
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"继续\"}}]}\n\n"))
				flusher.Flush()
			}
		}
	}))
	defer server.Close()
	executor := &HTTPAnswerExecutor{
		Client: server.Client(), FirstTokenTimeout: 2 * time.Second,
		IdleTimeout: time.Second, TotalTimeout: 300 * time.Millisecond,
	}
	_, err := executor.Stream(context.Background(), testAnswerProfile(server.URL), "问题", nil)
	if err == nil || !strings.Contains(err.Error(), "总回答超时") {
		t.Fatalf("expected total timeout error, got %v", err)
	}
}

func TestHTTPExecutorUserCancelStaysContextCanceled(t *testing.T) {
	server := hangServer()
	defer server.Close()
	executor := &HTTPAnswerExecutor{
		Client: server.Client(), FirstTokenTimeout: 5 * time.Second,
		IdleTimeout: 5 * time.Second, TotalTimeout: 10 * time.Second,
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	_, err := executor.Stream(ctx, testAnswerProfile(server.URL), "问题", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("user cancellation must stay context.Canceled, got %v", err)
	}
}

func TestHTTPExecutorHealthyStreamRecordsTiming(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"缓存穿透是\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"查询不存在的数据\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	executor := &HTTPAnswerExecutor{Client: server.Client()}
	var timing AnswerTiming
	timingCalled := false
	executor.OnTiming = func(t AnswerTiming) {
		timing = t
		timingCalled = true
	}
	answer, err := executor.Stream(context.Background(), testAnswerProfile(server.URL), "问题", nil)
	if err != nil {
		t.Fatalf("healthy stream failed: %v", err)
	}
	if answer != "缓存穿透是查询不存在的数据" {
		t.Fatalf("unexpected answer: %q", answer)
	}
	if !timingCalled || timing.ChunkCount != 2 || timing.TotalBytes == 0 {
		t.Fatalf("timing not recorded: called=%v %+v", timingCalled, timing)
	}
}

// TestCoordinatorRecoversWhenBothLanesHang reproduces the core reliability
// failure mode: the first two model requests hang forever, the third and
// fourth questions queue, and the watchdog must free the lanes so the newest
// pending question starts first while the older one still runs afterwards.
func TestCoordinatorRecoversWhenBothLanesHang(t *testing.T) {
	var entered int32
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		if atomic.AddInt32(&entered, 1) <= 2 {
			// The first two requests hang forever unless released.
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"恢复\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	defer close(release)
	executor := &HTTPAnswerExecutor{
		Client: server.Client(), FirstTokenTimeout: 300 * time.Millisecond,
		IdleTimeout: 300 * time.Millisecond, TotalTimeout: 5 * time.Second,
	}
	done := make(chan completionResult, 4)
	coordinator := NewAnswerCoordinator(executor, nil, nil, nil, func(question Question, _ string, err error) {
		done <- completionResult{id: question.QuestionID, err: err}
	})
	defer coordinator.Close()

	for _, id := range []string{"q1", "q2", "q3", "q4"} {
		coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: id}, id, testAnswerProfile(server.URL))
	}

	// Both lanes hang; q3/q4 must wait in the queue, not start immediately.
	time.Sleep(150 * time.Millisecond)
	if enteredCount := atomic.LoadInt32(&entered); enteredCount != 2 {
		t.Fatalf("expected exactly two stuck active requests, got %d", enteredCount)
	}

	// The watchdog must error q1/q2 out and promote q4 before q3.
	results := map[string]error{}
	for range 4 {
		select {
		case result := <-done:
			results[result.id] = result.err
		case <-time.After(8 * time.Second):
			t.Fatalf("answers did not reach terminal state: %+v", results)
		}
	}
	if results["q1"] == nil || results["q2"] == nil {
		t.Fatalf("hung requests must fail with a timeout error: %+v", results)
	}
	if results["q3"] != nil || results["q4"] != nil {
		t.Fatalf("queued questions must still complete after lanes recover: %+v", results)
	}
	coordinator.Wait()
}

func TestLatencyTrackerComputesEndToEndTotals(t *testing.T) {
	tracker := NewLatencyTracker()
	asrAt := time.Now().Add(-2 * time.Second)
	record := &QuestionLatency{
		SessionID: "s1", QuestionID: "q1", Question: "Redis呢？",
		ASRAt: asrAt, ReadyAt: asrAt.Add(700 * time.Millisecond), SubmitAt: asrAt.Add(720 * time.Millisecond),
		RetrievalMS: 40, ContextBuildMS: 1,
	}
	tracker.Register(record)
	tracker.MarkQueueTiming("q1", JobTiming{QueueWaitMS: 15, ActiveMS: 1800})
	tracker.MarkAnswerChunk("q1", asrAt.Add(time.Second))
	tracker.MarkAnswerChunk("q1", asrAt.Add(1200*time.Millisecond))
	finished, ok := tracker.Finish("q1", LatencyOutcomeCompleted)
	if !ok {
		t.Fatal("record must exist")
	}
	if finished.AggregateWaitMS != 700 {
		t.Fatalf("aggregate wait: %d", finished.AggregateWaitMS)
	}
	if finished.QueueWaitMS != 15 || finished.ActiveMS != 1800 {
		t.Fatalf("queue timing: %+v", finished)
	}
	if finished.ChunkCount != 2 || finished.SubmitToFirstChunkMS <= 0 {
		t.Fatalf("chunk metrics: %+v", finished)
	}
	if finished.Outcome != LatencyOutcomeCompleted || finished.TotalMS < 2000 {
		t.Fatalf("total/outcome: %+v", finished)
	}
	if _, ok := tracker.Finish("q1", LatencyOutcomeCompleted); ok {
		t.Fatal("record must be removed after finish")
	}
}

func TestRetrievalLoggerAsyncNeverBlocksAndWrites(t *testing.T) {
	path := t.TempDir() + "/retrieval.log"
	logger := newRetrievalLoggerAtPath(path)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			logger.LogPipeline("测试阶段", "测试问题", "详情")
		}
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("logging blocked the caller")
	}
	logger.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("log file missing: %v", err)
	}
	if !strings.Contains(string(data), "测试阶段") {
		t.Fatalf("log content missing: %q", string(data))
	}
}
