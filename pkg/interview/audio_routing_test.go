package interview

import (
	"context"
	"strings"
	"testing"
	"time"

	"Q-Solver/pkg/config"
	"Q-Solver/pkg/interviewhistory"
)

type recordingAnswerExecutor struct {
	inputs chan string
}

type recordingHistory struct {
	messages chan interviewhistory.Message
}

func (r *recordingHistory) StartSession(interviewhistory.Session) error { return nil }
func (r *recordingHistory) FinishSession(string, time.Time) error       { return nil }
func (r *recordingHistory) UpsertMessage(message interviewhistory.Message) error {
	r.messages <- message
	return nil
}

func (e *recordingAnswerExecutor) Stream(_ context.Context, _ config.AnswerModelConfig, input string, _ func(string)) (string, error) {
	e.inputs <- input
	return "回答", nil
}

func TestLocalCandidateSpeechIsContextOnlyAndDoesNotTriggerAnswer(t *testing.T) {
	executor := &recordingAnswerExecutor{inputs: make(chan string, 2)}
	manager := NewManager(func() config.Config { return config.Config{} }, func(string, ...any) {}, executor)
	manager.retrievalLog = nil
	manager.ctx = context.Background()
	manager.status = Status{SessionID: "session-1"}
	manager.activeTurnID = "turn-0"
	manager.activeQuestionID = "question-0"
	manager.candidateAnswers = make(map[string]string)

	manager.acceptLocal(TranscriptEvent{Kind: EventFinal, Text: "我负责过订单系统的重构"})

	select {
	case <-executor.inputs:
		t.Fatal("本机麦克风转写不应直接触发大模型回答")
	case <-time.After(50 * time.Millisecond):
	}

	manager.submit(Question{
		SessionID: "session-1", TurnID: "turn-1", QuestionID: "question-1",
		CorrectedText: "请介绍一下相关项目经验？", Submitted: true,
	})

	select {
	case input := <-executor.inputs:
		if !strings.Contains(input, "我负责过订单系统的重构") {
			t.Fatalf("后续面试官问题应携带求职者上一轮回答作为上下文，实际输入：%q", input)
		}
	case <-time.After(time.Second):
		t.Fatal("面试官问题应触发大模型回答")
	}
	manager.answers.Wait()
}

func TestSubmitStampsManualQuestionWithActiveSession(t *testing.T) {
	executor := &recordingAnswerExecutor{inputs: make(chan string, 1)}
	history := &recordingHistory{messages: make(chan interviewhistory.Message, 2)}
	manager := NewManager(func() config.Config { return config.Config{} }, func(string, ...any) {}, executor)
	manager.retrievalLog = nil
	manager.ctx = context.Background()
	manager.status = Status{SessionID: "session-current"}
	manager.historyRecorder = history

	manager.submit(Question{
		TurnID: "turn-1", QuestionID: "question-1",
		CorrectedText: "请介绍一下自己？", Submitted: true,
	})

	message := <-history.messages
	if message.SessionID != "session-current" {
		t.Fatalf("手动提交也必须使用活动会话 ID，实际为 %q", message.SessionID)
	}
	manager.answers.Wait()
}
