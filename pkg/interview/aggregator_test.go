package interview

import (
	"fmt"
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

func TestShortEllipticalFollowUpsSubmitMidInterview(t *testing.T) {
	cases := []string{
		"为什么？", "Redis呢？", "Redis呢", "线程池呢？", "线程池呢",
		"GC了解吧？", "GC了解吧", "项目里呢？", "项目里呢", "那具体怎么做？",
	}
	for _, text := range cases {
		a := NewAggregator(1300 * time.Millisecond)
		a.SetPreviousHint("请介绍一下你项目里的缓存设计")
		now := time.Now()
		a.Accept(TranscriptEvent{Kind: EventFinal, Text: text, Timestamp: now})
		question, reason, ready := a.Ready(now.Add(1400 * time.Millisecond))
		if !ready {
			t.Fatalf("short follow-up %q must submit, reason=%q", text, reason)
		}
		if question.CorrectedText != text {
			t.Fatalf("unexpected corrected text for %q: %q", text, question.CorrectedText)
		}
	}
}

func TestShortFollowUpWithoutQuestionMarkUsesConfiguredWait(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1300 * time.Millisecond)
	a.SetPreviousHint("介绍一下线程池的核心参数")
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "Redis呢", Timestamp: now})
	if _, reason, ready := a.Ready(now.Add(1290 * time.Millisecond)); ready {
		t.Fatalf("short follow-up must respect the stability window, reason=%q", reason)
	}
	if _, _, ready := a.Ready(now.Add(1310 * time.Millisecond)); !ready {
		t.Fatal("short follow-up must submit after the stability window")
	}
}

func TestFillerUtterancesNeverSubmit(t *testing.T) {
	for _, text := range []string{"好的", "嗯", "谢谢", "明白", "OK", "好的我们继续"} {
		a := NewAggregator(300 * time.Millisecond)
		a.SetPreviousHint("请介绍你的项目")
		now := time.Now()
		a.Accept(TranscriptEvent{Kind: EventFinal, Text: text, Timestamp: now})
		if _, reason, ready := a.Ready(now.Add(2 * time.Second)); ready {
			t.Fatalf("filler %q must not submit as a question", text)
		} else if reason == "" {
			t.Fatalf("filler %q must be rejected by the intent gate", text)
		}
	}
}

func TestFirstQuestionStillRequiresStrictIntent(t *testing.T) {
	now := time.Now()
	a := NewAggregator(300 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "Redis呢", Timestamp: now})
	if _, reason, ready := a.Ready(now.Add(3 * time.Second)); ready || reason != "等待完整提问" {
		t.Fatalf("first utterance without intent must stay pending: ready=%v reason=%q", ready, reason)
	}
}

func TestPartialOnlyShortFollowUpEventuallySubmits(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1300 * time.Millisecond)
	a.SetPreviousHint("介绍一下项目里的消息队列")
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "Kafka呢", Timestamp: now})
	// Repeated identical partials must not push the submit point further away.
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "Kafka呢", Timestamp: now.Add(1500 * time.Millisecond)})
	a.Accept(TranscriptEvent{Kind: EventPartial, Text: "Kafka呢", Timestamp: now.Add(3 * time.Second)})
	question, reason, ready := a.Ready(now.Add(4 * time.Second))
	if !ready || !strings.Contains(reason, "稳定识别文本") {
		t.Fatalf("stable partial-only follow-up must submit, ready=%v reason=%q", ready, reason)
	}
	if question.CorrectedText != "Kafka呢" {
		t.Fatalf("unexpected promoted text: %+v", question)
	}
}

func TestTwentyConsecutiveQuestionsAllRelease(t *testing.T) {
	a := NewAggregator(1300 * time.Millisecond)
	now := time.Now()
	previous := ""
	turns := map[string]bool{}
	for index := 1; index <= 20; index++ {
		text := fmt.Sprintf("第%d个问题：Redis缓存穿透怎么解决？", index)
		a.Accept(TranscriptEvent{Kind: EventFinal, Text: text, Timestamp: now})
		question, reason, ready := a.Ready(now.Add(710 * time.Millisecond))
		if !ready {
			t.Fatalf("question %d did not submit: %q", index, reason)
		}
		if question.TurnID == "" || turns[question.TurnID] {
			t.Fatalf("question %d reused or emptied a turn: %q", index, question.TurnID)
		}
		turns[question.TurnID] = true
		if strings.Contains(question.CorrectedText, "第") && index > 1 && strings.Contains(question.CorrectedText, fmt.Sprintf("第%d个", index-1)) {
			t.Fatalf("question %d was polluted by the previous question: %q", index, question.CorrectedText)
		}
		a.ResetTurn()
		previous = text
		a.SetPreviousHint(previous)
		now = now.Add(2 * time.Second)
	}
}

