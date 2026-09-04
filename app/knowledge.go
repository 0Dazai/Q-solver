package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"Q-Solver/pkg/config"
	"Q-Solver/pkg/knowledge"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SelectKnowledgeFiles() ([]string, error) {
	return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择本地资料（Markdown / PDF）",
		Filters: []runtime.FileFilter{{
			DisplayName: "Knowledge Files",
			Pattern:     "*.md;*.markdown;*.pdf",
		}},
	})
}

func (a *App) SelectKnowledgeDirectory() (string, error) {
	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "选择资料目录"})
}

func (a *App) IndexKnowledgeDirectory(directory string) error {
	var paths []string
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension == ".md" || extension == ".markdown" || extension == ".pdf" {
			paths = append(paths, path)
		}
		if len(paths) > 500 {
			return errors.New("单次目录导入最多 500 个资料文件")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("目录中没有 Markdown 或 PDF 文件")
	}
	return a.IndexKnowledgeFiles(paths)
}

func knowledgeFilesDirectory() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "Q-Solver", "knowledge", "files"), nil
}

func (a *App) GetKnowledgeStoragePath() (string, error) {
	return knowledgeFilesDirectory()
}

func (a *App) SaveKnowledgeNote(title, content string) (string, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" {
		return "", errors.New("标题和资料内容都需要填写")
	}
	replacer := strings.NewReplacer(`\`, "-", `/`, "-", ":", "-", "*", "-", "?", "-", `"`, "-", "<", "-", ">", "-", "|", "-")
	title = strings.Trim(replacer.Replace(title), ". ")
	if title == "" {
		title = "新建资料"
	}
	directory, err := knowledgeFilesDirectory()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(directory, title+".md")
	markdown := "# " + title + "\n\n" + content + "\n"
	if err := os.WriteFile(path, []byte(markdown), 0o600); err != nil {
		return "", err
	}
	if err := a.IndexKnowledgeFiles([]string{path}); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) SetKnowledgeAnswerMode(mode string) string {
	if mode != "general" && mode != "knowledge_first" && mode != "knowledge_only" {
		return "资料回答模式无效"
	}
	if err := a.configManager.Patch(func(cfg *config.Config) {
		cfg.Knowledge.AnswerMode = mode
		cfg.Knowledge.Enabled = true
	}); err != nil {
		return err.Error()
	}
	return ""
}

func (a *App) IndexKnowledgeFiles(paths []string) error {
	if a.knowledgeStore == nil {
		return errors.New("本地资料库尚未初始化")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	for _, path := range paths {
		extension := strings.ToLower(filepath.Ext(path))
		var content string
		switch extension {
		case ".md", ".markdown":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content = string(data)
		case ".pdf":
			text, err := knowledge.ExtractPDFText(path)
			if err != nil {
				return err
			}
			content = "# " + filepath.Base(path) + "\n\n" + text
		default:
			return errors.New("资料库支持 Markdown 和带文本层的 PDF 文件")
		}
		if err := a.knowledgeStore.IndexMarkdown(ctx, path, content); err != nil {
			return err
		}
	}
	a.EmitEvent("knowledge:changed")
	return nil
}

func (a *App) ReindexKnowledgeDocuments() error {
	documents, err := a.ListKnowledgeDocuments()
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(documents))
	for _, document := range documents {
		paths = append(paths, document.Path)
	}
	return a.IndexKnowledgeFiles(paths)
}

func (a *App) ListKnowledgeDocuments() ([]knowledge.Document, error) {
	if a.knowledgeStore == nil {
		return nil, errors.New("本地资料库尚未初始化")
	}
	documents, err := a.knowledgeStore.ListDocuments(a.ctx)
	if documents == nil {
		documents = []knowledge.Document{}
	}
	return documents, err
}

func (a *App) DeleteKnowledgeDocument(path string) error {
	if a.knowledgeStore == nil {
		return errors.New("本地资料库尚未初始化")
	}
	if err := a.knowledgeStore.DeleteDocument(a.ctx, path); err != nil {
		return err
	}
	a.EmitEvent("knowledge:changed")
	return nil
}

func (a *App) SearchKnowledge(query string) ([]knowledge.SearchResult, error) {
	if a.knowledgeStore == nil {
		return nil, errors.New("本地资料库尚未初始化")
	}
	results, err := a.knowledgeStore.Search(a.ctx, query, 10)
	if results == nil {
		results = []knowledge.SearchResult{}
	}
	return results, err
}
