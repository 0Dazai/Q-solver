//go:build cgo

package interview

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gen2brain/malgo"
)

const audioQueueCapacity = 128

// AudioCapture is adapted from the reviewed fork's loopback path. The device
// callback only copies to a bounded queue; packetising and network work happen
// off the realtime audio thread.
type AudioCapture struct {
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	mu      sync.Mutex
	running bool
	raw     chan []byte
	packets chan []byte
	done    chan struct{}
	wg      sync.WaitGroup
	dropped atomic.Uint64
}

func NewLoopbackCapture() (*AudioCapture, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	return &AudioCapture{ctx: ctx}, nil
}

func (c *AudioCapture) Start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.running {
		return nil
	}
	if c.ctx == nil {
		return errors.New("音频上下文已释放")
	}
	c.raw, c.packets, c.done = make(chan []byte, audioQueueCapacity), make(chan []byte, audioQueueCapacity), make(chan struct{})
	cfg := malgo.DefaultDeviceConfig(malgo.Loopback) // Windows WASAPI loopback.
	cfg.Capture.Format, cfg.Capture.Channels, cfg.SampleRate = malgo.FormatS16, 1, SampleRate
	callbacks := malgo.DeviceCallbacks{Data: func(_ []byte, input []byte, _ uint32) {
		if len(input) == 0 {
			return
		}
		copyInput := append([]byte(nil), input...)
		select {
		case c.raw <- copyInput:
		default:
			c.dropped.Add(1)
		}
	}}
	device, err := malgo.InitDevice(c.ctx.Context, cfg, callbacks)
	if err != nil {
		return err
	}
	if err = device.Start(); err != nil {
		device.Uninit()
		return err
	}
	c.device, c.running = device, true
	c.wg.Add(1)
	go c.packetize()
	return nil
}

func (c *AudioCapture) packetize() {
	defer c.wg.Done()
	buf := make([]byte, 0, PacketSize*4)
	for {
		select {
		case <-c.done:
			return
		case data := <-c.raw:
			buf = append(buf, data...)
			for len(buf) >= PacketSize {
				packet := append([]byte(nil), buf[:PacketSize]...)
				buf = buf[PacketSize:]
				select {
				case c.packets <- packet:
				default:
					c.dropped.Add(1)
				}
			}
		}
	}
}

func (c *AudioCapture) Packets() <-chan []byte { return c.packets }
func (c *AudioCapture) DroppedPackets() uint64 { return c.dropped.Load() }
func (c *AudioCapture) QueuedPackets() int {
	if c.packets == nil {
		return 0
	}
	return len(c.packets)
}
func (c *AudioCapture) IsRunning() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.running }

func (c *AudioCapture) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	device, done := c.device, c.done
	c.device, c.running = nil, false
	c.mu.Unlock()
	if device != nil {
		_ = device.Stop()
		device.Uninit()
	}
	close(done)
	c.wg.Wait()
}
func (c *AudioCapture) Restart() error { c.Stop(); return c.Start() }
func (c *AudioCapture) Close() {
	c.Stop()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx != nil {
		_ = c.ctx.Uninit()
		c.ctx.Free()
		c.ctx = nil
	}
}

// MicActivity captures microphone PCM for its separate ASR session and keeps
// a local activity window. It never persists raw microphone audio.
type MicActivity struct {
	ctx     *malgo.AllocatedContext
	device  *malgo.Device
	mu      sync.Mutex
	running bool
	until   atomic.Int64
	raw     chan []byte
	packets chan []byte
	done    chan struct{}
	wg      sync.WaitGroup
	dropped atomic.Uint64
}

func NewMicActivity() (*MicActivity, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	return &MicActivity{ctx: ctx}, nil
}
func (m *MicActivity) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	if m.ctx == nil {
		return errors.New("麦克风上下文已释放")
	}
	m.raw, m.packets, m.done = make(chan []byte, audioQueueCapacity), make(chan []byte, audioQueueCapacity), make(chan struct{})
	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format, cfg.Capture.Channels, cfg.SampleRate = malgo.FormatS16, 1, SampleRate
	callbacks := malgo.DeviceCallbacks{Data: func(_ []byte, input []byte, _ uint32) {
		if pcmActive(input) {
			m.until.Store(time.Now().Add(900 * time.Millisecond).UnixNano())
		}
		if len(input) == 0 {
			return
		}
		copyInput := append([]byte(nil), input...)
		select {
		case m.raw <- copyInput:
		default:
			m.dropped.Add(1)
		}
	}}
	d, err := malgo.InitDevice(m.ctx.Context, cfg, callbacks)
	if err != nil {
		return err
	}
	if err = d.Start(); err != nil {
		d.Uninit()
		return err
	}
	m.device, m.running = d, true
	m.wg.Add(1)
	go m.packetize()
	return nil
}
func (m *MicActivity) packetize() {
	defer m.wg.Done()
	buf := make([]byte, 0, PacketSize*4)
	for {
		select {
		case <-m.done:
			return
		case data := <-m.raw:
			buf = append(buf, data...)
			for len(buf) >= PacketSize {
				packet := append([]byte(nil), buf[:PacketSize]...)
				buf = buf[PacketSize:]
				select {
				case m.packets <- packet:
				default:
					m.dropped.Add(1)
				}
			}
		}
	}
}
func (m *MicActivity) ActiveUntil() time.Time {
	n := m.until.Load()
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}
func (m *MicActivity) IsActive() bool         { return time.Now().Before(m.ActiveUntil()) }
func (m *MicActivity) Packets() <-chan []byte { return m.packets }
func (m *MicActivity) DroppedPackets() uint64 { return m.dropped.Load() }
func (m *MicActivity) QueuedPackets() int {
	if m.packets == nil {
		return 0
	}
	return len(m.packets)
}
func (m *MicActivity) Stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	d, done := m.device, m.done
	m.device, m.running = nil, false
	m.mu.Unlock()
	if d != nil {
		_ = d.Stop()
		d.Uninit()
	}
	close(done)
	m.wg.Wait()
}
func (m *MicActivity) Close() {
	m.Stop()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ctx != nil {
		_ = m.ctx.Uninit()
		m.ctx.Free()
		m.ctx = nil
	}
}
func pcmActive(data []byte) bool {
	var peak int
	for i := 0; i+1 < len(data); i += 2 {
		v := int(int16(uint16(data[i]) | uint16(data[i+1])<<8))
		if v < 0 {
			v = -v
		}
		if v > peak {
			peak = v
		}
	}
	return peak > 900
}
