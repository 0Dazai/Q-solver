package resume

import (
	"Q-Solver/pkg/config"
	"Q-Solver/pkg/knowledge"
	"Q-Solver/pkg/logger"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Service struct {
	mu           sync.RWMutex
	config       config.Config
	resumeBase64 string
	extractPDF   func(string) (string, error)
}

func NewService(cfg config.Config, cm *config.ConfigManager) *Service {
	s := &Service{
		config:     cfg,
		extractPDF: knowledge.ExtractPDFText,
	}

	cm.Subscribe(func(newConfig config.Config, oldConfig config.Config) {
		s.mu.Lock()
		s.config = newConfig
		if newConfig.ResumePath != oldConfig.ResumePath {
			s.resumeBase64 = ""
		}
		s.mu.Unlock()
	})

	return s
}

func (s *Service) SelectResume(ctx context.Context) string {
	selection, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "选择简历 (PDF)",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "PDF Files",
				Pattern:     "*.pdf",
			},
		},
	})

	if err != nil {
		logger.Printf("选择文件失败: %v\n", err)
		return ""
	}

	if selection == "" {
		return ""
	}

	return selection
}

func (s *Service) ClearResume() {
	s.mu.Lock()
	s.resumeBase64 = ""
	s.mu.Unlock()
	logger.Println("简历缓存已清除")
}

func (s *Service) GetResumeBase64() (string, error) {
	s.mu.RLock()
	cached := s.resumeBase64
	resumePath := s.config.ResumePath
	s.mu.RUnlock()

	if len(cached) > 0 {
		logger.Println("使用缓存的简历 Base64")
		return cached, nil
	}
	if resumePath == "" {
		return "", nil
	}

	fileInfo, err := os.Stat(resumePath)
	if err != nil {
		return "", err
	}

	const maxResumeSize = 5 * 1024 * 1024
	if fileInfo.Size() > maxResumeSize {
		return "", fmt.Errorf("简历文件大小超过 5MB 限制")
	}

	content, err := os.ReadFile(resumePath)
	if err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(content)
	s.mu.Lock()
	s.resumeBase64 = encoded
	s.mu.Unlock()
	return encoded, nil
}

func (s *Service) ParseResume(_ context.Context) (string, error) {
	s.mu.RLock()
	resumePath := s.config.ResumePath
	s.mu.RUnlock()
	if strings.TrimSpace(resumePath) == "" {
		return "", fmt.Errorf("请先选择简历文件")
	}

	logger.Println("开始在本机提取简历 PDF")
	extractPDF := s.extractPDF
	if extractPDF == nil {
		extractPDF = knowledge.ExtractPDFText
	}
	content, err := extractPDF(resumePath)
	if err != nil {
		logger.Printf("本地简历解析失败: %v", err)
		return "", err
	}
	title := strings.TrimSuffix(filepath.Base(resumePath), filepath.Ext(resumePath))
	return "# " + title + "\n\n" + strings.TrimSpace(content) + "\n", nil
}
