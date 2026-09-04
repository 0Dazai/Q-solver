package interview

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Aggregator struct {
	mu            sync.Mutex
	question      Question
	submitted     map[string]time.Time
	recentAnswers []string
	localUntil    time.Time
	lastFinal     string
	sentenceWait  time.Duration
	turnNumber    int
}

func NewAggregator(sentenceWait time.Duration) *Aggregator {
	if sentenceWait < 300*time.Millisecond {
		sentenceWait = 1300 * time.Millisecond
	}
	return &Aggregator{submitted: make(map[string]time.Time), sentenceWait: sentenceWait}
}

func (a *Aggregator) SetSentenceWait(wait time.Duration) {
	if wait >= 300*time.Millisecond && wait <= 10*time.Second {
		a.mu.Lock()
		a.sentenceWait = wait
		a.mu.Unlock()
	}
}

func (a *Aggregator) MarkLocalSpeech(until time.Time) {
	a.mu.Lock()
	if until.After(a.localUntil) {
		a.localUntil = until
	}
	a.mu.Unlock()
}

func (a *Aggregator) RememberAnswer(answer string) {
	if text := normalize(answer); text != "" {
		a.mu.Lock()
		a.recentAnswers = append(a.recentAnswers, text)
		if len(a.recentAnswers) > 4 {
			a.recentAnswers = a.recentAnswers[1:]
		}
		a.mu.Unlock()
	}
}

func (a *Aggregator) Accept(event TranscriptEvent) Question {
	a.mu.Lock()
	defer a.mu.Unlock()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	text := strings.TrimSpace(event.Text)
	if text == "" {
		return cloneQuestion(a.question)
	}
	if event.Kind == EventFinal {
		if _, alreadySubmitted := a.submitted[fingerprint(normalize(text))]; alreadySubmitted {
			// A delayed final can arrive after a stable partial has already been
			// submitted and the turn reset. Ignore it before it opens a new turn;
			// otherwise the duplicate turn can hold subsequent questions hostage.
			return cloneQuestion(a.question)
		}
	}
	if a.question.TurnID == "" {
		a.turnNumber++
		a.question.TurnID = fmt.Sprintf("turn-%d", a.turnNumber)
		a.question.QuestionID = fmt.Sprintf("question-%d", a.turnNumber)
	}
	if event.SessionID != "" {
		a.question.SessionID = event.SessionID
	}
	a.question.LastSpeechAt = event.Timestamp
	if event.Kind == EventPartial {
		a.question.Partial = text
		return cloneQuestion(a.question)
	}
	if normalize(text) == normalize(a.lastFinal) || containsNormalized(a.question.Finals, text) {
		return cloneQuestion(a.question)
	}
	a.lastFinal = text
	a.question.Finals = append(a.question.Finals, text)
	a.question.RawText = joinSegments(a.question.Finals)
	a.question.CorrectedText = a.question.RawText
	return cloneQuestion(a.question)
}

// Ready only yields one stable question per turn. Ambiguous text remains a
// candidate until a later final or the next round resets it.
func (a *Aggregator) Ready(now time.Time) (Question, string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	q := a.question
	if q.TurnID == "" || q.Submitted {
		return q, "等待语音片段", false
	}
	if now.Before(a.localUntil) {
		return q, "本机正在讲话", false
	}

	answerText := strings.TrimSpace(q.CorrectedText)
	readyReason := ""
	if len(q.Finals) == 0 {
		answerText = strings.TrimSpace(q.Partial)
		if answerText == "" {
			return q, "等待语音片段", false
		}
		if now.Sub(q.LastSpeechAt) < partialFallbackWait(a.sentenceWait) {
			return q, "等待 ASR 完成句末", false
		}
		if !hasQuestionIntent(answerText) {
			return q, "等待完整提问", false
		}
		// A realtime ASR connection may occasionally keep a complete sentence as
		// partial forever. Promote only stable question-like text so the whole
		// interview pipeline does not stall behind a missing sentence_end event.
		a.question.Finals = []string{answerText}
		a.question.RawText = answerText
		a.question.CorrectedText = answerText
		a.question.Partial = ""
		q = a.question
		readyReason = "ASR final 缺失，已用稳定识别文本提交"
	} else if now.Sub(q.LastSpeechAt) < adaptiveWait(answerText, a.sentenceWait) {
		return q, "等待句末稳定", false
	}
	if !hasQuestionIntent(answerText) {
		return q, "等待完整提问", false
	}
	text := normalize(answerText)
	if a.isEcho(text) {
		return q, "疑似本机或回答回声", false
	}
	fingerprint := fingerprint(text)
	if _, exists := a.submitted[fingerprint]; exists {
		return q, "最近问题重复", false
	}
	a.question.Fingerprint = fingerprint
	a.question.Submitted = true
	a.submitted[fingerprint] = now
	return cloneQuestion(a.question), readyReason, true
}

