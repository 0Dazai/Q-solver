package interview

import (
	"testing"

	"Q-Solver/pkg/config"
)

func TestBuildAnswerPayloadDisablesThinkingByProvider(t *testing.T) {
	tests := []struct {
		name     string
		profile  config.AnswerModelConfig
		key      string
		wantType string
		wantBool bool
	}{
		{name: "deepseek chat", profile: config.AnswerModelConfig{Provider: "deepseek", Protocol: "openai_chat_completions", ThinkingMode: "disabled"}, key: "thinking", wantType: "disabled"},
		{name: "doubao responses", profile: config.AnswerModelConfig{Provider: "doubao", Protocol: "openai_responses", ThinkingMode: "disabled"}, key: "thinking", wantType: "disabled"},
		{name: "qwen chat", profile: config.AnswerModelConfig{Provider: "alibaba", Protocol: "openai_chat_completions", ThinkingMode: "disabled"}, key: "enable_thinking", wantBool: false},
		{name: "openai responses", profile: config.AnswerModelConfig{Provider: "openai", Protocol: "openai_responses", ThinkingMode: "disabled"}, key: "reasoning", wantType: "none"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, payload, err := buildAnswerPayload(test.profile, "hello")
			if err != nil {
				t.Fatal(err)
			}
			switch test.key {
			case "enable_thinking":
				if got, ok := payload[test.key].(bool); !ok || got != test.wantBool {
					t.Fatalf("%s=%#v, want %v", test.key, payload[test.key], test.wantBool)
				}
			case "reasoning":
				got := payload[test.key].(map[string]any)["effort"]
				if got != test.wantType {
					t.Fatalf("reasoning effort=%#v, want %q", got, test.wantType)
				}
			default:
				got := payload[test.key].(map[string]any)["type"]
				if got != test.wantType {
					t.Fatalf("thinking type=%#v, want %q", got, test.wantType)
				}
			}
		})
	}
}

func TestBuildAnswerPayloadAutoLeavesProviderDefaultsUntouched(t *testing.T) {
	_, payload, err := buildAnswerPayload(config.AnswerModelConfig{Provider: "deepseek", Protocol: "openai_chat_completions", ThinkingMode: "auto"}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"thinking", "reasoning", "reasoning_effort", "enable_thinking"} {
		if _, exists := payload[key]; exists {
			t.Fatalf("auto mode unexpectedly set %s", key)
		}
	}
}

func TestBuildAnswerPayloadInfersQwenForCustomProfile(t *testing.T) {
	_, payload, err := buildAnswerPayload(config.AnswerModelConfig{
		Provider: "custom", BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Model: "qwen3.5-flash", Protocol: "openai_chat_completions", ThinkingMode: "disabled",
	}, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := payload["enable_thinking"].(bool); !ok || got {
		t.Fatalf("enable_thinking=%#v, want false", payload["enable_thinking"])
	}
}
