package interview

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"Q-Solver/pkg/config"
)

type AnswerExecutor interface {
	Stream(context.Context, config.AnswerModelConfig, string, func(string)) (string, error)
}

// AnswerTiming captures per-phase latency for a single answer request.
// All durations are measured from the moment the HTTP request is dispatched.
type AnswerTiming struct {
	ConnectMS    int64 // request dispatched -> response headers received (TCP+TLS+TTFB-header)
	FirstByteMS  int64 // request dispatched -> first SSE "data:" line
	FirstTokenMS int64 // request dispatched -> first non-empty answer chunk
	TotalMS      int64 // request dispatched -> stream finished
	ChunkCount   int
	TotalBytes   int
}

// Centralized realtime-answer timeouts. A hung model request must never hold a
// concurrency lane forever: with two lanes, two dead SSE streams would stop
// every later interview question until the session is restarted.
const (
	// defaultFirstTokenTimeout bounds connect + TLS + model queueing + prompt
	// processing, i.e. everything before the first usable answer chunk.
	defaultFirstTokenTimeout = 8 * time.Second
	// defaultIdleTimeout bounds a stall in the middle of an SSE stream.
	defaultIdleTimeout = 15 * time.Second
	// defaultTotalTimeout bounds the whole spoken-style answer request.
	defaultTotalTimeout = 30 * time.Second
	// deepAnswerTotalTimeout applies to deepen requests, which enable reasoning
	// and allow 450-750 汉字 output.
	deepAnswerTotalTimeout = 90 * time.Second
)

type HTTPAnswerExecutor struct {
	Client   *http.Client
	OnTiming func(AnswerTiming)
	// Overrides for tests and future config wiring; zero uses the defaults.
	FirstTokenTimeout time.Duration
	IdleTimeout       time.Duration
	TotalTimeout      time.Duration
}

func (e *HTTPAnswerExecutor) firstTokenTimeout() time.Duration {
	if e != nil && e.FirstTokenTimeout > 0 {
		return e.FirstTokenTimeout
	}
	return defaultFirstTokenTimeout
}

func (e *HTTPAnswerExecutor) idleTimeout() time.Duration {
	if e != nil && e.IdleTimeout > 0 {
		return e.IdleTimeout
	}
	return defaultIdleTimeout
}

func (e *HTTPAnswerExecutor) totalTimeout(profile config.AnswerModelConfig) time.Duration {
	if e != nil && e.TotalTimeout > 0 {
		return e.TotalTimeout
	}
	if strings.EqualFold(strings.TrimSpace(profile.ThinkingMode), "enabled") {
		return deepAnswerTotalTimeout
	}
	return defaultTotalTimeout
}

