package app

import (
	"fmt"

	"Q-Solver/pkg/config"
)

// GetSettings 返回当前配置
func (a *App) GetSettings() config.Config {
	return a.configManager.Get().Public()
}

// UpdateSettings 更新配置（从前端 JSON）
func (a *App) UpdateSettings(configJson string) string {
	if err := a.configManager.UpdateFromJSON(configJson); err != nil {
		return err.Error()
	}
	return ""
}

// SetWorkMode is the narrow, immediate mode-switch path used by the main UI.
// It preserves all model and transcription settings while the config subscriber
// owns session start/stop and emits work-mode-changed after persistence.
func (a *App) SetWorkMode(mode string) string {
	if mode != "written" && mode != "interview" {
		return fmt.Sprintf("不支持的工作模式: %s", mode)
	}
	if err := a.configManager.Patch(func(cfg *config.Config) {
		cfg.WorkMode = mode
	}); err != nil {
		return err.Error()
	}
	return ""
}
