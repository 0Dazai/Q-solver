package interview

import (
	"strings"
	"testing"
)

func TestInterviewPromptFixesCandidateRoleAndKeepsUserPrompt(t *testing.T) {
	got := buildInterviewSystemPrompt("回答时突出 Go 经验")
	for _, required := range []string{"候选人", "第一人称", "JD", "真实参与程度", "理论准备", "Ownership", "不凭服务名", "现场分析", "Redisson Watchdog", "只回答当前问题", "不邀请继续展开", "突出 Go 经验"} {
		if !strings.Contains(got, required) {
			t.Fatalf("missing %q", required)
		}
	}
}

func TestInterviewPromptStaysCompact(t *testing.T) {
	if got := len([]rune(interviewSystemPrompt)); got > 850 {
		t.Fatalf("interview prompt grew beyond realtime budget: %d runes", got)
	}
}

func TestCandidateProfilePreloadsResume(t *testing.T) {
	profile := buildCandidateProfile("# 简历\n负责 Go 实时音频和 ASR 重连", nil, 12000)
	if !strings.Contains(profile.Context, "Go 实时音频") || profile.SourceHash == "" {
		t.Fatalf("profile=%+v", profile)
	}
}
