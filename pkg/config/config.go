package config

import (
	"Q-Solver/pkg/shortcut"
	"encoding/json"
	"runtime"
)

type Config struct {
	APIKey             string                         `json:"apiKey,omitempty"`
	Provider           string                         `json:"provider,omitempty"`
	BaseURL            string                         `json:"baseURL,omitempty"`
	Model              string                         `json:"model,omitempty"`
	Prompt             string                         `json:"prompt,omitempty"`
	DomainId           string                         `json:"domainId,omitempty"` // 行业/岗位选择ID
	Opacity            float64                        `json:"opacity,omitempty"`
	NoCompression      bool                           `json:"noCompression,omitempty"`
	CompressionQuality int                            `json:"compressionQuality,omitempty"`
	Sharpening         float64                        `json:"sharpening,omitempty"`
	Grayscale          bool                           `json:"grayscale,omitempty"`
	KeepContext        bool                           `json:"keepContext,omitempty"`
	InterruptThinking  bool                           `json:"interruptThinking,omitempty"`
	ScreenshotMode     string                         `json:"screenshotMode,omitempty"`
	ResumePath         string                         `json:"resumePath,omitempty"`
	ResumeContent      string                         `json:"resumeContent,omitempty"`
	Shortcuts          map[string]shortcut.KeyBinding `json:"shortcuts,omitempty"`

	// 辅助模型（用于总结对话生成问题导图）
	AssistantModel string `json:"assistantModel,omitempty"`

	// 窗口尺寸
	WindowWidth  int `json:"windowWidth,omitempty"`
	WindowHeight int `json:"windowHeight,omitempty"`

	// 主题（light / dark）
	Theme string `json:"theme,omitempty"`

	// WorkMode keeps screenshot solving and realtime interview assistance mutually exclusive.
	WorkMode       string              `json:"workMode,omitempty"`
	WrittenModel   AnswerModelConfig   `json:"writtenModel,omitempty"`
	InterviewModel AnswerModelConfig   `json:"interviewModel,omitempty"`
	Transcription  TranscriptionConfig `json:"transcription,omitempty"`
	Knowledge      KnowledgeConfig     `json:"knowledge,omitempty"`
}

type KnowledgeConfig struct {
	Enabled       bool     `json:"enabled"`
	AnswerMode    string   `json:"answerMode,omitempty"`
	Paths         []string `json:"paths,omitempty"`
	CloudEnabled  bool     `json:"cloudEnabled,omitempty"`
	CloudProvider string   `json:"cloudProvider,omitempty"`
	CloudEndpoint string   `json:"cloudEndpoint,omitempty"`
	Collection    string   `json:"collection,omitempty"`
}

// AnswerModelConfig is self-contained so selecting a model never borrows a
// credential from a different provider or protocol.
type AnswerModelConfig struct {
	Provider               string  `json:"provider,omitempty"`
	Model                  string  `json:"model,omitempty"`
	APIKey                 string  `json:"apiKey,omitempty"`
	APIKeySet              bool    `json:"apiKeySet,omitempty"`
	BaseURL                string  `json:"baseURL,omitempty"`
	Protocol               string  `json:"protocol,omitempty"`     // openai_chat_completions or openai_responses
	ThinkingMode           string  `json:"thinkingMode,omitempty"` // auto, disabled or enabled
	ReasoningLevel         string  `json:"reasoningLevel,omitempty"`
	DisableResponseStorage bool    `json:"disableResponseStorage,omitempty"`
	MaxTokens              int     `json:"maxTokens,omitempty"`
	Temperature            float64 `json:"temperature,omitempty"`
	SystemPrompt           string  `json:"systemPrompt,omitempty"`
}

