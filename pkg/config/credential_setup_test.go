package config

import (
	"os"
	"testing"
)

// This is an explicit, opt-in local setup test. No key is embedded in source;
// supplied values are persisted only through the Windows DPAPI secret store.
func TestAuthorizedCredentialSetup(t *testing.T) {
	qwen, deepseek, proxy := os.Getenv("Q_SOLVER_QWEN_API_KEY"), os.Getenv("Q_SOLVER_DEEPSEEK_API_KEY"), os.Getenv("Q_SOLVER_PROXY_API_KEY")
	if qwen == "" || deepseek == "" || proxy == "" {
		t.Skip("set all Q_SOLVER_*_API_KEY variables to perform authorized credential setup")
	}
	cm := NewConfigManager()
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	if err := cm.Patch(func(cfg *Config) {
		cfg.WrittenModel = AnswerModelConfig{
			Provider: "deepseek", Model: "deepseek-chat", APIKey: deepseek,
			BaseURL: "https://api.deepseek.com/v1", Protocol: "openai_chat_completions",
		}
		cfg.InterviewModel = AnswerModelConfig{
			Provider: "custom", Model: "gpt-5.5", APIKey: proxy,
			BaseURL: "https://proxy.ai-member.icu", Protocol: "openai_responses", ReasoningLevel: "xhigh", DisableResponseStorage: true,
		}
		cfg.Transcription.APIKey = qwen
		cfg.Transcription.Model = "fun-asr-realtime-2025-09-15"
		cfg.Transcription.Endpoint = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"
		cfg.Transcription.Language = "zh"
		cfg.Transcription.SentenceWaitMS = 1300
		cfg.Normalize()
	}); err != nil {
		t.Fatal(err)
	}
	public := cm.Get().Public()
	if !public.WrittenModel.APIKeySet || !public.InterviewModel.APIKeySet || !public.Transcription.APIKeySet {
		t.Fatal("credential setup did not persist all configured-state flags")
	}
}
