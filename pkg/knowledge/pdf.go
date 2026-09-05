package knowledge

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// ExtractPDFText extracts the embedded text layer locally. Scanned PDFs need OCR
// before they can be indexed.
func ExtractPDFText(path string) (string, error) {
	if text, err := extractWithPoppler(path); err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}
	if text, err := extractWithOCR(path); err == nil && strings.TrimSpace(text) != "" {
		return text, nil
	}
	return "", fmt.Errorf("PDF 文本提取与本地 OCR 均未得到内容")
}

// pdfTool locates an external PDF helper binary. Lookup order:
//  1. PATH (works for standard installs and non-Windows systems);
//  2. Q_SOLVER_PDF_TOOLS environment variable (os.PathListSeparator separated),
//     letting users point at any portable poppler/tesseract directory;
//  3. MSYS2 environment roots and the common default MSYS2 install drives.
//
// No machine-specific path is required for a working install: a tool on PATH
// or a configured Q_SOLVER_PDF_TOOLS value is enough.
func pdfTool(name string) (string, error) {
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, dir := range pdfToolDirs() {
		candidate := filepath.Join(dir, name)
		if runtime.GOOS == "windows" {
			candidate += ".exe"
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("未找到本地 PDF 工具 %s，可将其加入 PATH 或用 Q_SOLVER_PDF_TOOLS 指定目录", name)
}

func pdfToolDirs() []string {
	if custom := strings.TrimSpace(os.Getenv("Q_SOLVER_PDF_TOOLS")); custom != "" {
		return filepath.SplitList(custom)
	}
	dirs := make([]string, 0, 4)
	for _, env := range []string{"MSYS2_HOME", "MSYS_HOME"} {
		if root := strings.TrimSpace(os.Getenv(env)); root != "" {
			dirs = append(dirs, filepath.Join(root, "mingw64", "bin"))
		}
	}
	dirs = append(dirs, `C:\msys64\mingw64\bin`, `D:\msys64\mingw64\bin`)
	return dirs
}

func extractWithPoppler(path string) (string, error) {
	tool, err := pdfTool("pdftotext")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, tool, "-layout", "-enc", "UTF-8", path, "-")
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func extractWithOCR(path string) (string, error) {
	renderer, err := pdfTool("pdftoppm")
	if err != nil {
		return "", err
	}
	ocr, err := pdfTool("tesseract")
	if err != nil {
		return "", err
	}
	tempDir, err := os.MkdirTemp("", "q-solver-pdf-ocr-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	prefix := filepath.Join(tempDir, "page")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	if output, err := exec.CommandContext(ctx, renderer, "-png", "-r", "120", path, prefix).CombinedOutput(); err != nil {
		return "", fmt.Errorf("PDF 页面渲染失败: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	images, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return "", err
	}
	sort.Strings(images)
	outputs := make([][]byte, len(images))
	errs := make(chan error, len(images))
	workers := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for index, image := range images {
		wg.Add(1)
		go func(index int, image string) {
			defer wg.Done()
			workers <- struct{}{}
			defer func() { <-workers }()
			command := exec.CommandContext(ctx, ocr, image, "stdout", "-l", "chi_sim+eng", "--psm", "6")
			output, err := command.Output()
			if err != nil {
				errs <- fmt.Errorf("PDF 第 %d 页 OCR 失败: %w", index+1, err)
				return
			}
			outputs[index] = output
		}(index, image)
	}
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		return "", err
	}
	var result bytes.Buffer
	for _, output := range outputs {
		if len(output) > 0 {
			result.Write(output)
			result.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(result.String()), nil
}
