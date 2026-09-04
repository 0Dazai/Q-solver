package knowledge

import (
	"strings"
	"testing"
)

func TestChunkMarkdownPreservesHeadingPathAndCodeBlocks(t *testing.T) {
	content := `# 候选人资料

## 项目经历

负责 Q-Solver 的实时音频链路和故障恢复。

### 技术细节

` + "```go\nfunc reconnect() {}\n```" + `

| 指标 | 结果 |
| --- | --- |
| 延迟 | 低 |
`

	chunks := ChunkMarkdown("resume.md", content)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].TitlePath != "候选人资料 / 项目经历" {
		t.Fatalf("unexpected title path: %q", chunks[0].TitlePath)
	}
	if !strings.Contains(chunks[1].Content, "func reconnect") {
		t.Fatalf("code block was not preserved: %+v", chunks[1])
	}
	if !strings.Contains(chunks[2].Content, "| 延迟 | 低 |") {
		t.Fatalf("table was not preserved: %+v", chunks[2])
	}
	for _, chunk := range chunks {
		if chunk.ContentHash == "" || chunk.DocumentID == "" {
			t.Fatalf("chunk identifiers must be populated: %+v", chunk)
		}
	}
}

func TestChunkMarkdownSplitsLongParagraph(t *testing.T) {
	content := "# 长文\n\n" + strings.Repeat("这是一个较长的项目说明。", 300)
	chunks := ChunkMarkdown("long.md", content)
	if len(chunks) < 2 {
		t.Fatalf("expected long paragraph to split, got %d", len(chunks))
	}
	for _, chunk := range chunks {
		if len([]rune(chunk.Content)) > MaxChunkRunes {
			t.Fatalf("chunk exceeds limit: %d", len([]rune(chunk.Content)))
		}
	}
}
