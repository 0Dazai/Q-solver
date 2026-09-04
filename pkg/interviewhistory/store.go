package interviewhistory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("面试记录数据库路径为空")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", path+"?_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) migrate(ctx context.Context) error {
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS interview_sessions (
			id TEXT PRIMARY KEY,
			started_at TEXT NOT NULL,
			ended_at TEXT NOT NULL DEFAULT '',
			resume_path TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			answer_mode TEXT NOT NULL DEFAULT '',
			markdown_path TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS interview_messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL REFERENCES interview_sessions(id) ON DELETE CASCADE,
			turn_id TEXT NOT NULL,
			question_id TEXT NOT NULL DEFAULT '',
			role TEXT NOT NULL,
			content TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS interview_messages_session_order
			ON interview_messages(session_id, created_at, id)`,
	} {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("初始化面试记录失败: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) StartSession(ctx context.Context, session Session) error {
	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO interview_sessions
		(id,started_at,ended_at,resume_path,model,answer_mode,markdown_path)
		VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			resume_path=excluded.resume_path,model=excluded.model,
			answer_mode=excluded.answer_mode,markdown_path=excluded.markdown_path`,
		session.ID, formatTime(session.StartedAt), formatTime(session.EndedAt), session.ResumePath,
		session.Model, session.AnswerMode, session.MarkdownPath)
	return err
}

func (s *Store) FinishSession(ctx context.Context, sessionID string, endedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE interview_sessions SET ended_at=? WHERE id=?`, formatTime(endedAt), sessionID)
	return err
}

func (s *Store) UpsertMessage(ctx context.Context, message Message) error {
	now := time.Now()
	if message.CreatedAt.IsZero() {
		message.CreatedAt = now
	}
	if message.UpdatedAt.IsZero() {
		message.UpdatedAt = now
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO interview_messages
		(id,session_id,turn_id,question_id,role,content,status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			turn_id=excluded.turn_id,question_id=excluded.question_id,role=excluded.role,
			content=excluded.content,status=excluded.status,updated_at=excluded.updated_at`,
		message.ID, message.SessionID, message.TurnID, message.QuestionID, message.Role,
		message.Content, message.Status, formatTime(message.CreatedAt), formatTime(message.UpdatedAt))
	return err
}

func (s *Store) ListSessions(ctx context.Context, limit int) ([]Session, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,started_at,ended_at,resume_path,model,answer_mode,markdown_path
		FROM interview_sessions ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := make([]Session, 0)
	for rows.Next() {
		var session Session
		var startedAt, endedAt string
		if err := rows.Scan(&session.ID, &startedAt, &endedAt, &session.ResumePath, &session.Model, &session.AnswerMode, &session.MarkdownPath); err != nil {
			return nil, err
		}
		session.StartedAt, session.EndedAt = parseTime(startedAt), parseTime(endedAt)
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *Store) ListMessages(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,turn_id,question_id,role,content,status,created_at,updated_at
		FROM interview_messages WHERE session_id=? ORDER BY created_at,id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]Message, 0)
	for rows.Next() {
		var message Message
		var role, status, createdAt, updatedAt string
		if err := rows.Scan(&message.ID, &message.SessionID, &message.TurnID, &message.QuestionID,
			&role, &message.Content, &status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		message.Role, message.Status = Role(role), MessageStatus(status)
		message.CreatedAt, message.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *Store) ListMessagesPage(ctx context.Context, sessionID string, before time.Time, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT id,session_id,turn_id,question_id,role,content,status,created_at,updated_at
		FROM interview_messages WHERE session_id=?`
	args := []any{sessionID}
	if !before.IsZero() {
		query += ` AND created_at < ?`
		args = append(args, formatTime(before))
	}
	query += ` ORDER BY created_at DESC,id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	descending := make([]Message, 0)
	for rows.Next() {
		var message Message
		var role, status, createdAt, updatedAt string
		if err := rows.Scan(&message.ID, &message.SessionID, &message.TurnID, &message.QuestionID,
			&role, &message.Content, &status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		message.Role, message.Status = Role(role), MessageStatus(status)
		message.CreatedAt, message.UpdatedAt = parseTime(createdAt), parseTime(updatedAt)
		descending = append(descending, message)
	}
	messages := make([]Message, len(descending))
	for index := range descending {
		messages[len(descending)-1-index] = descending[index]
	}
	return messages, rows.Err()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
