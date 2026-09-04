package interviewhistory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func RenderMarkdown(session Session, messages []Message) string {
	var builder strings.Builder
	builder.WriteString("# 面试记录\n\n")
	fmt.Fprintf(&builder, "- 开始时间：%s\n", session.StartedAt.Local().Format("2006-01-02 15:04:05"))
	if !session.EndedAt.IsZero() {
		fmt.Fprintf(&builder, "- 结束时间：%s\n", session.EndedAt.Local().Format("2006-01-02 15:04:05"))
	}
	if session.ResumePath != "" {
		fmt.Fprintf(&builder, "- 使用简历：%s\n", filepath.Base(session.ResumePath))
	}
	if session.Model != "" {
		fmt.Fprintf(&builder, "- 回答模型：%s\n", session.Model)
	}
	if session.AnswerMode != "" {
		fmt.Fprintf(&builder, "- 资料策略：%s\n", session.AnswerMode)
	}
	turnNumber := 0
	currentTurn := ""
	for _, message := range messages {
		if message.TurnID != currentTurn {
			currentTurn = message.TurnID
			turnNumber++
			fmt.Fprintf(&builder, "\n## 第 %d 轮\n\n", turnNumber)
		}
		switch message.Role {
		case RoleInterviewer:
			builder.WriteString("### 面试官问题\n\n")
		case RoleAISuggestion:
			builder.WriteString("### AI 参考答案\n\n")
		case RoleCandidate:
			builder.WriteString("### 我的实际回答\n\n")
		}
		builder.WriteString(strings.TrimSpace(message.Content))
		builder.WriteString("\n\n")
	}
	return builder.String()
}

func writeMarkdown(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
