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
	previousHint  string
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

// SetPreviousHint records the last submitted question so short elliptical
// follow-ups ("Redis呢", "项目里呢") can be recognized mid-interview. The first
// question of a session still goes through the strict intent gate.
func (a *Aggregator) SetPreviousHint(text string) {
	a.mu.Lock()
	a.previousHint = strings.TrimSpace(text)
	a.mu.Unlock()
}

// questionIntent distinguishes "no question" (noise, filler, echo) from "the
// current question is short". Mid-interview short follow-ups without ASR
// punctuation must still submit, or the turn never releases and pollutes the
// next question.
func (a *Aggregator) questionIntent(text string) bool {
	if hasQuestionIntent(text) {
		return true
	}
	return hasShortFollowUpIntent(text, a.previousHint)
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
	if event.Kind == EventPartial {
		// Re-emitted identical partials must not reset the stability clock,
		// otherwise a partial-only question can be delayed forever.
		unchanged := a.question.Partial != "" && normalize(text) == normalize(a.question.Partial)
		a.question.Partial = text
		if !unchanged {
			a.question.LastSpeechAt = event.Timestamp
		}
		return cloneQuestion(a.question)
	}
	a.question.LastSpeechAt = event.Timestamp
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
		if !a.questionIntent(answerText) {
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
	if !a.questionIntent(answerText) {
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

// SubmitCurrent handles the explicit manual submit action. The user already
// reviewed (usually edited) the text, so the intent gate must not reject short
// follow-ups such as "Redis呢" that the automatic path may still be holding.
func (a *Aggregator) SubmitCurrent(now time.Time) (Question, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.question.TurnID == "" || a.question.Submitted || strings.TrimSpace(a.question.CorrectedText) == "" {
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

// SplitBeforeFinal resolves the pending turn right before a new ASR final is
// appended. Once the pending text has had a full stability window it can never
// grow into the next question, so it must either submit now or be dropped:
// keeping blocked text (echo, filler, rejected intent) concatenated into the
// next final is what makes one stuck turn pollute every later question.
// Ordinary multi-part requests such as "介绍...，然后说说..." still append,
// because their stability window has not elapsed yet.
func (a *Aggregator) SplitBeforeFinal(next TranscriptEvent, now time.Time) (Question, bool) {
	if next.Kind != EventFinal {
		return Question{}, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.question.TurnID == "" || a.question.Submitted {
		return Question{}, false
	}
	text := strings.TrimSpace(a.question.CorrectedText)
	if text == "" {
		text = strings.TrimSpace(a.question.Partial)
	}
	if text == "" {
		return Question{}, false
	}
	if now.Sub(a.question.LastSpeechAt) < adaptiveWait(text, a.sentenceWait) {
		return Question{}, false
	}
	if a.questionIntent(text) && !a.isEcho(normalize(text)) {
		fp := fingerprint(normalize(text))
		if _, exists := a.submitted[fp]; exists {
			a.question, a.lastFinal = Question{}, ""
			return Question{}, false
		}
		question := a.question
		question.Finals = []string{text}
		question.RawText, question.CorrectedText = text, text
		question.Partial = ""
		question.Submitted, question.Fingerprint = true, fp
		a.submitted[fp] = now
		a.question, a.lastFinal = Question{}, ""
		return question, true
	}
	a.question, a.lastFinal = Question{}, ""
	return Question{}, false
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

// Short elliptical follow-ups that interviewers really say. They only submit
// once a previous question exists, so session-opening noise cannot trigger an
// answer, and pure filler ("好的", "嗯") never passes.
var shortFollowUpEndingParticles = []string{"呢", "吧", "吗", "么"}
var shortFollowUpContentMarkers = []string{"为什么", "怎么", "如何", "具体", "原理", "细节", "原因", "举例", "说说", "展开", "底层"}
var pureFillerUtterances = map[string]bool{
	"好的": true, "好": true, "嗯": true, "嗯嗯": true, "嗯好": true, "哦": true, "噢": true,
	"啊": true, "呃": true, "行": true, "可以": true, "对": true, "是的": true, "是": true,
	"谢谢": true, "谢谢啦": true, "辛苦了": true, "辛苦啦": true, "明白了": true, "明白": true,
	"没问题": true, "ok": true, "okay": true, "okk": true, "好滴": true, "好嘞": true,
	"收到": true, "哈哈": true, "哈哈哈": true, "嗯呐": true, "对对对": true, "好好好": true,
}

func hasShortFollowUpIntent(text, previousHint string) bool {
	if strings.TrimSpace(previousHint) == "" {
		return false
	}
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) < 2 || len(runes) > 14 {
		return false
	}
	if isPureFillerUtterance(text) {
		return false
	}
	if strings.ContainsAny(text, "?？") {
		return true
	}
	for _, particle := range shortFollowUpEndingParticles {
		if strings.HasSuffix(text, particle) {
			return true
		}
	}
	for _, marker := range shortFollowUpContentMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	// "Redis呢", "线程池呢", "GC了解吧" survive even when ASR drops punctuation.
	return len(extractTechnicalTerms(text)) > 0
}

func isPureFillerUtterance(text string) bool {
	if len([]rune(text)) > 8 {
		return false
	}
	return pureFillerUtterances[normalize(text)]
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
