//go:build windows

package config

import (
	"os"
	"strings"
	"testing"
)

func TestTranscriptionSecretDPAPIRoundTrip(t *testing.T) {
	path := t.TempDir() + "\\qwen-secret"
	const secret = "qwen-test-key-not-a-real-credential"
	if err := saveTranscriptionSecret(path, secret); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatal("DPAPI file must not contain plaintext")
	}
	got, err := loadTranscriptionSecret(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != secret {
		t.Fatalf("unexpected secret: %q", got)
	}
}
