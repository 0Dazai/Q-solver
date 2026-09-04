package app

import (
	"context"
	"fmt"
	"time"

	"Q-Solver/pkg/interview"
)

func (a *App) TestConnection(apiKey, baseURL, provider, model string) string {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.llmService.TestConnection(ctx, apiKey, baseURL, provider, model)
}

func (a *App) GetModels(apiKey, baseURL, provider string) ([]string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return a.llmService.GetModels(ctx, apiKey, baseURL, provider)
}

// TestModelProfile and GetProfileModels allow the UI to use a DPAPI-protected
// per-mode key without returning that key to JavaScript.
func (a *App) TestModelProfile(mode, apiKey, baseURL, provider, model, protocol, thinkingMode, reasoningLevel string) string {
	profile := a.configManager.Get().WrittenModel
	if mode == "interview" {
		profile = a.configManager.Get().InterviewModel
	}
	if apiKey != "" {
		profile.APIKey = apiKey
	}
	if baseURL != "" {
		profile.BaseURL = baseURL
	}
	if provider != "" {
		profile.Provider = provider
	}
	if model != "" {
		profile.Model = model
	}
	if protocol != "" {
		profile.Protocol = protocol
	}
	if thinkingMode != "" {
		profile.ThinkingMode = thinkingMode
	}
	profile.ReasoningLevel = reasoningLevel
	if profile.APIKey == "" {
		return "请先填写该模型的 API Key"
	}
	if profile.MaxTokens <= 0 {
		profile.MaxTokens = 16
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	testCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	_, err := (&interview.HTTPAnswerExecutor{}).Stream(testCtx, profile, "请只回复 OK。", nil)
	if err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) GetProfileModels(mode, apiKey, baseURL, provider string) ([]string, error) {
	if apiKey == "" {
		var ok bool
		apiKey, ok = a.profileKey(mode)
		if !ok {
			return nil, fmt.Errorf("请先填写该模型的 API Key")
		}
	}
	return a.GetModels(apiKey, baseURL, provider)
}

func (a *App) profileKey(mode string) (string, bool) {
	cfg := a.configManager.Get()
	if mode == "interview" {
		return cfg.InterviewModel.APIKey, cfg.InterviewModel.APIKey != ""
	}
	return cfg.WrittenModel.APIKey, cfg.WrittenModel.APIKey != ""
}