func TestStaleBlockedTurnDoesNotPolluteNextQuestion(t *testing.T) {
	now := time.Now()
	a := NewAggregator(500 * time.Millisecond)
	a.SetPreviousHint("介绍一下项目")
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "好的我们继续", Timestamp: now})
	// 5s later the blocked text had its chance; a new final must start clean.
	if question, ok := a.SplitBeforeFinal(TranscriptEvent{Kind: EventFinal, Text: "介绍一下项目架构", Timestamp: now.Add(5 * time.Second)}, now.Add(5*time.Second)); ok {
		t.Fatalf("blocked filler must not be submitted as a question: %+v", question)
	}
	q := a.Accept(TranscriptEvent{Kind: EventFinal, Text: "介绍一下项目架构", Timestamp: now.Add(5 * time.Second)})
	if q.CorrectedText != "介绍一下项目架构" {
		t.Fatalf("stale filler polluted the next question: %q", q.CorrectedText)
	}
}

func TestStaleEchoTurnIsDroppedBeforeNextQuestion(t *testing.T) {
	now := time.Now()
	a := NewAggregator(500 * time.Millisecond)
	a.SetPreviousHint("介绍一下项目")
	a.RememberAnswer("我在项目里负责缓存模块的设计")
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "我在项目里负责缓存模块的设计", Timestamp: now})
	if _, _, ready := a.Ready(now.Add(2 * time.Second)); ready {
		t.Fatal("echo must not submit")
	}
	if _, ok := a.SplitBeforeFinal(TranscriptEvent{Kind: EventFinal, Text: "缓存穿透怎么处理？", Timestamp: now.Add(3 * time.Second)}, now.Add(3*time.Second)); ok {
		t.Fatal("echo must not be submitted by the split path either")
	}
	q := a.Accept(TranscriptEvent{Kind: EventFinal, Text: "缓存穿透怎么处理？", Timestamp: now.Add(3 * time.Second)})
	if q.CorrectedText != "缓存穿透怎么处理？" {
		t.Fatalf("echo polluted the next question: %q", q.CorrectedText)
	}
}

func TestSplitBeforeFinalSubmitsStaleSuppressedQuestion(t *testing.T) {
	now := time.Now()
	a := NewAggregator(time.Second)
	a.SetPreviousHint("上一轮问题")
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "说说你踩过的最大的坑？", Timestamp: now})
	a.MarkLocalSpeech(now.Add(10 * time.Second))
	if _, _, ready := a.Ready(now.Add(1500 * time.Millisecond)); ready {
		t.Fatal("local speech must suppress automatic submit")
	}
	question, ok := a.SplitBeforeFinal(TranscriptEvent{Kind: EventFinal, Text: "那第二个问题，说说你的优点。", Timestamp: now.Add(2 * time.Second)}, now.Add(2*time.Second))
	if !ok || question.CorrectedText != "说说你踩过的最大的坑？" || !question.Submitted {
		t.Fatalf("stale suppressed question should submit late: %+v ok=%v", question, ok)
	}
}

func TestNaturalPauseStillAppendsWithinStabilityWindow(t *testing.T) {
	now := time.Now()
	a := NewAggregator(1300 * time.Millisecond)
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "介绍一下Redis", Timestamp: now})
	if _, ok := a.SplitBeforeFinal(TranscriptEvent{Kind: EventFinal, Text: "以及持久化。", Timestamp: now.Add(600 * time.Millisecond)}, now.Add(600*time.Millisecond)); ok {
		t.Fatal("natural pause within the window must not split the turn")
	}
	a.Accept(TranscriptEvent{Kind: EventFinal, Text: "以及持久化。", Timestamp: now.Add(600 * time.Millisecond)})
	q, _, ready := a.Ready(now.Add(2200 * time.Millisecond))
	if !ready || len(q.Finals) != 2 {
		t.Fatalf("expected one merged two-part question: %+v ready=%v", q, ready)
	}
}

func TestManualSubmitAllowsShortFollowUp(t *testing.T) {
	a := NewAggregator(time.Second)
	a.ReplaceCurrent("Redis呢")
	q, ok := a.SubmitCurrent(time.Now())
	if !ok || q.CorrectedText != "Redis呢" {
		t.Fatalf("manual submit of a short follow-up failed: %+v ok=%v", q, ok)
	}
}
