package resume

import (
	"context"
	"strings"
	"testing"

	"Q-Solver/pkg/config"
)

func TestParseResumeUsesLocalPDFExtractor(t *testing.T) {
	var extractedPath string
	service := &Service{
		config: config.Config{ResumePath: `D:\fixtures\候选人简历.pdf`},
		extractPDF: func(path string) (string, error) {
			extractedPath = path
			return "姓名：测试候选人\n技能：Go、Vue", nil
		},
	}

	markdown, err := service.ParseResume(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if extractedPath != service.config.ResumePath {
		t.Fatalf("local extractor received %q, want %q", extractedPath, service.config.ResumePath)
	}
	if !strings.Contains(markdown, "# 候选人简历") ||
		!strings.Contains(markdown, "技能：Go、Vue") {
		t.Fatalf("unexpected parsed Markdown: %q", markdown)
	}
}
