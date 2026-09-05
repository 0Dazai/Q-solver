package interview

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// QuestionLatency aggregates one question's end-to-end realtime pipeline
// timings. The content is sanitized by construction: only durations,
// identifiers, and the question text (truncated at the logging layer) are
// recorded — never prompt bodies, resume text, knowledge chunks, or
// credentials. The stage split lets a log reader tell apart ASR delay,
// aggregation delay, retrieval delay, queueing delay, connection delay,
// model inference delay, and SSE stalls.
type QuestionLatency struct {
	SessionID  string
	QuestionID string
	Question   string

	// ASRAt is the timestamp of the last ASR event that fed the turn;
	// ReadyAt is when the aggregator decided the question was stable.
	ASRAt                time.Time
	ReadyAt              time.Time
	SubmitAt             time.Time
	AggregateWaitMS      int64 // ReadyAt - ASRAt
	RetrievalMS          int64
	ContextBuildMS       int64
	QueueWaitMS          int64 // coordinator: enqueue -> executor dispatch
	ActiveMS             int64 // coordinator: executor dispatch -> stream end
	SubmitToFirstChunkMS int64 // submit -> first chunk forwarded to the frontend
	ChunkCount           int
	TotalMS              int64 // ASRAt -> terminal state
	Outcome              string
}

const (
	LatencyOutcomeCompleted = "completed"
	LatencyOutcomeError     = "error"
	LatencyOutcomeCanceled  = "canceled"
	LatencyOutcomeTimeout   = "timeout"
)

// LatencyTracker keeps in-flight per-question latency records. Records are
// registered at submit time and removed when the answer reaches a terminal
// state, so a crashed session cannot grow the map without bound.
type LatencyTracker struct {
	mu      sync.Mutex
	records map[string]*QuestionLatency
}

func NewLatencyTracker() *LatencyTracker {
	return &LatencyTracker{records: make(map[string]*QuestionLatency)}
}

func (t *LatencyTracker) Register(record *QuestionLatency) {
	if t == nil || record == nil || record.QuestionID == "" {
		return
	}
	t.mu.Lock()
	t.records[record.QuestionID] = record
	t.mu.Unlock()
}

func (t *LatencyTracker) Discard(questionID string) {
	if t == nil || questionID == "" {
		return
	}
	t.mu.Lock()
	delete(t.records, questionID)
	t.mu.Unlock()
}

func (t *LatencyTracker) MarkQueueTiming(questionID string, timing JobTiming) {
	if t == nil || questionID == "" {
		return
	}
	t.mu.Lock()
	if record := t.records[questionID]; record != nil {
		record.QueueWaitMS = timing.QueueWaitMS
		record.ActiveMS = timing.ActiveMS
	}
	t.mu.Unlock()
}

func (t *LatencyTracker) MarkAnswerChunk(questionID string, at time.Time) {
	if t == nil || questionID == "" {
		return
	}
	t.mu.Lock()
	if record := t.records[questionID]; record != nil {
		if record.SubmitToFirstChunkMS == 0 && !record.SubmitAt.IsZero() {
			record.SubmitToFirstChunkMS = at.Sub(record.SubmitAt).Milliseconds()
		}
		record.ChunkCount++
	}
	t.mu.Unlock()
}

// Finish removes the record and returns it for logging. The returned bool is
// false when no record exists (for example cancel paths before registration).
func (t *LatencyTracker) Finish(questionID string, outcome string) (*QuestionLatency, bool) {
	if t == nil || questionID == "" {
		return nil, false
	}
	t.mu.Lock()
	record := t.records[questionID]
	delete(t.records, questionID)
	t.mu.Unlock()
	if record == nil {
		return nil, false
	}
	record.Outcome = outcome
	if !record.ASRAt.IsZero() {
		record.TotalMS = time.Since(record.ASRAt).Milliseconds()
	}
	if record.ReadyAt.After(record.ASRAt) {
		record.AggregateWaitMS = record.ReadyAt.Sub(record.ASRAt).Milliseconds()
	}
	return record, true
}

// NormalizeLatencyOutcome maps an answer error to a sanitized outcome label.
func NormalizeLatencyOutcome(err error) string {
	switch {
	case err == nil:
		return LatencyOutcomeCompleted
	case errors.Is(err, context.DeadlineExceeded), strings.Contains(err.Error(), "超时"):
		return LatencyOutcomeTimeout
	default:
		return LatencyOutcomeError
	}
}
