//go:build !cgo

package interview

import (
	"errors"
	"time"
)

// This build deliberately has no audio fallback. WASAPI loopback requires the
// CGO-backed malgo implementation, and reporting that fact is safer than
// silently switching to an unrelated capture path.
type AudioCapture struct{}

func NewLoopbackCapture() (*AudioCapture, error) {
	return nil, errors.New("当前构建未启用 CGO，无法使用 WASAPI Loopback")
}
func (c *AudioCapture) Start() error {
	return errors.New("当前构建未启用 CGO，无法使用 WASAPI Loopback")
}
func (c *AudioCapture) Stop()                  {}
func (c *AudioCapture) Restart() error         { return c.Start() }
func (c *AudioCapture) Close()                 {}
func (c *AudioCapture) Packets() <-chan []byte { return nil }
func (c *AudioCapture) DroppedPackets() uint64 { return 0 }
func (c *AudioCapture) QueuedPackets() int     { return 0 }
func (c *AudioCapture) IsRunning() bool        { return false }

type MicActivity struct{}

func NewMicActivity() (*MicActivity, error) {
	return nil, errors.New("当前构建未启用 CGO，无法检测麦克风活动")
}
func (m *MicActivity) Start() error {
	return errors.New("当前构建未启用 CGO，无法检测麦克风活动")
}
func (m *MicActivity) Stop()                  {}
func (m *MicActivity) Close()                 {}
func (m *MicActivity) ActiveUntil() time.Time { return time.Time{} }
func (m *MicActivity) IsActive() bool         { return false }
func (m *MicActivity) Packets() <-chan []byte { return nil }
func (m *MicActivity) DroppedPackets() uint64 { return 0 }
func (m *MicActivity) QueuedPackets() int     { return 0 }
