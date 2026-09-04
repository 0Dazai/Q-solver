package config

import "testing"

func TestKnowledgeDefaultsAreLocalAndKnowledgeFirst(t *testing.T) {
	cfg := NewDefaultConfig()
	if !cfg.Knowledge.Enabled {
		t.Fatal("local knowledge should be enabled by default")
	}
	if cfg.Knowledge.AnswerMode != "knowledge_first" {
		t.Fatalf("unexpected default answer mode: %q", cfg.Knowledge.AnswerMode)
	}
	if cfg.Knowledge.CloudEnabled {
		t.Fatal("cloud vector enhancement must be disabled by default")
	}
}

func TestNormalizeMigratesLegacyModelWithoutOverwritingInterviewKey(t *testing.T) {
	cfg := Config{
		Provider: "openai", Model: "written-model", APIKey: "written-key", BaseURL: "https://written.example/v1",
		InterviewModel: AnswerModelConfig{Model: "interview-model", APIKey: "interview-key", BaseURL: "https://interview.example/v1"},
	}
	cfg.Normalize()
	if cfg.WrittenModel.APIKey != "written-key" || cfg.WrittenModel.Model != "written-model" {
		t.Fatalf("legacy profile was not migrated: %+v", cfg.WrittenModel)
	}
	if cfg.InterviewModel.APIKey != "interview-key" || cfg.InterviewModel.APIKey == cfg.WrittenModel.APIKey {
		t.Fatalf("model credentials must stay independent: %+v", cfg)
	}
}

func TestTranscriptionDefaultsAreConfigurable(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.Transcription.Model = "custom-asr"
	cfg.Transcription.Endpoint = "wss://example.test/asr"
	cfg.Normalize()
	if cfg.Transcription.Model != "custom-asr" || cfg.Transcription.Endpoint != "wss://example.test/asr" {
		t.Fatalf("ASR defaults overwrote explicit configuration: %+v", cfg.Transcription)
	}
}

func TestDefaultShortcutsIncludeInterviewListening(t *testing.T) {
	cfg := NewDefaultConfig()
	binding, ok := cfg.Shortcuts["interview_listening"]
	if !ok || binding.ComboID == "" || binding.KeyName == "" {
		t.Fatalf("missing interview listening shortcut: %+v", cfg.Shortcuts)
	}
}

func TestThinkingModeDefaultsAndLegacyMigration(t *testing.T) {
	defaults := NewDefaultConfig()
	if defaults.WrittenModel.ThinkingMode != "auto" || defaults.InterviewModel.ThinkingMode != "disabled" {
		t.Fatalf("unexpected thinking defaults: written=%q interview=%q", defaults.WrittenModel.ThinkingMode, defaults.InterviewModel.ThinkingMode)
	}
	legacy := Config{
		WrittenModel:   AnswerModelConfig{Protocol: "openai_chat_completions", ReasoningLevel: "high"},
		InterviewModel: AnswerModelConfig{Protocol: "openai_chat_completions"},
		Transcription:  TranscriptionConfig{SentenceWaitMS: 1300},
		Knowledge:      KnowledgeConfig{AnswerMode: "knowledge_first"},
	}
	legacy.Normalize()
	if legacy.WrittenModel.ThinkingMode != "enabled" || legacy.InterviewModel.ThinkingMode != "disabled" {
		t.Fatalf("legacy thinking settings were not migrated: %+v", legacy)
	}
}

func TestPublicSettingsNeverExposeProfileSecrets(t *testing.T) {
	cfg := Config{
		APIKey:         "legacy-secret",
		WrittenModel:   AnswerModelConfig{APIKey: "written-secret"},
		InterviewModel: AnswerModelConfig{APIKey: "interview-secret"},
		Transcription:  TranscriptionConfig{APIKey: "transcription-secret"},
	}
	public := cfg.Public()
	if public.APIKey != "" || public.WrittenModel.APIKey != "" || public.InterviewModel.APIKey != "" || public.Transcription.APIKey != "" {
		t.Fatalf("public settings leaked a credential: %+v", public)
	}
	if !public.WrittenModel.APIKeySet || !public.InterviewModel.APIKeySet || !public.Transcription.APIKeySet {
		t.Fatalf("public settings should preserve configured-state flags: %+v", public)
	}
}
