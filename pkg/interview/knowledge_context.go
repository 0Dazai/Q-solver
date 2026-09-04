package interview

import (
	"fmt"
	"strings"

	"Q-Solver/pkg/knowledge"
)

const maxKnowledgeContextRunes = 1600

func buildKnowledgeAnswerInput(
	question string,
	mode knowledge.AnswerMode,
	results []knowledge.SearchResult,
) (string, []knowledge.SearchResult) {
	return buildKnowledgeAnswerInputBounded(question, mode, results, maxKnowledgeContextRunes, 3)
}

func buildKnowledgeAnswerInputBounded(
	question string,
	mode knowledge.AnswerMode,
	results []knowledge.SearchResult,
	maxRunes int,
	maxResults int,
) (string, []knowledge.SearchResult) {
	question = strings.TrimSpace(question)
	if mode == knowledge.AnswerModeGeneral {
		return question, nil
	}
	if len(results) == 0 {
		if mode == knowledge.AnswerModeKnowledgeOnly {
			return question + "\n\n本次选择“仅限资料”模式，但本地资料中未找到相关依据。请只回答：资料中未找到足够依据。", nil
		}
		return question, nil
	}
	if maxRunes <= 0 {
		maxRunes = maxKnowledgeContextRunes
	}
	if maxResults <= 0 {
		maxResults = 3
	}
	var context strings.Builder
	usedResults := make([]knowledge.SearchResult, 0, maxResults)
	seen := make(map[string]bool)
	for _, result := range results {
		if len(usedResults) >= maxResults {
			break
		}
		key := result.ContentHash
		if key == "" {
			key = result.Path + "#" + fmt.Sprint(result.Ordinal)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		remaining := maxRunes - len([]rune(context.String()))
		if remaining <= 0 {
			break
		}
		content := trimContextText(strings.TrimSpace(result.Content), minInt(520, remaining))
		block := fmt.Sprintf("\n[%d] 文件：%s\n标题：%s\n内容：%s\n",
			len(usedResults)+1, result.Path, result.TitlePath, content)
		block = trimContextText(block, remaining)
		context.WriteString(block)
		usedResults = append(usedResults, result)
	}
	instruction := "优先依据以下本地资料回答；资料未覆盖的部分可以使用通用知识补充，并明确区分。"
	if mode == knowledge.AnswerModeKnowledgeOnly {
		instruction = "仅依据以下本地资料回答。资料没有直接依据的内容请说明“资料中未找到足够依据”，不要补充外部事实。"
	}
	return question + "\n\n" + instruction + "\n" + strings.TrimSpace(context.String()), usedResults
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
