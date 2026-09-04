package interview

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"Q-Solver/pkg/config"

	"github.com/gorilla/websocket"
)

// ASRClient is intentionally small so capture and session lifetime can be
// tested without a live WebSocket.
type ASRClient interface {
	SendAudio([]byte) error
	Events() <-chan TranscriptEvent
	Errors() <-chan error
	Close() error
}

// DashScopeASR implements the Fun-ASR WebSocket protocol documented at:
// https://help.aliyun.com/zh/model-studio/fun-asr-realtime-websocket-api
// The protocol is task based, not the OpenAI Realtime session.update protocol.
type DashScopeASR struct {
	conn      *websocket.Conn
	taskID    string
	events    chan TranscriptEvent
	errs      chan error
	mu        sync.Mutex
	closed    bool
	closing   bool
	started   bool
	done      chan struct{}
	readDone  chan struct{}
	closeDone chan struct{}
}

func ConnectDashScope(ctx context.Context, cfg config.TranscriptionConfig) (*DashScopeASR, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("千问 API Key 未配置")
	}
	endpoint, err := normalizeEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))
	header.Set("User-Agent", "Q-Solver/1.0")
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, header)
	if err != nil {
		return nil, fmt.Errorf("连接千问实时 ASR: %w", err)
	}

	c := &DashScopeASR{
		conn: conn, taskID: newTaskID(), events: make(chan TranscriptEvent, 32),
		errs: make(chan error, 4), done: make(chan struct{}), readDone: make(chan struct{}), closeDone: make(chan struct{}),
	}
	if err := c.runTask(cfg); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := c.waitStarted(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	go c.readLoop()
	return c, nil
}

func normalizeEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("千问服务地址未配置")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "wss" && u.Scheme != "ws") || u.Host == "" {
		return "", errors.New("千问服务地址必须是有效的 ws/wss URL")
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api-ws/v1/inference"
	}
	if !strings.HasSuffix(u.Path, "/api-ws/v1/inference") {
		return "", errors.New("Fun-ASR 服务地址必须使用 /api-ws/v1/inference")
	}
	return u.String(), nil
}

func (c *DashScopeASR) runTask(cfg config.TranscriptionConfig) error {
	return c.writeJSON(buildRunTaskRequest(c.taskID, cfg))
}

func buildRunTaskRequest(taskID string, cfg config.TranscriptionConfig) map[string]any {
	parameters := map[string]any{
		"format": "pcm", "sample_rate": SampleRate,
		"max_sentence_silence": cfg.SentenceWaitMS,
		"heartbeat":            true,
	}
	if language := strings.TrimSpace(cfg.Language); language != "" {
		parameters["language_hints"] = []string{language}
	}
	if vocabularyID := strings.TrimSpace(cfg.VocabularyID); vocabularyID != "" {
		parameters["vocabulary_id"] = vocabularyID
	}
	input := map[string]any{}
	// The default 2025-09-15 model does not support context. Newer official
	// realtime models do, so retain the user's terms and send them only where
	// the provider documents this parameter as valid.
	if supportsContext(cfg.Model) {
		terms := append(append([]string(nil), cfg.Hotwords...), cfg.ContextPhrases...)
		if text := strings.TrimSpace(strings.Join(terms, " ")); text != "" {
			input["context"] = []map[string]any{{
				"role": "user", "content": []map[string]string{{"type": "input_text", "text": text}},
			}}
		}
	}
	request := map[string]any{
		"header": map[string]any{
			"action": "run-task", "task_id": taskID, "streaming": "duplex",
		},
		"payload": map[string]any{
			"task_group": "audio", "task": "asr", "function": "recognition",
			"model": cfg.Model, "parameters": parameters, "input": input,
		},
	}
	return request
}

func supportsContext(model string) bool {
	model = strings.TrimSpace(model)
	return model == "fun-asr-realtime" || model == "fun-asr-realtime-2025-11-07"
}