func (e *HTTPAnswerExecutor) Stream(ctx context.Context, profile config.AnswerModelConfig, question string, onChunk func(string)) (string, error) {
	if profile.APIKey == "" || profile.Model == "" {
		return "", errors.New("面试回答模型尚未完整配置")
	}
	if profile.Protocol == "" {
		profile.Protocol = "openai_chat_completions"
	}
	base := strings.TrimRight(profile.BaseURL, "/")
	if base == "" {
		return "", errors.New("面试回答模型 Base URL 未配置")
	}
	path, payload, err := buildAnswerPayload(profile, question)
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(payload)
	totalTimeout := e.totalTimeout(profile)
	reqCtx, cancelTotal := context.WithTimeout(ctx, totalTimeout)
	defer cancelTotal()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, base+path, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+profile.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	client := e.Client
	if client == nil {
		client = http.DefaultClient
	}
	// The watchdog cancels the request when no SSE data arrives in time. It
	// enforces the first-token deadline until the first chunk arrives and the
	// idle deadline afterwards, so both a slow model queue and a mid-stream
	// hang free the concurrency lane instead of blocking later questions.
	start := time.Now()
	timeoutHit := make(chan string, 1)
	watchdogDone := make(chan struct{})
	firstChunkDone := make(chan struct{})
	var firstChunkOnce sync.Once
	var lastActivity atomic.Int64
	lastActivity.Store(start.UnixNano())
	go func() {
		defer close(watchdogDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-reqCtx.Done():
				return
			case now := <-ticker.C:
				select {
				case <-firstChunkDone:
					if now.Sub(time.Unix(0, lastActivity.Load())) > e.idleTimeout() {
						select {
						case timeoutHit <- "idle":
						default:
						}
						cancelTotal()
						return
					}
				default:
					if now.Sub(start) > e.firstTokenTimeout() {
						select {
						case timeoutHit <- "first-token":
						default:
						}
						cancelTotal()
						return
					}
				}
			}
		}
	}()
	defer func() {
		cancelTotal()
		<-watchdogDone
	}()
	resp, err := client.Do(req)
	connectMS := time.Since(start).Milliseconds()
	if err != nil {
		return "", streamError(err, timeoutHit, e.firstTokenTimeout(), totalTimeout)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("回答模型请求失败 (%d): %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var answer strings.Builder
	var nonStreaming strings.Builder
	sawSSE := false
	firstByteMS := int64(0)
	firstTokenMS := int64(0)
	chunkCount := 0
	totalBytes := 0
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			nonStreaming.WriteString(line)
			continue
		}
		if !sawSSE {
			firstByteMS = time.Since(start).Milliseconds()
		}
		sawSSE = true
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		chunk := extractAnswerChunk(profile.Protocol, data)
		if chunk != "" {
			if firstTokenMS == 0 {
				firstTokenMS = time.Since(start).Milliseconds()
			}
			answer.WriteString(chunk)
			chunkCount++
			totalBytes += len(chunk)
			if onChunk != nil {
				onChunk(chunk)
			}
			lastActivity.Store(time.Now().UnixNano())
			firstChunkOnce.Do(func() { close(firstChunkDone) })
		}
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), streamError(err, timeoutHit, e.firstTokenTimeout(), totalTimeout)
	}
	totalMS := time.Since(start).Milliseconds()
	if e.OnTiming != nil {
		e.OnTiming(AnswerTiming{
			ConnectMS:    connectMS,
			FirstByteMS:  firstByteMS,
			FirstTokenMS: firstTokenMS,
			TotalMS:      totalMS,
			ChunkCount:   chunkCount,
			TotalBytes:   totalBytes,
		})
	}
	if !sawSSE {
		fallback := extractNonStreamingAnswer(profile.Protocol, []byte(nonStreaming.String()))
		if fallback == "" {
			return "", errors.New("回答模型返回了无法识别的非流式响应")
		}
		if onChunk != nil {
			onChunk(fallback)
		}
		return fallback, nil
	}
	return answer.String(), nil
}

// streamError keeps user cancellation recognizable as context.Canceled while
// turning watchdog firings into explicit, sanitized timeout descriptions.
func streamError(cause error, timeoutHit chan string, firstToken, total time.Duration) error {
	select {
	case reason := <-timeoutHit:
		if reason == "first-token" {
			return fmt.Errorf("首 Token 超时（%s），已释放回答通道", firstToken)
		}
		return errors.New("回答流空闲超时，已释放回答通道")
	default:
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return fmt.Errorf("总回答超时（%s），已释放回答通道", total)
	}
	return cause
}

func buildAnswerPayload(profile config.AnswerModelConfig, question string) (string, map[string]any, error) {
	var path string
	var payload map[string]any
	if profile.Protocol == "openai_responses" {
		path = "/responses"
		input := []map[string]any{{"role": "user", "content": question}}
		payload = map[string]any{"model": profile.Model, "input": input, "stream": true, "max_output_tokens": profile.MaxTokens, "temperature": profile.Temperature}
		if profile.SystemPrompt != "" {
			payload["instructions"] = profile.SystemPrompt
		}
		if profile.DisableResponseStorage {
			payload["store"] = false
		}
	} else if profile.Protocol == "openai_chat_completions" {
		path = "/chat/completions"
		messages := []map[string]string{}
		if profile.SystemPrompt != "" {
			messages = append(messages, map[string]string{"role": "system", "content": profile.SystemPrompt})
		}
		messages = append(messages, map[string]string{"role": "user", "content": question})
		payload = map[string]any{"model": profile.Model, "messages": messages, "stream": true, "max_tokens": profile.MaxTokens, "temperature": profile.Temperature}
	} else {
		return "", nil, fmt.Errorf("不支持的面试回答协议: %s", profile.Protocol)
	}
	applyThinkingPayload(payload, profile)
	if profile.MaxTokens <= 0 {
		delete(payload, "max_tokens")
		delete(payload, "max_output_tokens")
	}
	return path, payload, nil
}

