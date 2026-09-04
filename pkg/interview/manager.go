package interview

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"Q-Solver/pkg/config"
	"Q-Solver/pkg/interviewhistory"
	"Q-Solver/pkg/knowledge"
)

const (
	maxReconnectAttempts = 3
	maxReconnectCycles   = 3
)

type Manager struct {
	config   func() config.Config
	emit     func(string, ...any)
	executor AnswerExecutor

	lifecycleMu        sync.Mutex
	mu                 sync.Mutex
	running            bool
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	capture            *AudioCapture
	mic                *MicActivity
	asr                ASRClient
	localASR           ASRClient
	aggregator         *Aggregator
	answers            *AnswerCoordinator
	status             Status
	localFinals        []string
	knowledgeRetriever knowledge.Retriever
	knowledgeContext   KnowledgeContextProvider
	candidateProfile   CandidateProfile
	profileKeywords    ProfileKeywords
	historyRecorder    HistoryRecorder
	retrievalLog       *RetrievalLogger
	activeTurnID       string
	activeQuestionID   string
	candidateAnswers   map[string]string
	deepenRequested    bool
	retrievalCache     map[string][]knowledge.SearchResult
	retrievalCacheMu   sync.Mutex
	answerChunkStarted map[string]bool
	previousQuestion   string

	reconnectCycles      int
	localReconnectCycles int
	asrConnectedAt       time.Time
	eventSequence        uint64
}

type HistoryRecorder interface {
	StartSession(interviewhistory.Session) error
	UpsertMessage(interviewhistory.Message) error
	FinishSession(string, time.Time) error
}

type KnowledgeContextProvider interface {
	PinnedMarkdownContext(context.Context, int) ([]string, error)
}

func (m *Manager) SetKnowledgeRetriever(retriever knowledge.Retriever) {
	m.mu.Lock()
	m.knowledgeRetriever = retriever
	m.mu.Unlock()
}

func (m *Manager) SetKnowledgeContextProvider(provider KnowledgeContextProvider) {
	m.mu.Lock()
	m.knowledgeContext = provider
	m.mu.Unlock()
}

func (m *Manager) SetHistoryRecorder(recorder HistoryRecorder) {
	m.mu.Lock()
	m.historyRecorder = recorder
	m.mu.Unlock()
}

func NewManager(configFn func() config.Config, emit func(string, ...any), executor AnswerExecutor) *Manager {
	manager := &Manager{
		config: configFn, emit: emit, executor: executor, retrievalLog: NewRetrievalLogger(),
		answerChunkStarted: make(map[string]bool),
	}
	if httpExecutor, ok := executor.(*HTTPAnswerExecutor); ok && manager.retrievalLog != nil {
		httpExecutor.OnTiming = manager.retrievalLog.LogAnswerTiming
	}
	manager.answers = NewAnswerCoordinator(executor, func() config.AnswerModelConfig {
		profile := withInterviewProfile(configFn().InterviewModel)
		if manager.deepenRequested {
			profile.ReasoningLevel = "medium"
		}
		return profile
	}, manager.handleAnswerState, manager.emitAnswerChunk, manager.finishAnswer)
	return manager
}

func (m *Manager) Start(parent context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.start(parent)
}