type TranscriptionConfig struct {
	Engine         string   `json:"engine,omitempty"`
	APIKey         string   `json:"apiKey,omitempty"`
	APIKeySet      bool     `json:"apiKeySet,omitempty"`
	Model          string   `json:"model,omitempty"`
	Endpoint       string   `json:"endpoint,omitempty"`
	Region         string   `json:"region,omitempty"`
	Language       string   `json:"language,omitempty"`
	Hotwords       []string `json:"hotwords,omitempty"`
	ContextPhrases []string `json:"contextPhrases,omitempty"`
	VocabularyID   string   `json:"vocabularyId,omitempty"`
	SentenceWaitMS int      `json:"sentenceWaitMs,omitempty"`
	AutoSubmit     bool     `json:"autoSubmit,omitempty"`
}

const DefaultModel = ""

func NewDefaultConfig() Config {
	return Config{
		APIKey:             "",
		Provider:           "openai",
		BaseURL:            "https://api.openai.com/v1",
		Model:              DefaultModel,
		ResumePath:         "",
		Prompt:             "",
		DomainId:           "general-assistant", // 默认选择通用的
		Opacity:            1.0,
		KeepContext:        false,
		InterruptThinking:  false,
		ScreenshotMode:     "fullscreen", // 默认全屏截图，确保捕获完整内容
		NoCompression:      false,        // 保持压缩以减小文件大小
		CompressionQuality: 92,           // 高质量压缩，确保 AI 清晰识别文字
		Sharpening:         0.3,          // 适度锐化，增强文字边缘清晰度
		Grayscale:          false,        // 保持彩色，某些场景颜色有意义
		ResumeContent:      "",

		Shortcuts: getDefaultShortcuts(),

		// 辅助模型
		AssistantModel: "",

		// 窗口尺寸默认值
		WindowWidth:  0,
		WindowHeight: 0,

		// 主题默认值
		Theme: "light",

		WorkMode: "written",
		WrittenModel: AnswerModelConfig{
			Provider: "openai", BaseURL: "https://api.openai.com/v1", Protocol: "openai_chat_completions", ThinkingMode: "auto",
		},
		InterviewModel: AnswerModelConfig{
			Provider: "openai", BaseURL: "https://api.openai.com/v1", Protocol: "openai_chat_completions", ThinkingMode: "disabled",
		},
		Transcription: TranscriptionConfig{
			Engine: "auto", Model: "fun-asr-realtime-2025-09-15", Endpoint: "wss://dashscope.aliyuncs.com/api-ws/v1/inference",
			Language: "zh", SentenceWaitMS: 1300, AutoSubmit: true,
		},
		Knowledge: KnowledgeConfig{Enabled: true, AnswerMode: "knowledge_first"},
	}
}

