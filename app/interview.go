package app

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"Q-Solver/pkg/interview"
	"Q-Solver/pkg/interviewhistory"
)

func (a *App) StartInterviewSession() string {
	if a.interviewManager == nil {
		return "面试会话尚未初始化"
	}
	if a.configManager.Get().WorkMode != "interview" {
		return "请先切换到面试模式"
	}
	if err := a.interviewManager.Start(a.ctx); err != nil {
		return err.Error()
	}
	return ""
}
func (a *App) StopInterviewSession() {
	if a.interviewManager != nil {
		a.interviewManager.Stop()
	}
}

// ToggleInterviewListening is the only toggle path used by both the interview
// UI and its configurable global shortcut.
func (a *App) ToggleInterviewListening() string {
	if a.interviewManager == nil {
		return "面试会话尚未初始化"
	}
	if a.configManager.Get().WorkMode != "interview" {
		return "请先切换到面试模式"
	}
	if err := a.interviewManager.Toggle(a.ctx); err != nil {
		return err.Error()
	}
	return ""
}
func (a *App) CancelInterviewAnswer() {
	if a.interviewManager != nil {
		a.interviewManager.CancelAnswer()
	}
}
func (a *App) EditInterviewQuestion(text string) {
	if a.interviewManager != nil {
		a.interviewManager.EditQuestion(text)
	}
}
func (a *App) SubmitInterviewQuestion() {
	if a.interviewManager != nil {
		a.interviewManager.SubmitCurrent()
	}
}
func (a *App) GetInterviewStatus() interview.Status {
	if a.interviewManager == nil {
		return interview.Status{}
	}
	return a.interviewManager.Status()
}

func (a *App) ListInterviewSessions(limit int) ([]interviewhistory.Session, error) {
	if a.interviewHistory == nil {
		return []interviewhistory.Session{}, nil
	}
	return a.interviewHistory.ListSessions(a.ctx, limit)
}

func (a *App) GetInterviewTimeline(sessionID, before string, limit int) ([]interviewhistory.Message, error) {
	if a.interviewHistory == nil {
		return []interviewhistory.Message{}, nil
	}
	var beforeTime time.Time
	if before != "" {
		parsed, err := time.Parse(time.RFC3339Nano, before)
		if err != nil {
			return nil, fmt.Errorf("历史分页时间格式错误: %w", err)
		}
		beforeTime = parsed
	}
	return a.interviewHistory.ListMessagesPage(a.ctx, sessionID, beforeTime, limit)
}

func (a *App) OpenInterviewMarkdown(sessionID string) error {
	if a.interviewHistory == nil {
		return fmt.Errorf("面试记录服务尚未初始化")
	}
	path, err := a.interviewHistory.MarkdownPath(a.ctx, sessionID)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(a.interviewHistory.MarkdownDirectory(), path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("面试记录路径校验失败")
	}
	return exec.Command("explorer.exe", "/select,"+path).Start()
}

func (a *App) OpenInterviewRecordDirectory() error {
	if a.interviewHistory == nil {
		return fmt.Errorf("面试记录服务尚未初始化")
	}
	return exec.Command("explorer.exe", a.interviewHistory.MarkdownDirectory()).Start()
}

// TestTranscriptionConnection is intentionally connection-only: it validates
// endpoint, authorization, and session setup without uploading microphone or
// loopback audio.
func (a *App) TestTranscriptionConnection(apiKey, model, endpoint, language, engine string) string {
	cfg := a.configManager.Get().Transcription
	if apiKey != "" {
		cfg.APIKey = apiKey
	}
	cfg.Model, cfg.Endpoint, cfg.Language, cfg.Engine = model, endpoint, language, engine
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	testCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var client interview.ASRClient
	var err error
	if cfg.Engine == "windows" || (cfg.Engine != "dashscope" && cfg.APIKey == "") {
		client, err = interview.NewSystemLoopbackASR(testCtx, cfg.Language)
	} else {
		client, err = interview.ConnectDashScope(testCtx, cfg)
	}
	if err != nil {
		return err.Error()
	}
	_ = client.Close()
	return ""
}
