package interview

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Q-Solver/pkg/config"
)

func TestResponsesStreamDoesNotAppendDoneSnapshot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"第一段\"}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"第二段\"}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.output_text.done\",\"text\":\"第一段第二段\"}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{}}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	var streamed strings.Builder
	answer, err := (&HTTPAnswerExecutor{}).Stream(context.Background(), config.AnswerModelConfig{
		APIKey: "test", BaseURL: server.URL, Model: "gpt-test",
		Protocol: "openai_responses", MaxTokens: 32,
	}, "测试", func(chunk string) { streamed.WriteString(chunk) })
	if err != nil {
		t.Fatal(err)
	}
	if answer != "第一段第二段" || streamed.String() != answer {
		t.Fatalf("Responses output duplicated or diverged: answer=%q streamed=%q", answer, streamed.String())
	}
}

func TestChatCompletionsStreamStillCombinesDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"第一段\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"第二段\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	answer, err := (&HTTPAnswerExecutor{}).Stream(context.Background(), config.AnswerModelConfig{
		APIKey: "test", BaseURL: server.URL, Model: "chat-test",
		Protocol: "openai_chat_completions", MaxTokens: 32,
	}, "测试", nil)
	if err != nil {
		t.Fatal(err)
	}
	if answer != "第一段第二段" {
		t.Fatalf("chat stream changed unexpectedly: %q", answer)
	}
}

func TestResponsesChunkIgnoresTypedTerminalText(t *testing.T) {
	if got := extractAnswerChunk("openai_responses", `{"type":"response.output_text.done","text":"完整答案"}`); got != "" {
		t.Fatalf("terminal snapshot must not be appended: %q", got)
	}
	if got := extractAnswerChunk("openai_responses", `{"delta":"兼容增量"}`); got != "兼容增量" {
		t.Fatalf("type-less relay delta lost: %q", got)
	}
}
