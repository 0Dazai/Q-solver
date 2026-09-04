package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"Q-Solver/pkg/config"
)

func TestAuthorizedProxyDiagnostic(t *testing.T) {
	if testingEnv("Q_SOLVER_PROXY_DIAGNOSTIC") != "1" {
		t.Skip("set Q_SOLVER_PROXY_DIAGNOSTIC=1 to run the authorized proxy diagnostic")
	}
	cm := config.NewConfigManager()
	if err := cm.Load(); err != nil {
		t.Fatal(err)
	}
	p := cm.Get().InterviewModel
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"model": p.Model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 16, "stream": true,
	})
	host := "https://proxy.ai-member.icu"
	baseURLs := []string{host, host + "/v1", host + "/api/v1", host + "/openai/v1"}
	for _, baseURL := range baseURLs {
		url := baseURL + "/chat/completions"
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+p.APIKey)
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Logf("POST %s failed: %v", url, err)
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		_ = response.Body.Close()
		message := regexp.MustCompile(`sk-[A-Za-z0-9._-]+`).ReplaceAllString(string(data), "[redacted]")
		var decoded struct {
			Choices []json.RawMessage `json:"choices"`
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 && strings.Contains(message, `"delta":{"content"`) {
			t.Logf("working streaming API base: %s", baseURL)
			return
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 && json.Unmarshal([]byte(message), &decoded) == nil && len(decoded.Choices) > 0 {
			t.Logf("working API base: %s", baseURL)
			return
		}
		t.Logf("POST %s -> HTTP %d: %s", url, response.StatusCode, strings.TrimSpace(message))
	}
	t.Fatal("no tested OpenAI-compatible API base accepted the request")
}
