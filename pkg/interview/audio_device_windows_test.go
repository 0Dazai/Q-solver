//go:build windows && cgo

package interview

import (
	"os"
	"testing"
	"time"
)

// This is opt-in because it touches the default Windows playback device. It
// does not persist or upload audio; it only verifies lifecycle cleanup.
func TestLoopbackStartStop(t *testing.T) {
	if os.Getenv("Q_SOLVER_DEVICE_TEST") != "1" {
		t.Skip("set Q_SOLVER_DEVICE_TEST=1 to exercise the Windows audio device")
	}
	capture, err := NewLoopbackCapture()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.Close()
	if err := capture.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	capture.Stop()
	if capture.IsRunning() {
		t.Fatal("loopback capture is still running after Stop")
	}
}

func TestMicActivityStartStop(t *testing.T) {
	if os.Getenv("Q_SOLVER_DEVICE_TEST") != "1" {
		t.Skip("set Q_SOLVER_DEVICE_TEST=1 to exercise the Windows microphone")
	}
	mic, err := NewMicActivity()
	if err != nil {
		t.Fatal(err)
	}
	defer mic.Close()
	if err := mic.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(120 * time.Millisecond)
	mic.Stop()
}