func (m *Manager) start(parent context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	cfg := m.config()
	cfg.Normalize()
	ctx, cancel := context.WithCancel(parent)
	m.ctx, m.cancel, m.running = ctx, cancel, true
	m.eventSequence = 0
	m.status = Status{SessionID: newSessionID(), Running: true, SystemAudio: "starting", ASR: "connecting"}
	m.aggregator = NewAggregator(time.Duration(cfg.Transcription.SentenceWaitMS) * time.Millisecond)
	m.localFinals = nil
	m.candidateProfile = buildCandidateProfile(cfg.ResumeContent, nil, 12000)
	m.profileKeywords = ExtractProfileKeywords(m.candidateProfile.Context, 40)
	m.candidateAnswers = make(map[string]string)
	m.answerChunkStarted = make(map[string]bool)
	m.retrievalCache = make(map[string][]knowledge.SearchResult)
	m.previousQuestion = ""
	m.reconnectCycles, m.localReconnectCycles = 0, 0
	m.asrConnectedAt = time.Now()
	sessionID := m.status.SessionID
	recorder := m.historyRecorder
	contextProvider := m.knowledgeContext
	m.mu.Unlock()
	if contextProvider != nil {
		preloadCtx, preloadCancel := context.WithTimeout(ctx, 800*time.Millisecond)
		pinnedMarkdown, preloadErr := contextProvider.PinnedMarkdownContext(preloadCtx, 6000)
		preloadCancel()
		if preloadErr == nil {
			m.mu.Lock()
			m.candidateProfile = buildCandidateProfile(cfg.ResumeContent, pinnedMarkdown, 12000)
			m.profileKeywords = ExtractProfileKeywords(m.candidateProfile.Context, 40)
			m.mu.Unlock()
		}
	}
	if recorder != nil {
		_ = recorder.StartSession(interviewhistory.Session{
			ID: sessionID, StartedAt: time.Now(), ResumePath: cfg.ResumePath,
			Model: cfg.InterviewModel.Model, AnswerMode: cfg.Knowledge.AnswerMode,
		})
	}

	asr, engine, err := newTranscriptionClient(ctx, cfg.Transcription, false)
	if err != nil {
		m.finishStartFailure(err)
		return err
	}
	localASR, _, err := newTranscriptionClient(ctx, cfg.Transcription, true)
	if err != nil {
		_ = asr.Close()
		m.finishStartFailure(err)
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(err)
		return err
	}
	capture, err := NewLoopbackCapture()
	if err != nil {
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(err)
		return err
	}
	if err = capture.Start(); err != nil {
		capture.Close()
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(err)
		return err
	}
	if err := ctx.Err(); err != nil {
		capture.Close()
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(err)
		return err
	}
	mic, err := NewMicActivity()
	if err != nil {
		capture.Close()
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(err)
		return err
	}
	if err = mic.Start(); err != nil {
		mic.Close()
		capture.Close()
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(err)
		return err
	}
	m.mu.Lock()
	if ctx.Err() != nil {
		m.mu.Unlock()
		mic.Close()
		capture.Close()
		_ = asr.Close()
		_ = localASR.Close()
		m.finishStartFailure(ctx.Err())
		return ctx.Err()
	}
	m.asr, m.localASR, m.capture, m.mic = asr, localASR, capture, mic
	m.status.SystemAudio, m.status.ASR, m.status.Engine = "running", "connected", engine
	m.mu.Unlock()
	m.emitStatus()
	m.wg.Add(5)
	go m.audioLoop()
	go m.asrLoop(asr)
	go m.micAudioLoop()
	go m.localASRLoop(localASR)
	go m.stabilityLoop()
	return nil
}

func (m *Manager) finishStartFailure(err error) {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	sessionID, recorder := m.status.SessionID, m.historyRecorder
	m.running = false
	m.status = Status{ASR: "error", SystemAudio: "stopped", LastError: err.Error()}
	m.mu.Unlock()
	if recorder != nil && sessionID != "" {
		_ = recorder.FinishSession(sessionID, time.Now())
	}
	m.emitStatus()
}
func (m *Manager) Stop() {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.stop()
}

