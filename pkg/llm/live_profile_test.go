package llm

import (
	"context"
	"os"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

func TestAuthorizedSavedProfiles(t *testing.T) {
	if value := testingEnv("Q_SOLVER_LIVE_MODEL_TEST"); value != "1" {
		t.Skip("set Q_SOLVER_LIVE_MODEL_TEST=1 to run authorized billable model checks")
	}
	cm := config.NewConfigManager()
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	cfg := cm.Get()
	profiles := []struct {
		name  string
		model config.AnswerModelConfig
	}{
		{name: "written", model: cfg.WrittenModel},
	}
	for _, profile := range profiles {
		if profile.model.APIKey == "" {
			t.Fatalf("%s profile key was not available from protected storage", profile.name)
		}
		t.Run(profile.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			adapter := NewOpenAIAdapter(&config.Config{
				Provider: profile.model.Provider, Model: profile.model.Model,
				APIKey: profile.model.APIKey, BaseURL: profile.model.BaseURL,
			})
			if err := adapter.TestChat(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}

}

func testingEnv(key string) string { return os.Getenv(key) }
