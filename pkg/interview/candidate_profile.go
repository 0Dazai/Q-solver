package interview

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

type ProfileSection struct {
	Source  string
	Heading string
	Content string
}

type CandidateProfile struct {
	Context    string
	Sections   []ProfileSection
	SourceHash string
	PreparedAt time.Time
}

func buildCandidateProfile(resumeContent string, pinnedMarkdown []string, maxRunes int) CandidateProfile {
	parts := make([]string, 0, len(pinnedMarkdown)+1)
	sections := make([]ProfileSection, 0, 16)
	if text := strings.TrimSpace(resumeContent); text != "" {
		parts = append(parts, "## 候选人简历\n"+text)
		sections = append(sections, splitProfileSections("简历", text)...)
	}
	for _, document := range pinnedMarkdown {
		if text := strings.TrimSpace(document); text != "" {
			parts = append(parts, "## 面试常驻资料\n"+text)
			sections = append(sections, splitProfileSections("Markdown资料", text)...)
		}
	}
	context := strings.Join(parts, "\n\n")
	if maxRunes > 0 {
		context = trimContextText(context, maxRunes)
	}
	sum := sha256.Sum256([]byte(context))
	return CandidateProfile{Context: context, Sections: sections, SourceHash: hex.EncodeToString(sum[:8]), PreparedAt: time.Now()}
}

func answerInputWithCandidateProfile(question string, profile CandidateProfile, keywords []string) string {
	if strings.TrimSpace(profile.Context) == "" {
		return question
	}
	context := profile.Context
	if len(keywords) > 0 {
		context = filterProfileByKeywords(context, keywords)
	}
	return "候选人预载资料（以下是候选人的真实简历和常驻资料，回答时请主动引用其中的相关项目、技术栈或经历作为论据，不要复述资料标题）：\n" +
		context + "\n\n当前面试官问题：\n" + question
}

func selectProfileContext(profile CandidateProfile, plan QuestionPlan) string {
	if plan.ProfileBudgetRunes <= 0 || len(profile.Sections) == 0 {
		return ""
	}
	type scoredSection struct {
		section ProfileSection
		score   int
		ordinal int
	}
	scored := make([]scoredSection, 0, len(profile.Sections))
	for index, section := range profile.Sections {
		if plan.Type == QuestionTypeSelfIntro && section.Source != "简历" {
			continue
		}
		text := strings.ToLower(section.Heading + "\n" + section.Content)
		score := 0
		for _, term := range plan.QueryTerms {
			term = strings.ToLower(strings.TrimSpace(term))
			if term == "" {
				continue
			}
			if strings.Contains(strings.ToLower(section.Heading), term) {
				score += 5
			}
			if strings.Contains(text, term) {
				score += 2
			}
		}
		if section.Source == "简历" {
			score++
		}
		if plan.Type == QuestionTypeSelfIntro && section.Source == "简历" {
			score += maxInt(6-index, 1)
		}
		scored = append(scored, scoredSection{section: section, score: score, ordinal: index})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].ordinal < scored[j].ordinal
	})

	var selected strings.Builder
	used := 0
	for _, item := range scored {
		if item.score <= 1 && plan.Type != QuestionTypeSelfIntro && plan.Type != QuestionTypeBehavioral {
			continue
		}
		content := strings.TrimSpace(item.section.Content)
		if content == "" {
			continue
		}
		block := "### " + item.section.Source
		if item.section.Heading != "" {
			block += " / " + item.section.Heading
		}
		block += "\n" + content + "\n\n"
		remaining := plan.ProfileBudgetRunes - used
		if remaining <= 0 {
			break
		}
		block = trimContextText(block, remaining)
		selected.WriteString(block)
		used += len([]rune(block))
	}
	return strings.TrimSpace(selected.String())
}

func splitProfileSections(source, text string) []ProfileSection {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	sections := make([]ProfileSection, 0, 12)
	heading := ""
	var current strings.Builder
	flush := func() {
		content := strings.TrimSpace(current.String())
		if content != "" {
			sections = append(sections, ProfileSection{Source: source, Heading: heading, Content: trimContextText(content, 1200)})
		}
		current.Reset()
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			flush()
			heading = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			continue
		}
		if trimmed == "" {
			flush()
			continue
		}
		current.WriteString(line)
		current.WriteByte('\n')
	}
	flush()
	return sections
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func filterProfileByKeywords(context string, keywords []string) string {
	paragraphs := strings.Split(context, "\n\n")
	if len(paragraphs) <= 2 {
		return context
	}
	keywordSet := make(map[string]bool)
	for _, kw := range keywords {
		if kw = strings.ToLower(strings.TrimSpace(kw)); len(kw) >= 2 {
			keywordSet[kw] = true
		}
	}
	if len(keywordSet) == 0 {
		return context
	}
	filtered := []string{paragraphs[0], paragraphs[1]}
	matchedAny := false
	for i := 2; i < len(paragraphs); i++ {
		p := strings.ToLower(paragraphs[i])
		for kw := range keywordSet {
			if strings.Contains(p, kw) {
				filtered = append(filtered, paragraphs[i])
				matchedAny = true
				break
			}
		}
	}
	if !matchedAny {
		return context
	}
	return strings.Join(filtered, "\n\n")
}

func extractQuestionKeywords(question string) []string {
	fields := strings.FieldsFunc(question, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '，' ||
			r == '.' || r == '。' || r == '?' || r == '？' || r == '!' || r == '！' ||
			r == ';' || r == '；' || r == ':' || r == '：' || r == '(' || r == ')' ||
			r == '（' || r == '）' || r == '"' || r == '\'' ||
			r == '-' || r == '—' || r == '/' || r == '\\'
	})
	var keywords []string
	seen := make(map[string]bool)
	for _, f := range fields {
		f = strings.ToLower(strings.TrimSpace(f))
		if len(f) < 2 || seen[f] {
			continue
		}
		seen[f] = true
		keywords = append(keywords, f)
	}
	return keywords
}