func (m *Manager) stop() {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return
	}
	cancel, asr, localASR, capture, mic, answers := m.cancel, m.asr, m.localASR, m.capture, m.mic, m.answers
	sessionID, recorder := m.status.SessionID, m.historyRecorder
	m.mu.Unlock()
	if answers != nil {
		answers.CancelAll()
	}
	if cancel != nil {
		cancel()
	}
	if asr != nil {
		_ = asr.Close()
	}
	if localASR != nil {
		_ = localASR.Close()
	}
	if capture != nil {
		capture.Close()
	}
	if mic != nil {
		mic.Close()
	}
	if answers != nil {
		answers.Wait()
	}
	m.wg.Wait()
	m.mu.Lock()
	m.running, m.asr, m.localASR, m.capture, m.mic = false, nil, nil, nil, nil
	m.localFinals = nil
	lastError := m.status.LastError
	m.status = Status{SystemAudio: "stopped", ASR: "disconnected", Suppression: "监听已暂停", LastError: lastError}
	m.mu.Unlock()
	if recorder != nil && sessionID != "" {
		_ = recorder.FinishSession(sessionID, time.Now())
	}
	m.emitStatus()
}
func (m *Manager) Restart(parent context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.stop()
	return m.start(parent)
}
func (m *Manager) Toggle(parent context.Context) error {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	if m.isRunning() {
		m.stop()
		return nil
	}
	return m.start(parent)
}
func (m *Manager) isRunning() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.running }
func (m *Manager) IsRunning() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.running }
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.status
	if m.capture != nil {
		s.DroppedPackets, s.QueuedPackets = m.capture.DroppedPackets(), m.capture.QueuedPackets()
	}
	if m.mic != nil {
		s.Microphone = m.mic.IsActive()
	}
	return s
}
func (m *Manager) CancelAnswer() {
	if m.answers != nil {
		m.answers.Cancel()
	}
	m.mu.Lock()
	m.status.Suppression = "正在停止当前回答"
	m.mu.Unlock()
	m.emitStatus()
}
func (m *Manager) CurrentQuestion() Question {
	m.mu.Lock()
	a := m.aggregator
	m.mu.Unlock()
	if a == nil {
		return Question{}
	}
	return a.Current()
}
func (m *Manager) EditQuestion(text string) {
	m.mu.Lock()
	a := m.aggregator
	m.mu.Unlock()
	if a == nil {
		return
	}
	if strings.Contains(text, "请进一步展开") || strings.Contains(text, "深入分析") || strings.Contains(text, "补充原理") {
		m.mu.Lock()
		m.deepenRequested = true
		m.mu.Unlock()
	}
	question := a.ReplaceCurrent(text)
	m.mu.Lock()
	m.status.Candidate, m.status.Partial, m.status.Suppression = question.CorrectedText, "", "等待手动提交"
	m.mu.Unlock()
	m.emit("interview:question", question)
	m.emitStatus()
}
func (m *Manager) SubmitCurrent() {
	m.mu.Lock()
	a := m.aggregator
	m.mu.Unlock()
	if a == nil {
		return
	}
	if question, ok := a.SubmitCurrent(time.Now()); ok {
		m.submit(question)
		return
	}
	m.mu.Lock()
	m.status.Suppression = "当前候选问题尚不可提交"
	m.mu.Unlock()
	m.emitStatus()
}

