package interview

import "strings"

const maxLocalResponseSegments = 8

// answerInputWithLocalResponse keeps microphone transcripts out of the UI and
// gives them only to the next answer request as bounded conversational context.
func answerInputWithLocalResponse(question string, localFinals []string) string {
	question = strings.TrimSpace(question)
	if len(localFinals) == 0 {
		return question
	}
	parts := make([]string, 0, len(localFinals))
	for _, final := range localFinals {
		if text := strings.TrimSpace(final); text != "" {
			parts = append(parts, trimContextText(text, 1200))
		}
	}
	if len(parts) == 0 {
		return question
	}
	return "当前对方问题：\n" + question + "\n\n本机用户刚才的回答（仅作下一轮回答的上下文参考；不要复述、评价或单独回应这段内容，直接回答当前对方问题）：\n" + strings.Join(parts, "\n")
}

func trimContextText(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}
