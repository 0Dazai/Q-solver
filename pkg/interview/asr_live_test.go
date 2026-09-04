package interview

import (
	"context"
	"os"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

func TestDashScopeHandshakeFromEnvironment(t *testing.T) {
	key := os.Getenv("Q_SOLVER_QWEN_API_KEY")
	if key == "" {
		t.Skip("set Q_SOLVER_QWEN_API_KEY to run the authorized live handshake test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := ConnectDashScope(ctx, config.TranscriptionConfig{
		APIKey: key, Model: "fun-asr-realtime-2025-09-15",
		Endpoint: "wss://dashscope.aliyuncs.com/api-ws/v1/inference", Language: "zh", SentenceWaitMS: 1300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDashScopeTaskFinishesWithStoredConfig(t *testing.T) {
	if os.Getenv("Q_SOLVER_STORED_ASR_TEST") != "1" {
		t.Skip("set Q_SOLVER_STORED_ASR_TEST=1 to run the authorized stored-credential ASR task test")
	}
	manager := config.NewConfigManager()
	if err := manager.Load(); err != nil {
		t.Fatal(err)
	}
	cfg := manager.Get().Transcription
	if cfg.APIKey == "" {
		t.Skip("no stored transcription credential")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := ConnectDashScope(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendAudio(make([]byte, PacketSize)); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}
