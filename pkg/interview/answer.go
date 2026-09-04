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

type HTTPAnswerExecutor struct {
	Client   *http.Client
	OnTiming func(AnswerTiming)
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(body))
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
	start := time.Now()
	resp, err := client.Do(req)
	connectMS := time.Since(start).Milliseconds()
	if err != nil {
		return "", err
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
		}
	}
	if err := scanner.Err(); err != nil {
		return answer.String(), err
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