// getDefaultShortcuts 根据平台返回默认快捷键配置
func getDefaultShortcuts() map[string]shortcut.KeyBinding {
	if runtime.GOOS == "darwin" {
		// macOS 使用简化的快捷键（不依赖 Windows VK 码）
		return map[string]shortcut.KeyBinding{
			"solve":               {ComboID: "Cmd+1", KeyName: "⌘1"},
			"send":                {ComboID: "Cmd+J", KeyName: "⌘J"},
			"cancel":              {ComboID: "Escape", KeyName: "Esc"},
			"delete":              {ComboID: "Cmd+D", KeyName: "⌘D"},
			"toggle":              {ComboID: "Cmd+2", KeyName: "⌘2"},
			"clickthrough":        {ComboID: "Cmd+3", KeyName: "⌘3"},
			"move_up":             {ComboID: "Cmd+Option+Up", KeyName: "⌘⌥↑"},
			"move_down":           {ComboID: "Cmd+Option+Down", KeyName: "⌘⌥↓"},
			"move_left":           {ComboID: "Cmd+Option+Left", KeyName: "⌘⌥←"},
			"move_right":          {ComboID: "Cmd+Option+Right", KeyName: "⌘⌥→"},
			"scroll_up":           {ComboID: "Cmd+Option+Shift+Up", KeyName: "⌘⌥⇧↑"},
			"scroll_down":         {ComboID: "Cmd+Option+Shift+Down", KeyName: "⌘⌥⇧↓"},
			"mode_toggle":         {ComboID: "Cmd+4", KeyName: "⌘4"},
			"interview_listening": {ComboID: "Cmd+5", KeyName: "⌘5"},
		}
	}
	// Windows 默认快捷键
	return map[string]shortcut.KeyBinding{
		"screenshot":          {ComboID: "119", KeyName: "F8"},
		"send":                {ComboID: "74+162", KeyName: "Ctrl+J"},
		"cancel":              {ComboID: "27", KeyName: "Esc"},
		"delete":              {ComboID: "68+162", KeyName: "Ctrl+D"},
		"toggle":              {ComboID: "120", KeyName: "F9"},
		"minimize":            {ComboID: "118", KeyName: "F7"},
		"clickthrough":        {ComboID: "121", KeyName: "F10"},
		"move_up":             {ComboID: "38+164", KeyName: "Alt+↑"},
		"move_down":           {ComboID: "40+164", KeyName: "Alt+↓"},
		"move_left":           {ComboID: "37+164", KeyName: "Alt+←"},
		"move_right":          {ComboID: "39+164", KeyName: "Alt+→"},
		"scroll_up":           {ComboID: "33+164", KeyName: "Alt+PgUp"},
		"scroll_down":         {ComboID: "34+164", KeyName: "Alt+PgDn"},
		"mode_toggle":         {ComboID: "117", KeyName: "F6"},
		"interview_listening": {ComboID: "122", KeyName: "F11"},
	}
}

// Normalize preserves existing installations while moving the legacy model
// fields into the independent written-model profile on first load.
func (c *Config) Normalize() {
	defaults := NewDefaultConfig()
	if c.WorkMode == "" {
		c.WorkMode = defaults.WorkMode
	}
	if c.WrittenModel.Model == "" && c.Model != "" {
		c.WrittenModel = AnswerModelConfig{
			Provider: c.Provider, Model: c.Model, APIKey: c.APIKey, BaseURL: c.BaseURL,
			Protocol: "openai_chat_completions", SystemPrompt: c.Prompt,
		}
	}
	if c.WrittenModel.Protocol == "" {
		c.WrittenModel.Protocol = "openai_chat_completions"
	}
	if c.InterviewModel.Protocol == "" {
		c.InterviewModel.Protocol = "openai_chat_completions"
	}
	if c.WrittenModel.ThinkingMode == "" {
		if c.WrittenModel.ReasoningLevel != "" {
			c.WrittenModel.ThinkingMode = "enabled"
		} else {
			c.WrittenModel.ThinkingMode = defaults.WrittenModel.ThinkingMode
		}
	}
	if c.InterviewModel.ThinkingMode == "" {
		if c.InterviewModel.ReasoningLevel != "" {
			c.InterviewModel.ThinkingMode = "enabled"
		} else {
			c.InterviewModel.ThinkingMode = defaults.InterviewModel.ThinkingMode
		}
	}
	if c.InterviewModel.BaseURL == "" {
		c.InterviewModel.BaseURL = c.WrittenModel.BaseURL
	}
	// The established screenshot solver still consumes the legacy top-level
	// fields. Keep them as a derived compatibility view of the written profile;
	// never derive the profile from them after initial migration.
	if c.WrittenModel.Provider != "" {
		c.Provider = c.WrittenModel.Provider
	}
	if c.WrittenModel.Model != "" {
		c.Model = c.WrittenModel.Model
	}
	if c.WrittenModel.BaseURL != "" {
		c.BaseURL = c.WrittenModel.BaseURL
	}
	if c.WrittenModel.SystemPrompt != "" || c.Prompt == "" {
		c.Prompt = c.WrittenModel.SystemPrompt
	}
	c.APIKey = c.WrittenModel.APIKey
	if c.Transcription.Model == "" {
		c.Transcription.Model = defaults.Transcription.Model
	}
	if c.Transcription.Engine == "" {
		c.Transcription.Engine = "auto"
	}
	if c.Transcription.Endpoint == "" {
		c.Transcription.Endpoint = defaults.Transcription.Endpoint
	}
	if c.Transcription.Language == "" {
		c.Transcription.Language = defaults.Transcription.Language
	}
	if c.Transcription.SentenceWaitMS == 0 {
		c.Transcription.SentenceWaitMS = defaults.Transcription.SentenceWaitMS
	}
	if c.Knowledge.AnswerMode == "" {
		c.Knowledge.AnswerMode = defaults.Knowledge.AnswerMode
	}
}

