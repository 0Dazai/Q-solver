package knowledge

import "time"

const MaxChunkRunes = 1600

type Chunk struct {
	ID          string    `json:"id"`
	DocumentID  string    `json:"documentId"`
	Path        string    `json:"path"`
	TitlePath   string    `json:"titlePath"`
	Content     string    `json:"content"`
	ContentHash string    `json:"contentHash"`
	Ordinal     int       `json:"ordinal"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type SearchResult struct {
	Chunk
	Source string  `json:"source"`
	Score  float64 `json:"score"`
}

type Document struct {
	ID          string    `json:"id"`
	Path        string    `json:"path"`
	ContentHash string    `json:"contentHash"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type AnswerMode string

const (
	AnswerModeGeneral        AnswerMode = "general"
	AnswerModeKnowledgeFirst AnswerMode = "knowledge_first"
	AnswerModeKnowledgeOnly  AnswerMode = "knowledge_only"
)
