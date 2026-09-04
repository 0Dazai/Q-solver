//go:build windows

package interview

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestSystemSpeechCulture(t *testing.T) {
	for _, language := range []string{"", "zh", "zh-CN"} {
		if got, err := systemSpeechCulture(language); err != nil || got != "zh-CN" {
			t.Fatalf("unexpected culture for %q: %q, %v", language, got, err)
		}
	}
	if _, err := systemSpeechCulture("en-US"); err == nil {
		t.Fatal("unsupported system speech language should be explicit")
	}
}

func TestSystemSpeechCommandDoesNotCreateConsoleWindow(t *testing.T) {
	cmd := newSystemSpeechCommand()
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&0x08000000 == 0 {
		t.Fatal("Windows Speech helper must start without a visible console window")
	}
}

func TestSystemSpeechLoopbackStarts(t *testing.T) {
	if os.Getenv("Q_SOLVER_SYSTEM_ASR_TEST") != "1" {
		t.Skip("set Q_SOLVER_SYSTEM_ASR_TEST=1 to run the local Windows Speech startup test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := NewSystemLoopbackASR(ctx, "zh")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-client.Errors():
		if err != nil {
			_ = client.Close()
			t.Fatal(err)
		}
	case <-time.After(600 * time.Millisecond):
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSystemSpeechLoopbackStaysAliveWhileReceivingPCM(t *testing.T) {
	if os.Getenv("Q_SOLVER_SYSTEM_ASR_TEST") != "1" {
		t.Skip("set Q_SOLVER_SYSTEM_ASR_TEST=1 to run the local Windows Speech stability test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	client, err := NewSystemLoopbackASR(ctx, "zh")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	}()

	packet := make([]byte, 16_000*2*30/1000) // 30 ms of 16 kHz mono PCM S16LE silence.
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(6 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case err, ok := <-client.Errors():
			if !ok {
				t.Fatal("Windows 系统语音识别意外关闭")
			}
			if err != nil {
				t.Fatal(err)
			}
		case <-ticker.C:
			if err := client.SendAudio(packet); err != nil {
				select {
				case processErr, ok := <-client.Errors():
					if ok && processErr != nil {
						t.Fatalf("写入 PCM 失败: %v；识别器错误: %v", err, processErr)
					}
					t.Fatalf("写入 PCM 失败: %v；识别器错误通道已关闭", err)
				case <-time.After(time.Second):
					t.Fatalf("写入 PCM 失败: %v；未收到识别器退出原因", err)
				}
			}
		case <-deadline.C:
			return
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}

func TestSystemSpeechMicrophoneStarts(t *testing.T) {
	if os.Getenv("Q_SOLVER_SYSTEM_ASR_TEST") != "1" {
		t.Skip("set Q_SOLVER_SYSTEM_ASR_TEST=1 to run the local Windows Speech microphone test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := NewSystemMicrophoneASR(ctx, "zh")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-client.Errors():
		if err != nil {
			_ = client.Close()
			t.Fatal(err)
		}
	case <-time.After(600 * time.Millisecond):
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}
