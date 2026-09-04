package interview

import (
	"strings"
	"testing"
	"time"
)

func TestAnswerInputIncludesLocalResponseOnlyAsReference(t *testing.T) {
	input := answerInputWithLocalResponse("请介绍一下你的项目。", []string{"我刚才负责了订单服务的改造。"})
	if !strings.Contains(input, "请介绍一下你的项目。") || !strings.Contains(input, "订单服务") {
		t.Fatalf("missing question or local response: %q", input)
	}
	if !strings.Contains(input, "仅作下一轮回答的上下文参考") || !strings.Contains(input, "不要复述") {
		t.Fatalf("local response instructions are missing: %q", input)
	}
}

func TestLocalTranscriptIsDisplayedButOnlyFinalsBecomeContext(t *testing.T) {
	var events []TranscriptEvent
	m := NewManager(nil, func(name string, data ...any) {
		if name == "interview:local-transcript" {
			events = append(events, data[0].(TranscriptEvent))
		}
	}, nil)
	m.acceptLocal(TranscriptEvent{Kind: EventPartial, Text: "我负责", Timestamp: time.Now()})
	m.acceptLocal(TranscriptEvent{Kind: EventFinal, Text: "我负责订单服务。", Timestamp: time.Now()})
	if len(events) != 2 || events[0].Kind != EventPartial || events[1].Kind != EventFinal {
		t.Fatalf("local transcript event sequence is wrong: %+v", events)
	}
	finals := m.consumeLocalFinals()
	if len(finals) != 1 || finals[0] != "我负责订单服务。" {
		t.Fatalf("only final local transcript should become context: %+v", finals)
	}
}

func TestAnswerInputWithoutLocalResponseStaysQuestionOnly(t *testing.T) {
	if got := answerInputWithLocalResponse("解释 Go 的并发模型", nil); got != "解释 Go 的并发模型" {
		t.Fatalf("unexpected context-free input: %q", got)
	}
}
