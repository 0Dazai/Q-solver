package knowledge

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func ChunkMarkdown(path, content string) []Chunk {
	documentID := shortHash(filepath.Clean(path))
	headings := make([]string, 0, 6)
	blocks := parseMarkdownBlocks(content, &headings)
	chunks := make([]Chunk, 0, len(blocks))
	now := time.Now()
	for _, block := range blocks {
		for _, part := range splitRunes(block.content, MaxChunkRunes) {
			text := strings.TrimSpace(part)
			if text == "" {
				continue
			}
			ordinal := len(chunks)
			chunks = append(chunks, Chunk{
				ID:          fmt.Sprintf("%s-%04d", documentID, ordinal),
				DocumentID:  documentID,
				Path:        path,
				TitlePath:   strings.Join(block.headings, " / "),
				Content:     text,
				ContentHash: shortHash(text),
				Ordinal:     ordinal,
				UpdatedAt:   now,
			})
		}
	}
	return chunks
}

type markdownBlock struct {
	headings []string
	content  string
}

func parseMarkdownBlocks(content string, headings *[]string) []markdownBlock {
	scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(content, "\r\n", "\n")))
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	var blocks []markdownBlock
	var current []string
	inCode := false
	flush := func() {
		text := strings.TrimSpace(strings.Join(current, "\n"))
		if text != "" {
			blocks = append(blocks, markdownBlock{headings: append([]string(nil), (*headings)...), content: text})
		}
		current = nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if !inCode {
				flush()
				inCode = true
			}
			current = append(current, line)
			if strings.TrimSpace(line) == "```" && len(current) > 1 {
				inCode = false
				flush()
			}
			continue
		}
		if inCode {
			current = append(current, line)
			continue
		}
		if level, title, ok := markdownHeading(line); ok {
			flush()
			for len(*headings) >= level {
				*headings = (*headings)[:len(*headings)-1]
			}
			*headings = append(*headings, title)
			continue
		}
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		current = append(current, line)
	}
	flush()
	return blocks
}

func markdownHeading(line string) (int, string, bool) {
	trimmed := strings.TrimSpace(line)
	level := 0
	for level < len(trimmed) && level < 6 && trimmed[level] == '#' {
		level++
	}
	if level == 0 || len(trimmed) <= level || trimmed[level] != ' ' {
		return 0, "", false
	}
	title := strings.TrimSpace(trimmed[level:])
	return level, title, title != ""
}

func splitRunes(text string, limit int) []string {
	runes := []rune(text)
	if len(runes) <= limit {
		return []string{text}
	}
	parts := make([]string, 0, (len(runes)+limit-1)/limit)
	for len(runes) > 0 {
		end := minInt(limit, len(runes))
		if end < len(runes) {
			for i := end; i > limit/2; i-- {
				if strings.ContainsRune("。！？\n", runes[i-1]) {
					end = i
					break
				}
			}
		}
		parts = append(parts, string(runes[:end]))
		runes = runes[end:]
	}
	return parts
}

func shortHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", sum[:8])
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
