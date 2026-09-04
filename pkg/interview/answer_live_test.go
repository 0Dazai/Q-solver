package interview

import (
	"context"
	"os"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

func TestAuthorizedInterviewAnswerProfile(t *testing.T) {
	if os.Getenv("Q_SOLVER_LIVE_MODEL_TEST") != "1" {
		t.Skip("set Q_SOLVER_LIVE_MODEL_TEST=1 to run the authorized billable model check")
	}
	cm := config.NewConfigManager()
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	profile := cm.Get().InterviewModel
	profile.MaxTokens = 16
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	answer, err := (&HTTPAnswerExecutor{}).Stream(ctx, profile, "请只回复 OK。", nil)
	if err != nil {
		t.Fatal(err)
	}
	if answer == "" {
		t.Fatal("model connection completed without a streamed answer")
	}
}
