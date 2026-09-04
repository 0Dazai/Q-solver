package interview

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

type controlledAnswerExecutor struct {
	mu       sync.Mutex
	started  []string
	canceled chan string
	releases chan answerRelease
}

type answerRelease struct {
	question string
	done     chan struct{}
}

func (e *controlledAnswerExecutor) Stream(ctx context.Context, _ config.AnswerModelConfig, question string, _ func(string)) (string, error) {
	release := answerRelease{question: question, done: make(chan struct{})}
	e.mu.Lock()
	e.started = append(e.started, question)
	e.mu.Unlock()
	e.releases <- release
	select {
	case <-ctx.Done():
		e.canceled <- question
		return "", ctx.Err()
	case <-release.done:
		return "answer:" + question, nil
	}
}

func (e *controlledAnswerExecutor) Started() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.started...)
}

func newControlledExecutor() *controlledAnswerExecutor {
	return &controlledAnswerExecutor{
		canceled: make(chan string, 8),
		releases: make(chan answerRelease, 8),
	}
}

type completionResult struct {
	id  string
	err error
}

func TestAnswerCoordinatorStartsNewQuestionWithoutCancelingOldAnswer(t *testing.T) {
	executor := newControlledExecutor()
	done := make(chan completionResult, 2)
	coordinator := NewAnswerCoordinator(executor, nil, nil, nil, func(question Question, _ string, err error) {
		done <- completionResult{id: question.QuestionID, err: err}
	})
	defer coordinator.Close()

	q1 := Question{QuestionID: "q1", Submitted: true}
	q2 := Question{QuestionID: "q2", Submitted: true}
	if state := coordinator.SubmitWithProfile(context.Background(), q1, "q1", config.AnswerModelConfig{}); state != AnswerStarted {
		t.Fatalf("first state=%q", state)
	}
	first := <-executor.releases
	if state := coordinator.SubmitWithProfile(context.Background(), q2, "q2", config.AnswerModelConfig{}); state != AnswerStarted {
		t.Fatalf("new question should start in second lane, state=%q", state)
	}
	second := <-executor.releases

	select {
	case canceled := <-executor.canceled:
		t.Fatalf("old answer was canceled by new question: %q", canceled)
	case <-time.After(80 * time.Millisecond):
	}
	close(second.done)
	close(first.done)
	got := map[string]bool{}
	for range 2 {
		result := <-done
		if result.err != nil {
			t.Fatalf("unexpected completion error: %+v", result)
		}
		got[result.id] = true
	}
	if !got["q1"] || !got["q2"] {
		t.Fatalf("both old and new answers must complete: %+v", got)
	}
	coordinator.Wait()
}

func TestAnswerCoordinatorPrioritizesNewestQueuedButKeepsAllQuestions(t *testing.T) {
	executor := newControlledExecutor()
	done := make(chan completionResult, 4)
	coordinator := NewAnswerCoordinator(executor, nil, nil, nil, func(question Question, _ string, err error) {
		done <- completionResult{id: question.QuestionID, err: err}
	})
	defer coordinator.Close()

	for _, id := range []string{"q1", "q2", "q3", "q4"} {
		coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: id}, id, config.AnswerModelConfig{})
	}
	first := <-executor.releases
	second := <-executor.releases
	if started := executor.Started(); len(started) != 2 {
		t.Fatalf("expected two active requests, got %+v", started)
	}

	close(first.done)
	third := <-executor.releases
	if third.question != "q4" {
		t.Fatalf("newest queued question should be promoted first, got %q", third.question)
	}
	close(second.done)
	fourth := <-executor.releases
	if fourth.question != "q3" {
		t.Fatalf("older queued question must still run, got %q", fourth.question)
	}
	close(third.done)
	close(fourth.done)

	completed := map[string]bool{}
	for range 4 {
		result := <-done
		if result.err != nil {
			t.Fatalf("unexpected completion error: %+v", result)
		}
		completed[result.id] = true
	}
	for _, id := range []string{"q1", "q2", "q3", "q4"} {
		if !completed[id] {
			t.Fatalf("question %s did not complete: %+v", id, completed)
		}
	}
	coordinator.Wait()
}

func TestAnswerCoordinatorCancelNewestActiveAndContinueQueue(t *testing.T) {
	executor := newControlledExecutor()
	done := make(chan completionResult, 3)
	coordinator := NewAnswerCoordinator(executor, nil, nil, nil, func(question Question, _ string, err error) {
		done <- completionResult{id: question.QuestionID, err: err}
	})
	defer coordinator.Close()

	coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: "q1"}, "q1", config.AnswerModelConfig{})
	first := <-executor.releases
	coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: "q2"}, "q2", config.AnswerModelConfig{})
	<-executor.releases
	coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: "q3"}, "q3", config.AnswerModelConfig{})
	coordinator.Cancel()

	canceled := <-done
	if canceled.id != "q2" || !errors.Is(canceled.err, context.Canceled) {
		t.Fatalf("expected newest active q2 canceled, got %+v", canceled)
	}
	third := <-executor.releases
	if third.question != "q3" {
		t.Fatalf("queued q3 should start after cancel, got %q", third.question)
	}
	close(first.done)
	close(third.done)
	for range 2 {
		if result := <-done; result.err != nil {
			t.Fatalf("remaining answer failed: %+v", result)
		}
	}
	coordinator.Wait()
}

func TestAnswerCoordinatorDoesNotLoseSubmissionFromCompletionCallback(t *testing.T) {
	executor := newControlledExecutor()
	done := make(chan completionResult, 2)
	var coordinator *AnswerCoordinator
	coordinator = NewAnswerCoordinator(executor, nil, nil, nil, func(question Question, _ string, err error) {
		done <- completionResult{id: question.QuestionID, err: err}
		if question.QuestionID == "q1" {
			coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: "q2"}, "q2", config.AnswerModelConfig{})
		}
	})
	defer coordinator.Close()

	coordinator.SubmitWithProfile(context.Background(), Question{QuestionID: "q1"}, "q1", config.AnswerModelConfig{})
	first := <-executor.releases
	close(first.done)
	if result := <-done; result.id != "q1" || result.err != nil {
		t.Fatalf("unexpected first completion: %+v", result)
	}
	second := <-executor.releases
	close(second.done)
	if result := <-done; result.id != "q2" || result.err != nil {
		t.Fatalf("submission during completion was lost: %+v", result)
	}
	coordinator.Wait()
}

func TestAnswerCoordinatorNewestPriorityStillAgesOldQuestions(t *testing.T) {
	coordinator := NewAnswerCoordinator(nil, nil, nil, nil, nil)
	coordinator.pending = []*answerJob{{id: 3}, {id: 4}, {id: 5}, {id: 6}}
	coordinator.mu.Lock()
	order := []uint64{
		coordinator.popNewestPendingLocked().id,
		coordinator.popNewestPendingLocked().id,
		coordinator.popNewestPendingLocked().id,
		coordinator.popNewestPendingLocked().id,
	}
	coordinator.mu.Unlock()
	want := []uint64{6, 5, 3, 4}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("unexpected priority order: got %+v want %+v", order, want)
		}
	}
}
