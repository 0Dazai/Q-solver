package interviewhistory

import (
	"strings"
	"testing"
	"time"
)

func TestRenderMarkdownIncludesThreeRoles(t *testing.T) {
	session := Session{ID: "s1", StartedAt: time.Unix(10, 0)}
	messages := []Message{
		{TurnID: "t1", Role: RoleInterviewer, Content: "请介绍项目"},
		{TurnID: "t1", Role: RoleAISuggestion, Content: "我主要负责音频链路"},
		{TurnID: "t1", Role: RoleCandidate, Content: "我实际负责 ASR"},
	}
	got := RenderMarkdown(session, messages)
	for _, text := range []string{"### 面试官问题", "### AI 参考答案", "### 我的实际回答", "我实际负责 ASR"} {
		if !strings.Contains(got, text) {
			t.Fatalf("missing %q in %s", text, got)
		}
	}
}
