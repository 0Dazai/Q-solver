package knowledge

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPdfToolEnvOverrideWins(t *testing.T) {
	dir := t.TempDir()
	// A unique name keeps the test independent from tools that may already be
	// on PATH of the development machine.
	name := "qsolver-test-only-pdf-tool"
	bin := name
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if err := os.WriteFile(filepath.Join(dir, bin), []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("Q_SOLVER_PDF_TOOLS", dir)
	path, err := pdfTool(name)
	if err != nil {
		t.Fatalf("env override must resolve the tool: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("unexpected tool path: %q", path)
	}
}

func TestPdfToolMissingReturnsActionableError(t *testing.T) {
	t.Setenv("Q_SOLVER_PDF_TOOLS", t.TempDir())
	t.Setenv("PATH", "")
	t.Setenv("MSYS2_HOME", "")
	t.Setenv("MSYS_HOME", "")
	_, err := pdfTool("definitely-not-a-real-pdf-tool")
	if err == nil {
		t.Fatal("missing tool must return an error")
	}
	if !strings.Contains(err.Error(), "Q_SOLVER_PDF_TOOLS") {
		t.Fatalf("error should mention the configuration variable: %v", err)
	}
}
