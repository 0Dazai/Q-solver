package llm

import (
	"testing"

	"Q-Solver/pkg/config"
)

func TestValidateMessagesRejectsDeepSeekImages(t *testing.T) {
	adapter := NewOpenAIAdapter(&config.Config{
		Provider: "deepseek",
		BaseURL:  "https://api.deepseek.com/v1",
		APIKey:   "test-key",
		Model:    "deepseek-chat",
	})

	err := adapter.validateMessages([]Message{
		NewMultiPartMessage(RoleUser, []ContentPart{
			TextPart("read this screenshot"),
			ImagePart("data:image/png;base64,AAAA"),
		}),
	})

	if err == nil {
		t.Fatal("expected DeepSeek image input to be rejected")
	}
	if err.Error() != unsupportedImageInputMessage {
		t.Fatalf("unexpected error: %q", err.Error())
	}
}

func TestValidateMessagesAllowsDeepSeekText(t *testing.T) {
	adapter := NewOpenAIAdapter(&config.Config{
		Provider: "deepseek",
		BaseURL:  "https://api.deepseek.com/v1",
		APIKey:   "test-key",
		Model:    "deepseek-chat",
	})

	if err := adapter.validateMessages([]Message{NewUserMessage("hello")}); err != nil {
		t.Fatalf("expected DeepSeek text input to pass validation: %v", err)
	}
}

func TestValidateMessagesAllowsDoubaoImages(t *testing.T) {
	adapter := NewOpenAIAdapter(&config.Config{
		Provider: "doubao",
		BaseURL:  "https://ark.cn-beijing.volces.com/api/v3",
		APIKey:   "test-key",
		Model:    "doubao-vision-endpoint-id",
	})

	if err := adapter.validateMessages([]Message{
		NewMultiPartMessage(RoleUser, []ContentPart{
			TextPart("read this screenshot"),
			ImagePart("data:image/png;base64,AAAA"),
		}),
	}); err != nil {
		t.Fatalf("expected Doubao image input to pass validation: %v", err)
	}
}

func TestProviderCodeInfersArkAsDoubao(t *testing.T) {
	got := providerCode(&config.Config{BaseURL: "https://ark.cn-beijing.volces.com/api/v3"})
	if got != "doubao" {
		t.Fatalf("providerCode() = %q, want doubao", got)
	}
}
