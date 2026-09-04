package app

import (
	"Q-Solver/pkg/logger"
	"Q-Solver/pkg/solution"
	"context"
)

const MaxScreenshots = 3

var screenshotBuffer []string

func (a *App) TriggerScreenshot() {
	cfg := a.configManager.Get()
	if cfg.WorkMode == "interview" {
		a.EmitEvent("toast", "面试模式下不使用截图答题")
		return
	}

	if cfg.APIKey == "" {
		a.EmitEvent("require-api-key")
		return
	}

	if cfg.Model == "" {
		a.EmitEvent("toast", "请先选择模型")
		a.EmitEvent("open-settings", "model")
		return
	}

	if a.taskManager.HasRunningTask() {
		logger.Println("忽略截图：当前有任务正在运行")
		a.EmitEvent("toast", "正在处理中，请稍候...")
		return
	}

	if len(screenshotBuffer) >= MaxScreenshots {
		a.EmitEvent("toast", "最多截图 3 张图片，请先发送或删除")
		return
	}

	previewResult, err := a.GetScreenshotPreview(
		cfg.CompressionQuality,
		cfg.Sharpening,
		cfg.Grayscale,
		cfg.NoCompression,
		cfg.ScreenshotMode,
	)
	if err != nil {
		logger.Printf("截图失败: %v\n", err)
		a.EmitEvent("toast", "截图失败: "+err.Error())
		return
	}

	screenshotBuffer = append(screenshotBuffer, previewResult.Base64)
	a.EmitEvent("screenshot-taken", previewResult.Base64, len(screenshotBuffer))
}

func (a *App) RemoveScreenshot(index int) {
	if index < 0 || index >= len(screenshotBuffer) {
		return
	}
	screenshotBuffer = append(screenshotBuffer[:index], screenshotBuffer[index+1:]...)
	a.EmitEvent("screenshot-removed", index, len(screenshotBuffer))
}

func (a *App) RemoveLastScreenshot() {
	if len(screenshotBuffer) == 0 {
		return
	}
	index := len(screenshotBuffer) - 1
	screenshotBuffer = screenshotBuffer[:index]
	a.EmitEvent("screenshot-removed", index, len(screenshotBuffer))
}

func (a *App) ClearScreenshots() {
	screenshotBuffer = nil
	a.EmitEvent("screenshots-cleared")
}

func (a *App) TriggerSend() {
	cfg := a.configManager.Get()
	if cfg.WorkMode == "interview" {
		a.EmitEvent("toast", "面试模式下不使用截图答题")
		return
	}

	if cfg.APIKey == "" {
		a.EmitEvent("require-api-key")
		return
	}

	if cfg.Model == "" {
		a.EmitEvent("toast", "请先选择模型")
		a.EmitEvent("open-settings", "model")
		return
	}

	if len(screenshotBuffer) == 0 {
		previewResult, err := a.GetScreenshotPreview(
			cfg.CompressionQuality,
			cfg.Sharpening,
			cfg.Grayscale,
			cfg.NoCompression,
			cfg.ScreenshotMode,
		)
		if err != nil {
			logger.Printf("截图失败: %v\n", err)
			a.EmitEvent("toast", "截图失败: "+err.Error())
			return
		}
		screenshotBuffer = append(screenshotBuffer, previewResult.Base64)
	}

	if a.taskManager.HasRunningTask() {
		// 用户主动中断当前任务，StartTask 会自动取消旧上下文
		logger.Println("当前任务将被中断，准备开始新任务")
	}

	screenshots := make([]string, len(screenshotBuffer))
	copy(screenshots, screenshotBuffer)
	screenshotBuffer = nil

	a.EmitEvent("start-solving")
	a.EmitEvent("user-message", screenshots[0])

	ctx, taskID := a.taskManager.StartTask("solve")
	go func() {
		defer a.taskManager.CompleteTask(taskID)
		a.solveInternal(ctx, screenshots)
	}()
}

func (a *App) TriggerSolve() {
	a.TriggerSend()
}

func (a *App) TriggerDeleteScreenshot() {
	a.RemoveLastScreenshot()
}

func (a *App) solveInternal(ctx context.Context, screenshots []string) bool {
	cfg := a.configManager.Get()

	if cfg.APIKey == "" {
		a.EmitEvent("require-api-key")
		return false
	}

	req := solution.Request{
		Config:      cfg,
		Screenshots: screenshots,
	}

	cb := solution.Callbacks{
		EmitEvent: a.EmitEvent,
	}

	return a.solver.Solve(ctx, req, cb)
}

func (a *App) CancelRunningTask() bool {
	return a.taskManager.CancelCurrentTask()
}

// TriggerCancel interrupts the current answer in either work mode. Listening
// remains active in interview mode and has its own configurable shortcut.
func (a *App) TriggerCancel() {
	if a.configManager.Get().WorkMode == "interview" && a.interviewManager != nil {
		a.interviewManager.CancelAnswer()
		return
	}
	if a.taskManager.HasRunningTask() {
		a.CancelRunningTask()
		a.EmitEvent("cancel-generation", nil)
	}
}

func (a *App) IsInterruptThinkingEnabled() bool {
	return a.configManager.Get().InterruptThinking
}
