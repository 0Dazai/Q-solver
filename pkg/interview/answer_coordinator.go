package interview

import (
	"context"
	"sync"

	"Q-Solver/pkg/config"
)

const (
	defaultAnswerConcurrency = 2
	maxNewestPriorityStreak  = 2
)

type AnswerQueueState string

const (
	AnswerStarted AnswerQueueState = "started"
	AnswerQueued  AnswerQueueState = "queued"
)

type answerJob struct {
	id       uint64
	ctx      context.Context
	cancel   context.CancelFunc
	question Question
	input    string
	profile  config.AnswerModelConfig
	ready    chan struct{}
}

// AnswerCoordinator allows a small number of model requests to run together.
// Older requests are never preempted. When both lanes are occupied, questions
// wait in a LIFO queue so the newest interview question is promoted first while
// every older question remains available for later completion.
type AnswerCoordinator struct {
	mu       sync.Mutex
	idle     *sync.Cond
	executor AnswerExecutor
	profile  func() config.AnswerModelConfig
	onState  func(Question, AnswerQueueState)
	onChunk  func(Question, string)
	onDone   func(Question, string, error)

	active         map[uint64]*answerJob
	pending        []*answerJob
	nextID         uint64
	workers        int
	maxConcurrent  int
	priorityStreak int
	closed         bool
}

func NewAnswerCoordinator(
	executor AnswerExecutor,
	profile func() config.AnswerModelConfig,
	onState func(Question, AnswerQueueState),
	onChunk func(Question, string),
	onDone func(Question, string, error),
) *AnswerCoordinator {
	coordinator := &AnswerCoordinator{
		executor: executor, profile: profile, onState: onState, onChunk: onChunk, onDone: onDone,
		active: make(map[uint64]*answerJob), maxConcurrent: defaultAnswerConcurrency,
	}
	coordinator.idle = sync.NewCond(&coordinator.mu)
	return coordinator
}

func (c *AnswerCoordinator) Submit(parent context.Context, question Question, input string) AnswerQueueState {
	profile := config.AnswerModelConfig{}
	if c != nil && c.profile != nil {
		profile = c.profile()
	}
	return c.SubmitWithProfile(parent, question, input, profile)
}

func (c *AnswerCoordinator) SubmitWithProfile(parent context.Context, question Question, input string, profile config.AnswerModelConfig) AnswerQueueState {
	if c == nil || c.executor == nil || question.QuestionID == "" || input == "" {
		return ""
	}
	ctx, cancel := context.WithCancel(parent)
	job := &answerJob{ctx: ctx, cancel: cancel, question: question, input: input, profile: profile, ready: make(chan struct{})}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		cancel()
		return ""
	}
	c.nextID++
	job.id = c.nextID
	startNow := len(c.active) < c.maxConcurrent
	if startNow {
		c.active[job.id] = job
		c.workers++
	} else {
		c.pending = append(c.pending, job)
	}
	c.mu.Unlock()

	if startNow {
		c.publishState(question, AnswerStarted)
		close(job.ready)
		go c.run(job)
		return AnswerStarted
	}
	c.publishState(question, AnswerQueued)
	close(job.ready)
	return AnswerQueued
}

func (c *AnswerCoordinator) run(job *answerJob) {
	for job != nil {
		<-job.ready
		answer, err := c.executor.Stream(job.ctx, job.profile, job.input, func(chunk string) {
			if c.isActive(job) && job.ctx.Err() == nil && c.onChunk != nil {
				c.onChunk(job.question, chunk)
			}
		})

		c.mu.Lock()
		delete(c.active, job.id)
		next := c.popNewestPendingLocked()
		if next != nil {
			c.active[next.id] = next
		}
		c.mu.Unlock()

		c.publishDone(job.question, answer, err)
		job.cancel()
		job = next
		if job != nil {
			c.publishState(job.question, AnswerStarted)
			continue
		}

		c.mu.Lock()
		c.workers--
		if c.workers == 0 {
			c.idle.Broadcast()
		}
		c.mu.Unlock()
	}
}

func (c *AnswerCoordinator) popNewestPendingLocked() *answerJob {
	if len(c.pending) == 0 {
		return nil
	}
	if len(c.pending) == 1 {
		job := c.pending[0]
		c.pending = nil
		c.priorityStreak = 0
		return job
	}
	if c.priorityStreak >= maxNewestPriorityStreak {
		job := c.pending[0]
		copy(c.pending, c.pending[1:])
		c.pending[len(c.pending)-1] = nil
		c.pending = c.pending[:len(c.pending)-1]
		c.priorityStreak = 0
		return job
	}
	index := len(c.pending) - 1
	job := c.pending[index]
	c.pending[index] = nil
	c.pending = c.pending[:index]
	c.priorityStreak++
	return job
}

func (c *AnswerCoordinator) isActive(job *answerJob) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active[job.id] == job
}

func (c *AnswerCoordinator) publishState(question Question, state AnswerQueueState) {
	if c.onState != nil {
		c.onState(question, state)
	}
}

func (c *AnswerCoordinator) publishDone(question Question, answer string, err error) {
	if c.onDone != nil {
		c.onDone(question, answer, err)
	}
}

func (c *AnswerCoordinator) HasWork() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.active) > 0 || len(c.pending) > 0
}

func (c *AnswerCoordinator) Wait() {
	if c == nil {
		return
	}
	c.mu.Lock()
	for c.workers > 0 {
		c.idle.Wait()
	}
	c.mu.Unlock()
}

// Cancel stops the newest active answer. Other active and queued answers remain.
func (c *AnswerCoordinator) Cancel() {
	if c == nil {
		return
	}
	c.mu.Lock()
	var newest *answerJob
	for id, job := range c.active {
		if newest == nil || id > newest.id {
			newest = job
		}
	}
	c.mu.Unlock()
	if newest != nil {
		newest.cancel()
	}
}

// CancelAll is used when the interview session itself is stopping.
func (c *AnswerCoordinator) CancelAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	active := make([]*answerJob, 0, len(c.active))
	for _, job := range c.active {
		active = append(active, job)
	}
	pending := append([]*answerJob(nil), c.pending...)
	c.pending = nil
	c.priorityStreak = 0
	c.mu.Unlock()
	for _, job := range active {
		job.cancel()
	}
	for _, job := range pending {
		job.cancel()
		c.publishDone(job.question, "", context.Canceled)
	}
}

func (c *AnswerCoordinator) Close() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	c.CancelAll()
}
