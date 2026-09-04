package interview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"Q-Solver/pkg/common"
	"Q-Solver/pkg/knowledge"
)

const (
	maxRetrievalLogSize    = 5 * 1024 * 1024
	maxRetrievalLogBackups = 5
	maxLogQueryRunes       = 500
	maxLogContentRunes     = 200
	maxLogResults          = 10
)

// RetrievalLogger writes retrieval diagnostics to a rotating log file.
// It is independent from the build-tagged logger so that prod builds can
// still capture retrieval diagnostics during testing.
//
// Sensitive content (full resume, API keys, complete system prompt) is
// never written; only the query, result metadata, scores, and truncated
// content snippets are logged.
type RetrievalLogger struct {
	mu      sync.Mutex
	file    *os.File
	path    string
	enabled bool
}

func NewRetrievalLogger() *RetrievalLogger {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return &RetrievalLogger{enabled: false}
	}
	logDir := filepath.Join(configRoot, common.AppName, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return &RetrievalLogger{enabled: false}
	}
	return &RetrievalLogger{
		path:    filepath.Join(logDir, "retrieval.log"),
		enabled: true,
	}
}

// RetrievalLogEntry captures everything needed to diagnose a retrieval call.
type RetrievalLogEntry struct {
	Query            string
	ExpandedQuery    string
	QuestionType     QuestionType
	Decision         string
	InputRunes       int
	Results          []knowledge.SearchResult
	InjectedCount    int
	Err              error
	DurationMS       int64
	NormalizedScores []float64
	CacheHit         bool
}

func (l *RetrievalLogger) Log(entry RetrievalLogEntry) {
	if l == nil || !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] 检索调试 (耗时=%dms)\n",
		time.Now().Format("2006-01-02 15:04:05.000"), entry.DurationMS))
	sb.WriteString(fmt.Sprintf("  原查询: %s\n", truncateForLog(entry.Query, maxLogQueryRunes)))
	if entry.QuestionType != "" {
		sb.WriteString(fmt.Sprintf("  题型: %s, 决策: %s, 最终输入: %d字\n", entry.QuestionType, entry.Decision, entry.InputRunes))
	}
	if entry.ExpandedQuery != "" && entry.ExpandedQuery != entry.Query {
		sb.WriteString(fmt.Sprintf("  扩展查询: %s\n", truncateForLog(entry.ExpandedQuery, maxLogQueryRunes)))
	}
	if entry.Err != nil {
		sb.WriteString(fmt.Sprintf("  检索错误: %v\n", entry.Err))
	} else {
		cacheLabel := ""
		if entry.CacheHit {
			cacheLabel = " [缓存命中]"
		}
		sb.WriteString(fmt.Sprintf("  召回数量: %d, 注入数量: %d%s\n", len(entry.Results), entry.InjectedCount, cacheLabel))
		if len(entry.NormalizedScores) > 0 && entry.NormalizedScores[0] < 0.15 {
			sb.WriteString(fmt.Sprintf("  ⚠ 低分命中观察: top1归一化得分=%.4f（仅记录，未阻止注入，收集数据后定阈值）\n", entry.NormalizedScores[0]))
		}
		for i, r := range entry.Results {
			if i >= maxLogResults {
				sb.WriteString(fmt.Sprintf("  ... 还有 %d 条未记录\n", len(entry.Results)-maxLogResults))
				break
			}
			fileName := filepath.Base(r.Path)
			score := r.Score
			if i < len(entry.NormalizedScores) {
				sb.WriteString(fmt.Sprintf("  [%d] 原始分=%.4f 归一化=%.4f 来源=%s 文件=%s 标题=%s\n",
					i+1, score, entry.NormalizedScores[i], r.Source, fileName,
					truncateForLog(r.TitlePath, 100)))
			} else {
				sb.WriteString(fmt.Sprintf("  [%d] 得分=%.4f 来源=%s 文件=%s 标题=%s\n",
					i+1, score, r.Source, fileName,
					truncateForLog(r.TitlePath, 100)))
			}
			sb.WriteString(fmt.Sprintf("      内容: %s\n", truncateForLog(r.Content, maxLogContentRunes)))
		}
	}
	sb.WriteString("\n")

	l.file.WriteString(sb.String())
	l.rotateIfNeeded()
}

// LogPipeline records the small set of transitions needed to distinguish an
// ASR finalization problem from aggregation, retrieval, queueing, or API work.
func (l *RetrievalLogger) LogPipeline(stage, text, detail string) {
	if l == nil || !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ensureOpen(); err != nil {
		return
	}
	line := fmt.Sprintf("[%s] 实时链路 阶段=%s 文本=%s",
		time.Now().Format("2006-01-02 15:04:05.000"),
		truncateForLog(stage, 40), truncateForLog(text, maxLogQueryRunes))
	if strings.TrimSpace(detail) != "" {
		line += " 详情=" + truncateForLog(detail, 240)
	}
	_, _ = l.file.WriteString(line + "\n")
	l.rotateIfNeeded()
}

func (l *RetrievalLogger) ensureOpen() error {
	if l.file != nil {
		return nil
	}
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	l.file = f
	return nil
}

func (l *RetrievalLogger) rotateIfNeeded() {
	info, err := l.file.Stat()
	if err != nil || info.Size() < maxRetrievalLogSize {
		return
	}
	l.file.Close()
	l.file = nil
	for i := maxRetrievalLogBackups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s.%d", l.path, i)
		newer := fmt.Sprintf("%s.%d", l.path, i+1)
		os.Rename(old, newer)
	}
	os.Rename(l.path, l.path+".1")
}

func (l *RetrievalLogger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// LogAnswerTiming records per-phase latency for an answer request.
// Used to verify connection reuse and diagnose TTFT (time-to-first-token).
func (l *RetrievalLogger) LogAnswerTiming(timing AnswerTiming) {
	if l == nil || !l.enabled {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.ensureOpen(); err != nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] 回答耗时\n", time.Now().Format("2006-01-02 15:04:05.000")))
	sb.WriteString(fmt.Sprintf("  连接建立: %dms (TCP+TLS+响应头)\n", timing.ConnectMS))
	sb.WriteString(fmt.Sprintf("  首字节:   %dms (第一个 SSE data 行)\n", timing.FirstByteMS))
	sb.WriteString(fmt.Sprintf("  首Token:  %dms (第一个非空回答片段)\n", timing.FirstTokenMS))
	sb.WriteString(fmt.Sprintf("  总耗时:   %dms, 片段数=%d, 总字节=%d\n", timing.TotalMS, timing.ChunkCount, timing.TotalBytes))
	if timing.FirstTokenMS > 0 && timing.FirstByteMS > 0 {
		sb.WriteString(fmt.Sprintf("  模型推理: %dms (首Token - 首字节)\n", timing.FirstTokenMS-timing.FirstByteMS))
	}
	sb.WriteString("\n")

	l.file.WriteString(sb.String())
	l.rotateIfNeeded()
}

func truncateForLog(text string, maxRunes int) string {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}
