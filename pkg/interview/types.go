package interview

import "time"

const (
	SampleRate       = 16000
	BytesPerSample   = 2
	PacketDurationMS = 30
	PacketSize       = SampleRate * BytesPerSample * PacketDurationMS / 1000
)

type EventKind string

const (
	EventPartial EventKind = "partial"
	EventFinal   EventKind = "final"
)

type TranscriptEvent struct {
	Kind      EventKind
	Text      string
	Sequence  int64
	Timestamp time.Time
	SessionID string `json:"sessionId,omitempty"`
}

type Status struct {
	SessionID      string `json:"sessionId,omitempty"`
	EventSequence  uint64 `json:"eventSequence,omitempty"`
	Running        bool   `json:"running"`
	SystemAudio    string `json:"systemAudio"`
	Microphone     bool   `json:"microphone"`
	ASR            string `json:"asr"`
	Engine         string `json:"engine,omitempty"`
	DroppedPackets uint64 `json:"droppedPackets"`
	QueuedPackets  int    `json:"queuedPackets"`
	Partial        string `json:"partial,omitempty"`
	Candidate      string `json:"candidate,omitempty"`
	Submitted      string `json:"submitted,omitempty"`
	Suppression    string `json:"suppression,omitempty"`
	Answering      bool   `json:"answering"`
	LastError      string `json:"lastError,omitempty"`
}

type Question struct {
	SessionID     string    `json:"sessionId,omitempty"`
	TurnID        string    `json:"turnId"`
	QuestionID    string    `json:"questionId"`
	Partial       string    `json:"partial"`
	Finals        []string  `json:"finals"`
	RawText       string    `json:"rawText"`
	CorrectedText string    `json:"correctedText"`
	LastSpeechAt  time.Time `json:"lastSpeechAt"`
	Submitted     bool      `json:"submitted"`
	Fingerprint   string    `json:"fingerprint"`
}

type TimelineEvent struct {
	MessageID  string    `json:"messageId"`
	SessionID  string    `json:"sessionId"`
	TurnID     string    `json:"turnId"`
	QuestionID string    `json:"questionId,omitempty"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	Status     string    `json:"status"`
	Append     bool      `json:"append"`
	CreatedAt  time.Time `json:"createdAt"`
}
