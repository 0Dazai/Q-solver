package interview

import (
	"strings"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

func TestAggregatorMergesFinalsAndSubmitsOnce(t *testing.T) {
	now := time.Now()
	a := NewAggregator(time.Second)
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "介绍一下 Redis 缓存", Timestamp: now})
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "介绍一下 Redis 缓存穿透，", Timestamp: now})
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "然后说说它和缓存击穿的区别。", Timestamp: now.Add(200 * time.Millisecond)})
	question, reason, ready := a.Ready(now.Add(1500 * time.Millisecond))
	if !ready || reason != "" {
		t.Fatalf("expected stable question, ready=%v reason=%q", ready, reason)
	}
	if question.CorrectedText != "介绍一下 Redis 缓存穿透，然后说说它和缓存击穿的区别。" {
		t.Fatalf("unexpected merge: %q", question.CorrectedText)
	}
	if _, _, ready = a.Ready(now.Add(3 * time.Second)); ready {
		t.Fatal("same turn must not submit twice")
	}
}

func TestAggregatorPreservesSessionIDUntilAutomaticSubmit(t *testing.T) {
	a := NewAggregator(300 * time.Millisecond)
	now := time.Now()
	a.Accept(TranscriptEvent{
		SessionID: "session-current", Kind: EventFinal,
		Text: "请做一个自我介绍？", Timestamp: now,
	})

	question, _, ready := a.Ready(now.Add(time.Second))
	if !ready {
		t.Fatal("完整问题应进入自动提交")
	}
	if question.SessionID != "session-current" {
		t.Fatalf("自动提交必须保留会话 ID，实际为 %q", question.SessionID)
	}
}

func TestAdaptiveWaitUsesFastBoundaryForCompleteQuestion(t *testing.T) {
	if got := adaptiveWait("你为什么选择这个岗位？", 1300*time.Millisecond); got != 700*time.Millisecond {
		t.Fatalf("question-mark merge window should be 700ms, got %v", got)
	}
	if got := adaptiveWait("你为什么选择这个岗位?", 1300*time.Millisecond); got != 700*time.Millisecond {
		t.Fatalf("ascii question-mark merge window should be 700ms, got %v", got)
	}
	if got := adaptiveWait("然后在这个项目里面你主要", 1300*time.Millisecond); got < 800*time.Millisecond {
		t.Fatalf("unfinished wait=%v", got)
	}
}

func TestAggregatorQuestionMarkSubmitsAt700msBoundary(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1300 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "请介绍一下你的项目经历？", Timestamp: now})

	// 690ms: still within the merge window, must not submit.
	if _, reason, ready := a.Ready(now.Add(690 * time.Millisecond)); ready {
		t.Fatal("question must not submit before 700ms boundary")
	} else if reason != "等待句末稳定" {
		t.Fatalf("expected '等待句末稳定' at 690ms, got %q", reason)
	}

	// 710ms: past the merge window, should submit.
	question, reason, ready := a.Ready(now.Add(710 * time.Millisecond))
	if !ready {
		t.Fatalf("question should submit after 700ms, ready=%v reason=%q", ready, reason)
	}
	if question.CorrectedText != "请介绍一下你的项目经历？" {
		t.Fatalf("unexpected question text: %q", question.CorrectedText)
	}

	// same turn must not submit twice
	if _, _, ready = a.Ready(now.Add(500 * time.Millisecond)); ready {
		t.Fatal("same turn must not submit twice")
	}
}

func TestAggregatorPromotesStableQuestionPartialWhenFinalIsMissing(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1300 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "Spring Cloud 主要用哪些组件？", Timestamp: now})

	if _, _, ready := a.Ready(now.Add(1900 * time.Millisecond)); ready {
		t.Fatal("stable partial must retain a short finalization grace period")
	}
	question, reason, ready := a.Ready(now.Add(2100 * time.Millisecond))
	if !ready || !strings.Contains(reason, "稳定识别文本") {
		t.Fatalf("stable question partial should submit, ready=%v reason=%q", ready, reason)
	}
	if question.CorrectedText != "Spring Cloud 主要用哪些组件？" || len(question.Finals) != 1 || question.Partial != "" {
		t.Fatalf("partial was not promoted correctly: %+v", question)
	}
}

