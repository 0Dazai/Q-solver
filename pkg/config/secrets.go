package config

import (
	"encoding/json"
	"fmt"
)

func saveProtectedSecrets(path string, secrets protectedSecrets) error {
	plain, err := json.Marshal(secrets)
	if err != nil {
		return err
	}
	return saveTranscriptionSecret(path, string(plain))
}

func loadProtectedSecrets(path string) (protectedSecrets, error) {
	secret, err := loadTranscriptionSecret(path)
	if err != nil || secret == "" {
		return protectedSecrets{}, err
	}
	var secrets protectedSecrets
	if err := json.Unmarshal([]byte(secret), &secrets); err != nil {
		return protectedSecrets{}, fmt.Errorf("解析受保护凭据: %w", err)
	}
	return secrets, nil
}