func partialFallbackWait(configured time.Duration) time.Duration {
	wait := configured + 700*time.Millisecond
	if wait < 2*time.Second {
		return 2 * time.Second
	}
	if wait > 3*time.Second {
		return 3 * time.Second
	}
	return wait
}

func adaptiveWait(text string, configured time.Duration) time.Duration {
	text = strings.TrimSpace(text)
	for _, suffix := range []string{"以及", "然后", "因为", "包括", "主要", "这个", "就是", "怎么"} {
		if strings.HasSuffix(text, suffix) {
			if configured > 1200*time.Millisecond {
				return configured
			}
			return 1200 * time.Millisecond
		}
	}
	if strings.HasSuffix(text, "？") || strings.HasSuffix(text, "?") {
		// ASR often emits a question-mark final before a short clarification.
		// A 700ms merge window is still small compared with model TTFT, while
		// avoiding a second model job for the same spoken question.
		return 700 * time.Millisecond
	}
	if configured >= 700*time.Millisecond {
		return configured
	}
	return 700 * time.Millisecond
}

func (a *Aggregator) ResetTurn() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.question = Question{}
	a.lastFinal = ""
}

func (a *Aggregator) Current() Question {
	a.mu.Lock()
	defer a.mu.Unlock()
	return cloneQuestion(a.question)
}

func (a *Aggregator) ReplaceCurrent(text string) Question {
	a.mu.Lock()
	defer a.mu.Unlock()
	text = strings.TrimSpace(text)
	if text == "" {
		return cloneQuestion(a.question)
	}
	if a.question.TurnID == "" {
		a.turnNumber++
		a.question.TurnID = fmt.Sprintf("turn-%d", a.turnNumber)
		a.question.QuestionID = fmt.Sprintf("question-%d", a.turnNumber)
	}
	a.question.CorrectedText, a.question.RawText = text, text
	a.question.Finals = []string{text}
	a.question.LastSpeechAt, a.question.Submitted = time.Now(), false
	return cloneQuestion(a.question)
}

func (a *Aggregator) SubmitCurrent(now time.Time) (Question, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.question.TurnID == "" || a.question.Submitted || !hasQuestionIntent(a.question.CorrectedText) {
		return cloneQuestion(a.question), false
	}
	fingerprint := fingerprint(normalize(a.question.CorrectedText))
	if _, exists := a.submitted[fingerprint]; exists {
		return cloneQuestion(a.question), false
	}
	a.question.Submitted, a.question.Fingerprint = true, fingerprint
	a.submitted[fingerprint] = now
	return cloneQuestion(a.question), true
}