func applyThinkingPayload(payload map[string]any, profile config.AnswerModelConfig) {
	mode := strings.ToLower(strings.TrimSpace(profile.ThinkingMode))
	if mode == "" || mode == "auto" {
		return
	}
	provider := answerProviderCode(profile)
	level := strings.ToLower(strings.TrimSpace(profile.ReasoningLevel))
	if level == "" {
		level = "medium"
	}
	if profile.Protocol == "openai_responses" {
		if provider == "deepseek" || provider == "doubao" || provider == "volcengine" || provider == "ark" {
			payload["thinking"] = map[string]any{"type": map[bool]string{true: "enabled", false: "disabled"}[mode == "enabled"]}
			if mode == "enabled" {
				payload["reasoning_effort"] = level
			}
			return
		}
		if mode == "disabled" {
			level = "none"
		}
		payload["reasoning"] = map[string]any{"effort": level}
		return
	}
	switch provider {
	case "alibaba", "qwen":
		payload["enable_thinking"] = mode == "enabled"
	case "deepseek", "doubao", "volcengine", "ark":
		payload["thinking"] = map[string]any{"type": map[bool]string{true: "enabled", false: "disabled"}[mode == "enabled"]}
		if mode == "enabled" {
			payload["reasoning_effort"] = level
		}
	default:
		if mode == "disabled" {
			level = "none"
		}
		payload["reasoning_effort"] = level
	}
}

func answerProviderCode(profile config.AnswerModelConfig) string {
	provider := strings.ToLower(strings.TrimSpace(profile.Provider))
	baseURL := strings.ToLower(strings.TrimSpace(profile.BaseURL))
	model := strings.ToLower(strings.TrimSpace(profile.Model))
	if provider != "" && provider != "custom" && !(provider == "openai" && !strings.Contains(baseURL, "openai.com")) {
		return provider
	}
	switch {
	case strings.Contains(baseURL, "deepseek.com"), strings.HasPrefix(model, "deepseek-"):
		return "deepseek"
	case strings.Contains(baseURL, "volces.com"), strings.Contains(model, "doubao"), strings.HasPrefix(model, "ep-"):
		return "doubao"
	case strings.Contains(baseURL, "dashscope"), strings.Contains(baseURL, "aliyuncs.com"), strings.HasPrefix(model, "qwen"):
		return "alibaba"
	default:
		return provider
	}
}
func extractAnswerChunk(protocol, data string) string {
	var raw map[string]any
	if json.Unmarshal([]byte(data), &raw) != nil {
		return ""
	}
	if protocol == "openai_chat_completions" {
		if choices, ok := raw["choices"].([]any); ok && len(choices) > 0 {
			if choice, ok := choices[0].(map[string]any); ok {
				if delta, ok := choice["delta"].(map[string]any); ok {
					text, _ := delta["content"].(string)
					return text
				}
				if message, ok := choice["message"].(map[string]any); ok {
					text, _ := message["content"].(string)
					return text
				}
			}
		}
		return ""
	}
	// The Responses stream sends incremental text in
	// response.output_text.delta and then repeats the complete text in
	// response.output_text.done. Only delta events belong in the streamed
	// answer; appending the done snapshot duplicates the whole response.
	if eventType, _ := raw["type"].(string); eventType != "" {
		if eventType != "response.output_text.delta" {
			return ""
		}
		delta, _ := raw["delta"].(string)
		return delta
	}
	// Keep compatibility with relays that omit the official event type and
	// forward only a delta or text field.
	if delta, _ := raw["delta"].(string); delta != "" {
		return delta
	}
	if output, _ := raw["text"].(string); output != "" {
		return output
	}
	return ""
}

func extractNonStreamingAnswer(protocol string, data []byte) string {
	if protocol == "openai_chat_completions" {
		return extractAnswerChunk(protocol, string(data))
	}
	var raw map[string]any
	if json.Unmarshal(data, &raw) != nil {
		return ""
	}
	if text, _ := raw["output_text"].(string); text != "" {
		return text
	}
	return extractAnswerChunk(protocol, string(data))
}