func TestAggregatorDoesNotPromoteStableNonQuestionPartial(t *testing.T) {
	now := time.Now()
	a := NewAggregator(time.Second)
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "Spring Cloud 项目的背景说明", Timestamp: now})
	if _, reason, ready := a.Ready(now.Add(3 * time.Second)); ready || reason != "等待完整提问" {
		t.Fatalf("non-question partial should remain pending, ready=%v reason=%q", ready, reason)
	}
}

func TestAggregatorIgnoresLateFinalAfterPartialFallback(t *testing.T) {
	now := time.Now()
	a := NewAggregator(time.Second)
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "Spring Cloud 主要用哪些组件？", Timestamp: now})
	if _, _, ready := a.Ready(now.Add(2100 * time.Millisecond)); !ready {
		t.Fatal("stable partial should submit")
	}
	a.ResetTurn()
	late := a.Accept(TranscriptEvent{Kind: EventFinal, Text: "Spring Cloud 主要用哪些组件？", Timestamp: now.Add(2200 * time.Millisecond)})
	if late.TurnID != "" || late.CorrectedText != "" {
		t.Fatalf("late duplicate final opened a blocking turn: %+v", late)
	}
	q := a.Accept(TranscriptEvent{Kind: EventFinal, Text: "那 Spring Cloud Gateway 有什么作用？", Timestamp: now.Add(2300 * time.Millisecond)})
	if q.CorrectedText != "那 Spring Cloud Gateway 有什么作用？" {
		t.Fatalf("next question was not kept clean: %+v", q)
	}
}

func TestQuestionIntentRecognizesFullwidthPunctuationAndWhich(t *testing.T) {
	for _, text := range []string{"Spring Cloud 组件？", "主要用哪些组件", "是否了解 Redisson"} {
		if !hasQuestionIntent(text) {
			t.Fatalf("expected question intent for %q", text)
		}
	}
}

func TestAggregatorNonQuestionUsesConfiguredWait(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1300 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "请介绍一下你做过的项目", Timestamp: now})

	// The configured 1300ms wait is preserved for text without punctuation.
	if _, _, ready := a.Ready(now.Add(1290 * time.Millisecond)); ready {
		t.Fatal("non-question must not submit before configured wait")
	}
	if _, _, ready := a.Ready(now.Add(1310 * time.Millisecond)); !ready {
		t.Fatal("non-question should submit after configured wait")
	}
}

func TestAggregatorKeepsNaturalPauseInSameTurn(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1200 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "请介绍一下你做过的项目", Timestamp: now})
	if _, _, ready := a.Ready(now.Add(700 * time.Millisecond)); ready {
		t.Fatal("short pause must not split a question")
	}
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "以及最大的技术挑战。", Timestamp: now.Add(850 * time.Millisecond)})
	q, _, ready := a.Ready(now.Add(2200 * time.Millisecond))
	if !ready || len(q.Finals) != 2 {
		t.Fatalf("expected one two-part question: %+v", q)
	}
}

func TestAggregatorSuppressesLocalAndAnswerEcho(t *testing.T) {
	now := time.Now()
	a := NewAggregator(500 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "请解释 Redis 缓存穿透", Timestamp: now})
	a.MarkLocalSpeech(now.Add(2 * time.Second))
	if _, reason, ready := a.Ready(now.Add(time.Second)); ready || reason != "本机正在讲话" {
		t.Fatalf("local speech should suppress: %v %q", ready, reason)
	}
	b := NewAggregator(500 * time.Millisecond)
	b.RememberAnswer("请解释 Redis 缓存穿透的原理")
	b.Accept(TranscriptEvent{Kind: EventFinal, Text: "请解释 Redis 缓存穿透的原理", Timestamp: now})
	if _, reason, ready := b.Ready(now.Add(time.Second)); ready || reason != "疑似本机或回答回声" {
		t.Fatalf("echo should suppress: %v %q", ready, reason)
	}
}