// Public removes every credential before configuration crosses the Wails
// boundary. APIKeySet preserves useful UI feedback without exposing secrets.
func (c Config) Public() Config {
	c.WrittenModel.APIKeySet = c.WrittenModel.APIKey != ""
	c.InterviewModel.APIKeySet = c.InterviewModel.APIKey != ""
	c.Transcription.APIKeySet = c.Transcription.APIKey != ""
	c.APIKey = ""
	c.WrittenModel.APIKey = ""
	c.InterviewModel.APIKey = ""
	c.Transcription.APIKey = ""
	return c
}

func (c *Config) ToJSON() string {
	data, _ := json.MarshalIndent(c, "", "  ")
	return string(data)
}

func (c *Config) Validate() error {
	if c.ScreenshotMode != "" && c.ScreenshotMode != "fullscreen" && c.ScreenshotMode != "window" {
		return &ValidationError{Field: "screenshotMode", Message: "截图模式必须是 'fullscreen' 或 'window'"}
	}
	if c.Opacity < 0 || c.Opacity > 1 {
		return &ValidationError{Field: "opacity", Message: "透明度必须在 0-1 之间"}
	}
	if c.CompressionQuality < 1 || c.CompressionQuality > 100 {
		return &ValidationError{Field: "compressionQuality", Message: "压缩质量必须在 1-100 之间"}
	}
	if c.WorkMode != "" && c.WorkMode != "written" && c.WorkMode != "interview" {
		return &ValidationError{Field: "workMode", Message: "模式必须是 'written' 或 'interview'"}
	}
	if c.Transcription.SentenceWaitMS < 300 || c.Transcription.SentenceWaitMS > 10000 {
		return &ValidationError{Field: "transcription.sentenceWaitMs", Message: "句末等待必须在 300-10000 毫秒之间"}
	}
	if c.Transcription.Engine != "auto" && c.Transcription.Engine != "dashscope" && c.Transcription.Engine != "windows" {
		return &ValidationError{Field: "transcription.engine", Message: "转写引擎必须是 auto、dashscope 或 windows"}
	}
	if c.Knowledge.AnswerMode != "general" && c.Knowledge.AnswerMode != "knowledge_first" && c.Knowledge.AnswerMode != "knowledge_only" {
		return &ValidationError{Field: "knowledge.answerMode", Message: "资料回答模式必须是 general、knowledge_first 或 knowledge_only"}
	}
	for field, profile := range map[string]AnswerModelConfig{"writtenModel": c.WrittenModel, "interviewModel": c.InterviewModel} {
		if profile.ThinkingMode != "auto" && profile.ThinkingMode != "disabled" && profile.ThinkingMode != "enabled" {
			return &ValidationError{Field: field + ".thinkingMode", Message: "思考模式必须是 auto、disabled 或 enabled"}
		}
		switch profile.ReasoningLevel {
		case "", "minimal", "low", "medium", "high", "xhigh", "max":
		default:
			return &ValidationError{Field: field + ".reasoningLevel", Message: "思考强度必须是 minimal、low、medium、high、xhigh 或 max"}
		}
	}
	return nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
