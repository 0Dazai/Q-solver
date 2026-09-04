package interviewhistory

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"
)

var ErrQueueFull = errors.New("面试记录写入队列繁忙")

type operation struct {
	kind      string
	session   Session
	message   Message
	sessionID string
	endedAt   time.Time
}

type Service struct {
	store       *Store
	markdownDir string
	queue       chan operation
	done        chan struct{}
	closeOnce   sync.Once
}

func NewService(store *Store, markdownDir string) *Service {
	service := &Service{
		store:       store,
		markdownDir: markdownDir,
		queue:       make(chan operation, 256),
		done:        make(chan struct{}),
	}
	go service.run()
	return service
}

func (s *Service) enqueue(op operation) error {
	select {
	case s.queue <- op:
		return nil
	default:
		return ErrQueueFull
	}
}

func (s *Service) StartSession(session Session) error {
	if session.MarkdownPath == "" {
		name := session.StartedAt.Local().Format("2006-01-02-150405") + "-" + session.ID
		if len([]rune(name)) > 48 {
			name = string([]rune(name)[:48])
		}
		session.MarkdownPath = filepath.Join(s.markdownDir, name+".md")
	}
	return s.enqueue(operation{kind: "start", session: session})
}

func (s *Service) UpsertMessage(message Message) error {
	return s.enqueue(operation{kind: "message", message: message})
}

func (s *Service) FinishSession(sessionID string, endedAt time.Time) error {
	return s.enqueue(operation{kind: "finish", sessionID: sessionID, endedAt: endedAt})
}

func (s *Service) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	return s.store.ListSessions(ctx, limit)
}

func (s *Service) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	return s.store.ListMessages(ctx, sessionID)
}

func (s *Service) ListMessagesPage(ctx context.Context, sessionID string, before time.Time, limit int) ([]Message, error) {
	return s.store.ListMessagesPage(ctx, sessionID, before, limit)
}

func (s *Service) MarkdownDirectory() string { return s.markdownDir }

func (s *Service) MarkdownPath(ctx context.Context, sessionID string) (string, error) {
	sessions, err := s.store.ListSessions(ctx, 200)
	if err != nil {
		return "", err
	}
	for _, session := range sessions {
		if session.ID == sessionID {
			return session.MarkdownPath, nil
		}
	}
	return "", errors.New("未找到面试记录")
}

func (s *Service) run() {
	defer close(s.done)
	ctx := context.Background()
	for op := range s.queue {
		switch op.kind {
		case "start":
			_ = s.store.StartSession(ctx, op.session)
		case "message":
			if s.store.UpsertMessage(ctx, op.message) == nil {
				s.refreshMarkdown(ctx, op.message.SessionID)
			}
		case "finish":
			if s.store.FinishSession(ctx, op.sessionID, op.endedAt) == nil {
				s.refreshMarkdown(ctx, op.sessionID)
			}
		}
	}
}

func (s *Service) refreshMarkdown(ctx context.Context, sessionID string) {
	sessions, err := s.store.ListSessions(ctx, 200)
	if err != nil {
		return
	}
	var target Session
	for _, session := range sessions {
		if session.ID == sessionID {
			target = session
			break
		}
	}
	if target.ID == "" || target.MarkdownPath == "" {
		return
	}
	messages, err := s.store.ListMessages(ctx, sessionID)
	if err == nil {
		_ = writeMarkdown(target.MarkdownPath, RenderMarkdown(target, messages))
	}
}

func (s *Service) Close() error {
	s.closeOnce.Do(func() { close(s.queue) })
	<-s.done
	return s.store.Close()
}