// SplitBeforeFinal closes the current question when the next ASR final starts
// an explicitly new interviewer question. It deliberately does not split
// ordinary multi-part requests such as "介绍...，然后说说...".
func (a *Aggregator) SplitBeforeFinal(next TranscriptEvent, now time.Time) (Question, bool) {
	if next.Kind != EventFinal {
		return Question{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.question.TurnID == "" || a.question.Submitted || len(a.question.Finals) == 0 {
		return Question{}, false
	}
	if !isNewQuestionBoundary(a.question.CorrectedText, next.Text) || !hasQuestionIntent(a.question.CorrectedText) {
		return Question{}, false
	}
	fp := fingerprint(normalize(a.question.CorrectedText))
	if _, exists := a.submitted[fp]; exists {
		return Question{}, false
	}
	a.question.Submitted, a.question.Fingerprint = true, fp
	a.submitted[fp] = now
	question := cloneQuestion(a.question)
	a.question, a.lastFinal = Question{}, ""
	return question, true
}

func (a *Aggregator) isEcho(text string) bool {
	for _, answer := range a.recentAnswers {
		if similarity(text, answer) >= 0.82 {
			return true
		}
	}
	return false
}

func normalize(text string) string {
	replacer := strings.NewReplacer("，", "", "。", "", "？", "", "?", "", "！", "", "!", "", " ", "", "\n", "")
	return strings.ToLower(replacer.Replace(strings.TrimSpace(text)))
}

func joinSegments(parts []string) string { return strings.TrimSpace(strings.Join(parts, "")) }
func containsNormalized(parts []string, text string) bool {
	for _, part := range parts {
		if normalize(part) == normalize(text) {
			return true
		}
	}
	return false
}
func fingerprint(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum[:8])
}
func cloneQuestion(q Question) Question { q.Finals = append([]string(nil), q.Finals...); return q }
func hasQuestionIntent(text string) bool {
	if len([]rune(text)) < 4 {
		return false
	}
	if strings.ContainsAny(text, "?？") {
		return true
	}
	for _, marker := range []string{"吗", "么", "什么", "如何", "为什么", "介绍", "说说", "解释", "实现", "设计", "区别", "优缺点", "怎么", "请", "哪些", "哪几个", "是否", "能否", "了解吗", "讲一下", "谈谈"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
func isNewQuestionBoundary(current, next string) bool {
	current, next = strings.TrimSpace(current), strings.TrimSpace(next)
	if next == "" {
		return false
	}
	for _, marker := range []string{"下一个问题", "最后一个问题", "最后的问题", "接下来一个问题", "我们进入下一个"} {
		if strings.Contains(next, marker) {
			return true
		}
	}
	if !strings.ContainsAny(current, "?？") || !hasQuestionIntent(next) {
		return false
	}
	return strings.HasPrefix(next, "好") || strings.HasPrefix(next, "那么") || strings.HasPrefix(next, "那") || strings.HasPrefix(next, "然后")
}
func splitFinalAtQuestionBoundaries(text string) []string {
	remaining := strings.TrimSpace(text)
	if remaining == "" {
		return nil
	}
	parts := make([]string, 0, 2)
	for {
		index, markerLength := nextBoundaryMarker(remaining)
		if index <= 0 {
			break
		}
		parts = append(parts, strings.TrimSpace(remaining[:index]))
		remaining = strings.TrimSpace(remaining[index:])
		searchFrom := markerLength
		if searchFrom >= len(remaining) {
			break
		}
		innerIndex, _ := nextBoundaryMarker(remaining[searchFrom:])
		if innerIndex <= 0 {
			break
		}
		parts = append(parts, strings.TrimSpace(remaining[:searchFrom+innerIndex]))
		remaining = strings.TrimSpace(remaining[searchFrom+innerIndex:])
	}
	if remaining != "" {
		parts = append(parts, remaining)
	}
	return parts
}
func nextBoundaryMarker(text string) (int, int) {
	index, length := -1, 0
	for _, marker := range []string{"下一个问题", "最后一个问题", "最后的问题", "接下来一个问题", "我们进入下一个"} {
		if found := strings.Index(text, marker); found >= 0 && (index < 0 || found < index) {
			if found >= len("那么") && strings.HasSuffix(text[:found], "那么") {
				found -= len("那么")
			}
			index, length = found, len(marker)
		}
	}
	return index, length
}
func similarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return float64(min(len([]rune(a)), len([]rune(b)))) / float64(max(len([]rune(a)), len([]rune(b))))
	}
	seen := make(map[rune]bool)
	for _, r := range a {
		seen[r] = true
	}
	common := 0
	for _, r := range b {
		if seen[r] {
			common++
		}
	}
	return float64(common*2) / float64(len([]rune(a))+len([]rune(b)))
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
