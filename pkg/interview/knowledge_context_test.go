package interview

import (
	"strings"
	"testing"

	"Q-Solver/pkg/knowledge"
)

func TestBuildKnowledgeAnswerInputSupportsKnowledgeFirstAndOnly(t *testing.T) {
	results := []knowledge.SearchResult{{
		Chunk:  knowledge.Chunk{Path: "resume.md", TitlePath: "项目 / Q-Solver", Content: "负责实时音频链路和 ASR 重连。"},
		Source: "local_fts",
	}}
	first, citations := buildKnowledgeAnswerInput("如何处理断线？", knowledge.AnswerModeKnowledgeFirst, results)
	if !strings.Contains(first, "优先依据") || !strings.Contains(first, "ASR 重连") {
		t.Fatalf("knowledge-first context missing: %q", first)
	}
	if len(citations) != 1 || citations[0].Path != "resume.md" {
		t.Fatalf("citations missing: %+v", citations)
	}
	only, _ := buildKnowledgeAnswerInput("没有资料的问题", knowledge.AnswerModeKnowledgeOnly, nil)
	if !strings.Contains(only, "资料中未找到") {
		t.Fatalf("knowledge-only empty result must constrain the answer: %q", only)
	}
	general, citations := buildKnowledgeAnswerInput("通用问题", knowledge.AnswerModeGeneral, results)
	if general != "通用问题" || len(citations) != 0 {
		t.Fatalf("general mode must not inject knowledge: %q %+v", general, citations)
	}
}