func TestParseASREventHandlesPartialAndFinal(t *testing.T) {
	partial, ok := parseASREvent([]byte(`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"你好","sentence_id":1,"sentence_end":false}}}}`))
	if !ok || partial.Kind != EventPartial || partial.Text != "你好" {
		t.Fatalf("unexpected partial: %+v ok=%v", partial, ok)
	}
	final, ok := parseASREvent([]byte(`{"header":{"event":"result-generated"},"payload":{"output":{"sentence":{"text":"请介绍一下 Go","sentence_id":1,"sentence_end":true}}}}`))
	if !ok || final.Kind != EventFinal {
		t.Fatalf("unexpected final: %+v ok=%v", final, ok)
	}
}

func TestNormalizeFunASREndpoint(t *testing.T) {
	endpoint, err := normalizeEndpoint("wss://dashscope.aliyuncs.com")
	if err != nil || endpoint != "wss://dashscope.aliyuncs.com/api-ws/v1/inference" {
		t.Fatalf("unexpected endpoint %q, err=%v", endpoint, err)
	}
	if _, err := normalizeEndpoint("wss://example.test/api-ws/v1/realtime"); err == nil {
		t.Fatal("realtime endpoint must be rejected for Fun-ASR task protocol")
	}
}

func TestDashScopeRunTaskEnablesHeartbeat(t *testing.T) {
	request := buildRunTaskRequest("task-test", config.TranscriptionConfig{Model: "fun-asr-realtime", SentenceWaitMS: 800})
	payload := request["payload"].(map[string]any)
	parameters := payload["parameters"].(map[string]any)
	if heartbeat, ok := parameters["heartbeat"].(bool); !ok || !heartbeat {
		t.Fatalf("heartbeat must be enabled: %+v", parameters)
	}
	if _, ok := payload["input"].(map[string]any); !ok {
		t.Fatalf("run-task input missing: %+v", payload)
	}
}

func TestPacketFormat(t *testing.T) {
	if PacketSize != 960 {
		t.Fatalf("expected 30ms of 16kHz mono S16LE to be 960 bytes, got %d", PacketSize)
	}
}

func TestManualEditAndSubmit(t *testing.T) {
	a := NewAggregator(time.Second)
	a.ReplaceCurrent("请说明 Go 的并发模型")
	q, ok := a.SubmitCurrent(time.Now())
	if !ok || q.CorrectedText != "请说明 Go 的并发模型" {
		t.Fatalf("manual submit failed: %+v ok=%v", q, ok)
	}
}

func TestAggregatorSplitsExplicitNextQuestionWithoutSplittingSubquestions(t *testing.T) {
	now := time.Now()
	a := NewAggregator(time.Second)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "请介绍一下你的项目，然后说说技术挑战。", Timestamp: now})
	if _, ok := a.SplitBeforeFinal(TranscriptEvent{Kind: EventFinal, Text: "那么下一个问题，请说说一次团队冲突。", Timestamp: now.Add(time.Second)}, now.Add(time.Second)); !ok {
		t.Fatal("explicit next-question marker should split the prior question")
	}
	if q := a.Accept(TranscriptEvent{Kind: EventFinal, Text: "那么下一个问题，请说说一次团队冲突。", Timestamp: now.Add(time.Second)}); q.CorrectedText != "那么下一个问题，请说说一次团队冲突。" {
		t.Fatalf("new question should start a fresh turn: %+v", q)
	}
}

func TestSplitFinalAtQuestionBoundaries(t *testing.T) {
	parts := splitFinalAtQuestionBoundaries("请介绍项目。那么下一个问题，请说说团队冲突。最后一个问题，三个词形容自己。")
	if len(parts) != 3 {
		t.Fatalf("expected three final segments, got %+v", parts)
	}
	if parts[0] != "请介绍项目。" || parts[1] != "那么下一个问题，请说说团队冲突。" || parts[2] != "最后一个问题，三个词形容自己。" {
		t.Fatalf("unexpected split segments: %+v", parts)
	}
}
