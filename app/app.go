package app

import (
	"Q-Solver/pkg/common"
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/interview"
	"Q-Solver/pkg/interviewhistory"
	"Q-Solver/pkg/knowledge"
	"Q-Solver/pkg/llm"
	"Q-Solver/pkg/logger"
	"Q-Solver/pkg/resume"
	"Q-Solver/pkg/screen"
	"Q-Solver/pkg/shortcut"
	"Q-Solver/pkg/solution"
	"Q-Solver/pkg/state"
	"Q-Solver/pkg/task"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx context.Context

	configManager *config.ConfigManager
	stateManager  *state.StateManager
	taskManager   *task.TaskCoordinator

	llmService       *llm.Service
	resumeService    *resume.Service
	shortcutService  *shortcut.Service
	screenService    *screen.Service
	solver           *solution.Solver
	interviewManager *interview.Manager
	knowledgeStore   *knowledge.Store
	interviewHistory *interviewhistory.Service
}

func NewApp() *App {
	configManager := config.NewConfigManager()

	return &App{
		configManager: configManager,
		stateManager:  state.NewStateManager(),
		taskManager:   task.NewTaskCoordinator(),
		screenService: screen.NewService(),
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	if err := a.configManager.Load(); err != nil {
		logger.Printf("加载配置失败: %v", err)
	}

	cfg := a.configManager.Get()
	if cfg.WindowWidth > 0 && cfg.WindowHeight > 0 {
		runtime.WindowSetSize(ctx, cfg.WindowWidth, cfg.WindowHeight)
		logger.Printf("应用保存的窗口尺寸: %dx%d", cfg.WindowWidth, cfg.WindowHeight)
	}

	a.stateManager.Startup(ctx, a.EmitEvent)
	a.screenService.Startup(ctx)

	a.llmService = llm.NewService(a.configManager.Get(), a.configManager)
	a.solver = solution.NewSolver(a.llmService.GetProvider())
	a.resumeService = resume.NewService(a.configManager.Get(), a.configManager)
	sharedHTTPClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		Timeout: 0,
	}
	a.interviewManager = interview.NewManager(a.configManager.Get, a.EmitEvent, &interview.HTTPAnswerExecutor{Client: sharedHTTPClient})
	if configRoot, err := os.UserConfigDir(); err != nil {
		logger.Printf("确定面试记录数据库目录失败: %v", err)
	} else if historyStore, err := interviewhistory.OpenStore(filepath.Join(configRoot, common.AppName, "interviews", "interviews.db")); err != nil {
		logger.Printf("打开面试记录数据库失败: %v", err)
	} else {
		home, _ := os.UserHomeDir()
		markdownDir := filepath.Join(home, "Documents", common.AppName, "面试记录")
		a.interviewHistory = interviewhistory.NewService(historyStore, markdownDir)
		a.interviewManager.SetHistoryRecorder(a.interviewHistory)
	}
	if databasePath, err := knowledge.DefaultDatabasePath(common.AppName); err != nil {
		logger.Printf("确定本地资料库路径失败: %v", err)
	} else if store, err := knowledge.OpenStore(databasePath); err != nil {
		logger.Printf("打开本地资料库失败: %v", err)
	} else {
		a.knowledgeStore = store
		a.interviewManager.SetKnowledgeRetriever(knowledge.HybridRetriever{Local: store})
		a.interviewManager.SetKnowledgeContextProvider(store)
	}

	a.shortcutService = shortcut.NewService(a, a.configManager.Get().Shortcuts, func(callback func(map[string]shortcut.KeyBinding)) {
		a.configManager.Subscribe(func(newConfig config.Config, oldConfig config.Config) {
			callback(newConfig.Shortcuts)
		})
	})
	a.shortcutService.Start()

	a.configManager.Subscribe(a.onConfigChanged)
	a.stateManager.UpdateInitStatus(state.StatusReady)
	if a.configManager.Get().WorkMode == "interview" {
		go a.startInterviewIfCurrent()
	}
}

func (a *App) onConfigChanged(newConfig config.Config, oldConfig config.Config) {
	if a.solver != nil {
		a.solver.SetProvider(a.llmService.GetProvider())
	}

	if !newConfig.KeepContext && a.solver != nil {
		a.solver.ClearHistory()
	}
	if a.interviewManager != nil {
		if newConfig.WorkMode != oldConfig.WorkMode || !reflect.DeepEqual(newConfig.Transcription, oldConfig.Transcription) {
			go a.applyInterviewConfig()
		}
	}
	if newConfig.WorkMode != oldConfig.WorkMode {
		a.EmitEvent("work-mode-changed", newConfig.WorkMode)
	}

	logger.Println("配置已更新并应用")
}

func (a *App) OnShutdown(ctx context.Context) {
	if a.shortcutService != nil {
		a.shortcutService.Stop()
	}
	if a.interviewManager != nil {
		a.interviewManager.Stop()
	}
	if a.knowledgeStore != nil {
		if err := a.knowledgeStore.Close(); err != nil {
			logger.Printf("关闭本地资料库失败: %v", err)
		}
	}
	if a.interviewHistory != nil {
		if err := a.interviewHistory.Close(); err != nil {
			logger.Printf("关闭面试记录失败: %v", err)
		}
	}
	if err := a.configManager.Save(); err != nil {
		logger.Printf("保存配置失败: %v", err)
	}
}

func (a *App) applyInterviewConfig() {
	if a.configManager.Get().WorkMode != "interview" {
		a.interviewManager.Stop()
		return
	}
	if a.interviewManager.IsRunning() {
		_ = a.interviewManager.Restart(a.ctx)
		return
	}
	a.startInterviewIfCurrent()
}

func (a *App) startInterviewIfCurrent() {
	if a.configManager.Get().WorkMode != "interview" || a.interviewManager == nil {
		return
	}
	if err := a.interviewManager.Start(a.ctx); err != nil {
		a.EmitEvent("interview:error", err.Error())
	}
}

func (a *App) EmitEvent(eventName string, data ...interface{}) {
	runtime.EventsEmit(a.ctx, eventName, data...)
}

func (a *App) Show() {
	a.stateManager.ShowWindow()
}