func (c *DashScopeASR) waitStarted(ctx context.Context) error {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(12 * time.Second)
	}
	_ = c.conn.SetReadDeadline(deadline)
	defer c.conn.SetReadDeadline(time.Time{})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("等待千问 ASR 任务启动: %w", err)
		}
		kind, message := dashScopeEvent(data)
		switch kind {
		case "task-started":
			c.started = true
			return nil
		case "task-failed":
			return errors.New(message)
		}
	}
}

func (c *DashScopeASR) SendAudio(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || !c.started {
		return errors.New("ASR 连接尚未就绪")
	}
	// Fun-ASR accepts raw mono PCM as a binary WebSocket frame after task-started.
	return c.conn.WriteMessage(websocket.BinaryMessage, pcm)
}

func (c *DashScopeASR) Events() <-chan TranscriptEvent { return c.events }
func (c *DashScopeASR) Errors() <-chan error           { return c.errs }

func (c *DashScopeASR) writeJSON(value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("ASR 连接已关闭")
	}
	return c.conn.WriteJSON(value)
}

func (c *DashScopeASR) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	if c.closing {
		closeDone := c.closeDone
		c.mu.Unlock()
		<-closeDone
		return nil
	}
	c.closing = true
	started := c.started
	var finishErr error
	if c.started {
		finishErr = c.conn.WriteJSON(map[string]any{
			"header":  map[string]any{"action": "finish-task", "task_id": c.taskID, "streaming": "duplex"},
			"payload": map[string]any{"input": map[string]any{}},
		})
	}
	c.mu.Unlock()
	if finishErr == nil && started {
		select {
		case <-c.readDone:
		case <-time.After(8 * time.Second):
			finishErr = errors.New("等待千问 ASR task-finished 超时")
		}
	}
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	close(c.done)
	closeErr := c.conn.Close()
	close(c.closeDone)
	if finishErr != nil {
		return finishErr
	}
	return closeErr
}

func (c *DashScopeASR) readLoop() {
	defer close(c.events)
	defer close(c.errs)
	defer close(c.readDone)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			closing := c.closing
			c.mu.Unlock()
			if !closing {
				c.report(err)
			}
			return
		}
		kind, message := dashScopeEvent(data)
		if kind == "task-failed" {
			c.report(errors.New(message))
			return
		}
		if kind == "task-finished" {
			return
		}
		if event, ok := parseASREvent(data); ok {
			select {
			case c.events <- event:
			case <-c.done:
				return
			}
		}
	}
}

func (c *DashScopeASR) report(err error) {
	select {
	case c.errs <- err:
	default:
	}
}

func dashScopeEvent(data []byte) (string, string) {
	var raw struct {
		Header struct {
			Event        string `json:"event"`
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
		} `json:"header"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return "", ""
	}
	message := strings.TrimSpace(raw.Header.ErrorMessage)
	if message == "" {
		message = raw.Header.ErrorCode
	}
	if message == "" {
		message = "千问实时 ASR 任务失败"
	}
	return raw.Header.Event, message
}

func parseASREvent(data []byte) (TranscriptEvent, bool) {
	var raw struct {
		Header struct {
			Event string `json:"event"`
		} `json:"header"`
		Payload struct {
			Output struct {
				Sentence struct {
					Text        string `json:"text"`
					Heartbeat   bool   `json:"heartbeat"`
					SentenceEnd bool   `json:"sentence_end"`
					SentenceID  int64  `json:"sentence_id"`
				} `json:"sentence"`
			} `json:"output"`
		} `json:"payload"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Header.Event != "result-generated" {
		return TranscriptEvent{}, false
	}
	sentence := raw.Payload.Output.Sentence
	if sentence.Heartbeat || strings.TrimSpace(sentence.Text) == "" {
		return TranscriptEvent{}, false
	}
	kind := EventPartial
	if sentence.SentenceEnd {
		kind = EventFinal
	}
	return TranscriptEvent{Kind: kind, Text: sentence.Text, Sequence: sentence.SentenceID, Timestamp: time.Now()}, true
}

func newTaskID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(b[:4]), hex.EncodeToString(b[4:6]), hex.EncodeToString(b[6:8]), hex.EncodeToString(b[8:10]), hex.EncodeToString(b[10:]))
}
