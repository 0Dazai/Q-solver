package interview

import (
	"strings"

	"Q-Solver/pkg/knowledge"
)

const maxRealtimeInputRunes = 4500

func buildPlannedAnswerInput(
	question string,
	localFinals []string,
	profile CandidateProfile,
	plan QuestionPlan,
	mode knowledge.AnswerMode,
	results []knowledge.SearchResult,
) (string, []knowledge.SearchResult) {
	question = strings.TrimSpace(question)
	parts := []string{
		"本轮回答要求：\n" + plan.Instruction,
		"当前面试官问题：\n" + question,
	}
	if previous := trimContextText(strings.TrimSpace(plan.PreviousQuestion), 320); previous != "" {
		parts = append(parts, "上一轮面试官问题（仅用于技术语境消歧）：\n"+previous)
	}

	if localContext := boundedLocalContext(localFinals, 800); localContext != "" {
		parts = append(parts, "求职者上一轮回答（仅用于理解追问语境；不要评价或复述）：\n"+localContext)
	}
	profileContext := selectProfileContext(profile, plan)
	if profileContext != "" {
		parts = append(parts, "与本题相关的候选人真实资料（仅引用明确相关内容，不要补造）：\n"+profileContext)
	}

	results = removeProfileDuplicates(results, profileContext)
	knowledgeInput, citations := buildKnowledgeAnswerInputBounded("", mode, results, plan.KnowledgeBudgetRunes, plan.KnowledgeLimit)
	knowledgeInput = strings.TrimSpace(knowledgeInput)
	if knowledgeInput != "" {
		parts = append(parts, knowledgeInput)
	}

	return trimContextText(strings.Join(parts, "\n\n"), maxRealtimeInputRunes), citations
}

func removeProfileDuplicates(results []knowledge.SearchResult, profileContext string) []knowledge.SearchResult {
	if len(results) == 0 || strings.TrimSpace(profileContext) == "" {
		return results
	}
	normalizedProfile := normalizeContextForDedup(profileContext)
	filtered := make([]knowledge.SearchResult, 0, len(results))
	for _, result := range results {
		content := normalizeContextForDedup(result.Content)
		if len([]rune(content)) >= 40 && strings.Contains(normalizedProfile, content) {
			continue
		}
		filtered = append(filtered, result)
	}
	return filtered
}

func normalizeContextForDedup(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), ""))
}

func boundedLocalContext(localFinals []string, maxRunes int) string {
	if len(localFinals) == 0 || maxRunes <= 0 {
		return ""
	}
	parts := make([]string, 0, len(localFinals))
	for _, final := range localFinals {
		if text := strings.TrimSpace(final); text != "" {
			parts = append(parts, text)
		}
	}
	return trimContextText(strings.Join(parts, "\n"), maxRunes)
}
