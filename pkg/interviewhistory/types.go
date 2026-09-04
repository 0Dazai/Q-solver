package interviewhistory

import "time"

type Role string

const (
	RoleInterviewer  Role = "interviewer"
	RoleAISuggestion Role = "ai_suggestion"
	RoleCandidate    Role = "candidate"
)

type MessageStatus string

const (
	StatusStreaming MessageStatus = "streaming"
	StatusFinal     MessageStatus = "final"
	StatusCancelled MessageStatus = "cancelled"
	StatusError     MessageStatus = "error"
)

type Session struct {
	ID           string    `json:"id"`
	StartedAt    time.Time `json:"startedAt"`
	EndedAt      time.Time `json:"endedAt,omitempty"`
	ResumePath   string    `json:"resumePath,omitempty"`
	Model        string    `json:"model,omitempty"`
	AnswerMode   string    `json:"answerMode,omitempty"`
	MarkdownPath string    `json:"markdownPath,omitempty"`
}

type Message struct {
	ID         string        `json:"id"`
	SessionID  string        `json:"sessionId"`
	TurnID     string        `json:"turnId"`
	QuestionID string        `json:"questionId,omitempty"`
	Role       Role          `json:"role"`
	Content    string        `json:"content"`
	Status     MessageStatus `json:"status"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
}