func (m *Manager) audioLoop() {
	defer m.wg.Done()
	m.mu.Lock()
	capture := m.capture
	ctx := m.ctx
	m.mu.Unlock()
	if capture == nil {
		return
	}
	runAudioSender(ctx, capture.Packets(), func() ASRClient {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.asr
	}, m.setError)
}
func (m *Manager) micAudioLoop() {
	defer m.wg.Done()
	m.mu.Lock()
	mic, ctx := m.mic, m.ctx
	m.mu.Unlock()
	if mic == nil {
		return
	}
	runAudioSender(ctx, mic.Packets(), func() ASRClient {
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.localASR
	}, m.setError)
}
func (m *Manager) asrLoop(initial ASRClient) {
	defer m.wg.Done()
	asr := initial
	for {
		m.mu.Lock()
		ctx := m.ctx
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case event, ok := <-asr.Events():
			if !ok {
				if !m.reconnect(&asr) {
					return
				}
				continue
			}
			m.accept(event)
		case err, ok := <-asr.Errors():
			if ok && err != nil {
				if !m.reconnect(&asr) {
					m.setError(err)
					return
				}
			}
		}
	}
}
func (m *Manager) localASRLoop(initial ASRClient) {
	defer m.wg.Done()
	asr := initial
	for {
		m.mu.Lock()
		ctx := m.ctx
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case event, ok := <-asr.Events():
			if !ok {
				if !m.reconnectLocal(&asr) {
					return
				}
				continue
			}
			m.acceptLocal(event)
		case err, ok := <-asr.Errors():
			if ok && err != nil {
				if !m.reconnectLocal(&asr) {
					m.setError(err)
					return
				}
			}
		}
	}
}
func (m *Manager) reconnect(target *ASRClient) bool {
	m.mu.Lock()
	ctx := m.ctx
	if !m.running || ctx == nil || ctx.Err() != nil {
		m.mu.Unlock()
		return false
	}
	if time.Since(m.asrConnectedAt) >= 30*time.Second {
		m.reconnectCycles = 0
	}
	m.reconnectCycles++
	if m.reconnectCycles > maxReconnectCycles {
		m.status.ASR = "error"
		m.status.LastError = "Windows 系统语音识别连续退出，已停止监听"
		m.status.Suppression = "请检查 Windows 语音识别和默认播放设备后重新开始监听"
		m.mu.Unlock()
		m.emitStatus()
		go m.Stop()
		return false
	}
	if m.asr == *target {
		m.asr = nil
	}
	m.status.ASR = "reconnecting"
	m.mu.Unlock()
	_ = (*target).Close()
	m.emitStatus()
	for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
		cfg := m.config()
		client, _, err := newTranscriptionClient(ctx, cfg.Transcription, false)
		if err == nil {
			*target = client
			m.mu.Lock()
			m.asr, m.status.ASR, m.asrConnectedAt = client, "connected", time.Now()
			m.mu.Unlock()
			m.emitStatus()
			return true
		}
	}
	return false
}
func (m *Manager) reconnectLocal(target *ASRClient) bool {
	m.mu.Lock()
	ctx := m.ctx
	if !m.running || ctx == nil || ctx.Err() != nil {
		m.mu.Unlock()
		return false
	}
	m.localReconnectCycles++
	if m.localReconnectCycles > maxReconnectCycles {
		m.status.LastError = "本机回答的 Windows 语音识别连续退出，已停止本机转写"
		m.status.Suppression = "本机回答转写已暂停；系统音频仍可继续监听"
		m.mu.Unlock()
		m.emitStatus()
		return false
	}
	if m.localASR == *target {
		m.localASR = nil
	}
	m.mu.Unlock()
	_ = (*target).Close()
	for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
		cfg := m.config()
		client, _, err := newTranscriptionClient(ctx, cfg.Transcription, true)
		if err == nil {
			*target = client
			m.mu.Lock()
			m.localASR = client
			m.mu.Unlock()
			return true
		}
	}
	return false
}
func (m *Manager) stabilityLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case now := <-ticker.C:
			m.mu.Lock()
			a, mic, cfg := m.aggregator, m.mic, m.config()
			m.mu.Unlock()
			if a == nil {
				continue
			}
			if mic != nil && mic.IsActive() {
				a.MarkLocalSpeech(mic.ActiveUntil())
			}
			question, reason, ready := a.Ready(now)
			m.mu.Lock()
			changed := m.status.Candidate != question.CorrectedText || m.status.Suppression != reason
			m.status.Candidate, m.status.Suppression = question.CorrectedText, reason
			m.mu.Unlock()
			if ready && cfg.Transcription.AutoSubmit {
				if m.retrievalLog != nil {
					detail := "ASR final 自动提交"
					if reason != "" {
						detail = reason
					}
					m.retrievalLog.LogPipeline("问题已提交", question.CorrectedText, detail)
				}
				m.submit(question)
			} else if changed {
				m.emitStatus()
			}
		}
	}
}
func (m *Manager) accept(event TranscriptEvent) {
	m.mu.Lock()
	a := m.aggregator
	event.SessionID = m.status.SessionID
	m.mu.Unlock()
	events := []TranscriptEvent{event}
	if event.Kind == EventFinal {
		if parts := splitFinalAtQuestionBoundaries(event.Text); len(parts) > 1 {
			events = make([]TranscriptEvent, 0, len(parts))
			for index, part := range parts {
				segment := event
				segment.Text = part
				segment.Sequence = event.Sequence*100 + int64(index)
				events = append(events, segment)
			}
		}
	}
	for _, current := range events {
		if a != nil {
			if completed, ok := a.SplitBeforeFinal(current, time.Now()); ok {
				m.submit(completed)
			}
			question := a.Accept(current)
			question.SessionID = current.SessionID
			m.mu.Lock()
			m.status.Partial = question.Partial
			m.status.Candidate = question.CorrectedText
			if current.Kind == EventFinal {
				m.status.Partial = ""
				m.status.Suppression = "等待完整问题"
			}
			m.mu.Unlock()
			m.emit("interview:question", question)
		}
		m.emit("interview:transcript", current)
		m.emitStatus()
	}
}
func (m *Manager) acceptLocal(event TranscriptEvent) {
	m.mu.Lock()
	event.SessionID = m.status.SessionID
	turnID, questionID := m.activeTurnID, m.activeQuestionID
	m.mu.Unlock()
	m.emit("interview:local-transcript", event)
	if turnID != "" {
		content := strings.TrimSpace(event.Text)
		if content != "" {
			timeline := TimelineEvent{
				MessageID: timelineMessageID(event.SessionID, "candidate", turnID), SessionID: event.SessionID,
				TurnID: turnID, QuestionID: questionID, Role: string(interviewhistory.RoleCandidate),
				Content: content, Status: string(interviewhistory.StatusStreaming), CreatedAt: time.Now(),
			}
			if event.Kind == EventFinal {
				m.mu.Lock()
				existing := strings.TrimSpace(m.candidateAnswers[turnID])
				if existing != "" && normalize(existing) != normalize(content) {
					existing += "\n"
				}
				if normalize(existing) != normalize(content) {
					existing += content
				}
				m.candidateAnswers[turnID] = existing
				recorder := m.historyRecorder
				m.mu.Unlock()
				timeline.Content, timeline.Status = existing, string(interviewhistory.StatusFinal)
				if recorder != nil {
					_ = recorder.UpsertMessage(historyMessage(timeline))
				}
			}
			m.emit("interview:timeline", timeline)
		}
	}
	if event.Kind != EventFinal {
		return
	}
	text := strings.TrimSpace(event.Text)
	if text == "" {
		return
	}
	m.mu.Lock()
	if len(m.localFinals) == 0 || normalize(m.localFinals[len(m.localFinals)-1]) != normalize(text) {
		m.localFinals = append(m.localFinals, text)
		if len(m.localFinals) > maxLocalResponseSegments {
			m.localFinals = m.localFinals[len(m.localFinals)-maxLocalResponseSegments:]
		}
	}
	m.mu.Unlock()
}
func (m *Manager) submit(question Question) {
	if question.Submitted == false || question.CorrectedText == "" {
		return
	}
	m.mu.Lock()
	ctx := m.ctx
	m.status.Submitted, m.status.Candidate, m.status.Partial = question.CorrectedText, "", ""
	m.status.Suppression, m.status.Answering = "", true
	m.activeTurnID, m.activeQuestionID = question.TurnID, question.QuestionID
	question.SessionID = m.status.SessionID
	aggregator := m.aggregator
	answers := m.answers
	retriever := m.knowledgeRetriever
	candidateProfile := m.candidateProfile
	profileKeywords := m.profileKeywords
	previousQuestion := m.previousQuestion
	deepenRequested := m.deepenRequested
	cfg := m.config()
	if answers == nil {
		m.status.Answering = false
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	// Release the aggregator before retrieval and API queueing. Otherwise a new
	// interviewer sentence arriving during those steps is appended to the
	// already-submitted turn and then discarded by the later reset.
	if aggregator != nil {
		aggregator.ResetTurn()
	}
	questionEvent := TimelineEvent{
		MessageID: timelineMessageID(question.SessionID, "question", question.QuestionID), SessionID: question.SessionID,
		TurnID: question.TurnID, QuestionID: question.QuestionID,
		Role: string(interviewhistory.RoleInterviewer), Content: question.CorrectedText,
		Status: string(interviewhistory.StatusFinal), CreatedAt: time.Now(),
	}
	m.emit("interview:timeline", questionEvent)
	if recorder := m.historyRecorder; recorder != nil {
		_ = recorder.UpsertMessage(historyMessage(questionEvent))
	}
	localFinals := m.consumeLocalFinals()
	mode := knowledge.AnswerMode(cfg.Knowledge.AnswerMode)
	plan := planQuestion(question.CorrectedText, previousQuestion, profileKeywords, string(mode))
	if deepenRequested {
		plan.MaxTokens = 900
		plan.Instruction += " 本轮用户要求深入分析，请补充原理、边界条件和权衡，控制在 450-750 个汉字。"
	}
	m.mu.Lock()
	m.previousQuestion = question.CorrectedText
	m.mu.Unlock()

	var results []knowledge.SearchResult
	var normalizedScores []float64
	var retrievalErr error
	var searchDuration int64
	cacheHit := false
	expandedQuery := strings.Join(plan.QueryTerms, " ")
	if expandedQuery == "" {
		expandedQuery = question.CorrectedText
	}
	decision := "跳过检索"
	if retriever != nil && plan.ShouldRetrieve {
		searchStart := time.Now()
		cacheKey := normalize(expandedQuery)
		var cachedResults []knowledge.SearchResult
		m.retrievalCacheMu.Lock()
		if cached, ok := m.retrievalCache[cacheKey]; ok {
			cachedResults = cached
			cacheHit = true
		}
		m.retrievalCacheMu.Unlock()

		if cacheHit {
			results = cachedResults
			searchDuration = time.Since(searchStart).Milliseconds()
		} else {
			searchCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
			rawResults, searchErr := retriever.Search(searchCtx, expandedQuery, 20)
			cancel()
			retrievalErr = searchErr
			searchDuration = time.Since(searchStart).Milliseconds()
			if retrievalErr == nil {
				reranked, scores := RerankResults(rawResults, expandedQuery, ProfileKeywords{Terms: plan.QueryTerms})
				scoreByID := make(map[string]float64, len(reranked))
				for index, result := range reranked {
					if index < len(scores) {
						scoreByID[result.ID] = scores[index]
					}
				}
				gateTerms := plan.CurrentQueryTerms
				if len(gateTerms) == 0 {
					gateTerms = plan.QueryTerms
				}
				results = FilterRelevantResults(reranked, gateTerms)
				normalizedScores = make([]float64, 0, len(results))
				for _, result := range results {
					normalizedScores = append(normalizedScores, scoreByID[result.ID])
				}
				m.retrievalCacheMu.Lock()
				if len(m.retrievalCache) < 20 {
					m.retrievalCache[cacheKey] = results
				}
				m.retrievalCacheMu.Unlock()
			}
		}
		decision = "执行检索并通过质量门控"
		if retrievalErr == nil && len(results) == 0 {
			decision = "执行检索但无高质量命中"
		}
	}
	answerInput, citations := buildPlannedAnswerInput(question.CorrectedText, localFinals, candidateProfile, plan, mode, results)
	if len(citations) > 0 {
		m.emit("interview:sources", map[string]any{
			"sessionId": m.Status().SessionID, "questionId": question.QuestionID, "items": citations,
		})
	}
	if m.retrievalLog != nil {
		m.retrievalLog.Log(RetrievalLogEntry{
			Query: question.CorrectedText, ExpandedQuery: expandedQuery, QuestionType: plan.Type,
			Decision: decision, InputRunes: len([]rune(answerInput)), Results: results,
			NormalizedScores: normalizedScores, InjectedCount: len(citations), Err: retrievalErr,
			DurationMS: searchDuration, CacheHit: cacheHit,
		})
	}
	m.emit("interview:question", question)
	m.emitStatus()
	answerProfile := withInterviewProfile(cfg.InterviewModel)
	answerProfile.MaxTokens = plan.MaxTokens
	if deepenRequested {
		answerProfile.ThinkingMode = "enabled"
		answerProfile.ReasoningLevel = "medium"
	}
	if m.retrievalLog != nil {
		m.retrievalLog.LogPipeline("API 已入队", question.CorrectedText,
			fmt.Sprintf("provider=%s model=%s protocol=%s type=%s", answerProfile.Provider, answerProfile.Model, answerProfile.Protocol, plan.Type))
	}
	answers.SubmitWithProfile(ctx, question, answerInput, answerProfile)
	m.mu.Lock()
	m.deepenRequested = false
	m.mu.Unlock()
}

func (m *Manager) handleAnswerState(question Question, state AnswerQueueState) {
	content := "正在组织回答……"
	suppression := "正在生成回答"
	if state == AnswerQueued {
		content = "已排队，等待上一题回答完成……"
		suppression = "新问题已排队，当前回答继续生成"
	}
	key := timelineMessageID(question.SessionID, "answer", question.QuestionID)
	m.mu.Lock()
	m.status.Answering = true
	m.status.Suppression = suppression
	if state == AnswerStarted {
		m.answerChunkStarted[key] = false
	}
	m.mu.Unlock()
	m.emit("interview:timeline", TimelineEvent{
		MessageID: key, SessionID: question.SessionID, TurnID: question.TurnID, QuestionID: question.QuestionID,
		Role: string(interviewhistory.RoleAISuggestion), Content: content,
		Status: string(interviewhistory.StatusStreaming), Append: false, CreatedAt: time.Now(),
	})
	m.emitStatus()
}

func (m *Manager) emitAnswerChunk(question Question, chunk string) {
	key := timelineMessageID(question.SessionID, "answer", question.QuestionID)
	m.mu.Lock()
	hasStarted := m.answerChunkStarted[key]
	m.answerChunkStarted[key] = true
	m.mu.Unlock()
	m.emit("interview:answer", map[string]string{"sessionId": question.SessionID, "questionId": question.QuestionID, "text": chunk})
	m.emit("interview:timeline", TimelineEvent{
		MessageID: key, SessionID: question.SessionID,
		TurnID: question.TurnID, QuestionID: question.QuestionID,
		Role: string(interviewhistory.RoleAISuggestion), Content: chunk,
		Status: string(interviewhistory.StatusStreaming), Append: hasStarted, CreatedAt: time.Now(),
	})
}

func (m *Manager) finishAnswer(question Question, answer string, err error) {
	key := timelineMessageID(question.SessionID, "answer", question.QuestionID)
	busy := m.answers != nil && m.answers.HasWork()
	m.mu.Lock()
	a := m.aggregator
	recorder := m.historyRecorder
	m.status.Answering = busy
	delete(m.answerChunkStarted, key)
	if !busy {
		m.status.Suppression = ""
	}
	m.mu.Unlock()
	var event TimelineEvent
	if err == nil && answer != "" && a != nil {
		a.RememberAnswer(answer)
		event = TimelineEvent{
			MessageID: key, SessionID: question.SessionID,
			TurnID: question.TurnID, QuestionID: question.QuestionID,
			Role: string(interviewhistory.RoleAISuggestion), Content: answer,
			Status: string(interviewhistory.StatusFinal), CreatedAt: time.Now(),
		}
	} else {
		content := "模型未返回内容，请重新提交。"
		status := interviewhistory.StatusError
		switch {
		case errors.Is(err, context.Canceled):
			content = "已停止生成。"
			status = interviewhistory.StatusCancelled
		case err != nil:
			content = "回答生成失败：" + err.Error()
			m.mu.Lock()
			if m.status.LastError == "" {
				m.status.LastError = err.Error()
			}
			m.mu.Unlock()
		}
		event = TimelineEvent{
			MessageID: key, SessionID: question.SessionID,
			TurnID: question.TurnID, QuestionID: question.QuestionID,
			Role: string(interviewhistory.RoleAISuggestion), Content: content,
			Status: string(status), CreatedAt: time.Now(),
		}
	}
	if event.MessageID != "" {
		m.emit("interview:timeline", event)
		if recorder != nil {
			_ = recorder.UpsertMessage(historyMessage(event))
		}
	}
	m.emitStatus()
}

func timelineMessageID(sessionID, kind, entityID string) string {
	return sessionID + ":" + kind + ":" + entityID
}

func historyMessage(event TimelineEvent) interviewhistory.Message {
	return interviewhistory.Message{
		ID: event.MessageID, SessionID: event.SessionID, TurnID: event.TurnID,
		QuestionID: event.QuestionID, Role: interviewhistory.Role(event.Role),
		Content: event.Content, Status: interviewhistory.MessageStatus(event.Status),
		CreatedAt: event.CreatedAt, UpdatedAt: time.Now(),
	}
}
func (m *Manager) consumeLocalFinals() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	finals := append([]string(nil), m.localFinals...)
	m.localFinals = nil
	return finals
}
func (m *Manager) setError(err error) {
	m.mu.Lock()
	if m.status.LastError == "" {
		m.status.LastError = err.Error()
	}
	m.mu.Unlock()
	m.emitStatus()
}
func (m *Manager) emitStatus() {
	m.mu.Lock()
	m.eventSequence++
	m.status.EventSequence = m.eventSequence
	m.mu.Unlock()
	m.emit("interview:status", m.Status())
}

func newTranscriptionClient(ctx context.Context, cfg config.TranscriptionConfig, microphone bool) (ASRClient, string, error) {
	engine := strings.ToLower(strings.TrimSpace(cfg.Engine))
	useWindows := engine == "windows" || (engine != "dashscope" && strings.TrimSpace(cfg.APIKey) == "")
	if useWindows {
		if microphone {
			client, err := NewSystemMicrophoneASR(ctx, cfg.Language)
			return client, "windows", err
		}
		client, err := NewSystemLoopbackASR(ctx, cfg.Language)
		return client, "windows", err
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, "dashscope", errors.New("已选择千问转写，但未配置千问 API Key")
	}
	client, err := ConnectDashScope(ctx, cfg)
	return client, "dashscope", err
}
