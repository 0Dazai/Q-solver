//go:build !windows

package config

import "fmt"

func saveTranscriptionSecret(path, secret string) error {
	return fmt.Errorf("当前平台不支持 Windows DPAPI 千问密钥存储")
}
func loadTranscriptionSecret(path string) (string, error) { return "", nil }
