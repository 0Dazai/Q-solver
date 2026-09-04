package interview

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type senderTestASR struct {
	mu       sync.Mutex
	fail     bool
	received int
}

func (s *senderTestASR) SendAudio([]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("connection dropped")
	}
	s.received++
	return nil
}
func (*senderTestASR) Events() <-chan TranscriptEvent { return make(chan TranscriptEvent) }
func (*senderTestASR) Errors() <-chan error           { return make(chan error) }
func (*senderTestASR) Close() error                   { return nil }
func (s *senderTestASR) Received() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.received
}

func TestRunAudioSenderContinuesAfterClientReplacement(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	packets := make(chan []byte, 2)
	first := &senderTestASR{fail: true}
	second := &senderTestASR{}
	var mu sync.Mutex
	current := ASRClient(first)
	errorsSeen := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		runAudioSender(ctx, packets, func() ASRClient {
			mu.Lock()
			defer mu.Unlock()
			return current
		}, func(err error) { errorsSeen <- err })
		close(done)
	}()

	packets <- []byte{1}
	select {
	case <-errorsSeen:
	case <-time.After(time.Second):
		t.Fatal("expected first send error")
	}
	mu.Lock()
	current = second
	mu.Unlock()
	packets <- []byte{2}

	deadline := time.Now().Add(time.Second)
	for second.Received() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if second.Received() != 1 {
		t.Fatal("sender stopped instead of using replacement client")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("sender did not stop on context cancellation")
	}
}
