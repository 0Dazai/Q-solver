//go:build !windows

package interview

import (
	"context"
	"errors"
)

type systemASRInput string

const (
	systemASRPCM systemASRInput = "pcm"
	systemASRMic systemASRInput = "microphone"
)

func NewSystemSpeechASR(context.Context, string, systemASRInput) (ASRClient, error) {
	return nil, errors.New("Windows 系统语音识别仅支持 Windows")
}
func NewSystemLoopbackASR(ctx context.Context, language string) (ASRClient, error) {
	return NewSystemSpeechASR(ctx, language, systemASRPCM)
}
func NewSystemMicrophoneASR(ctx context.Context, language string) (ASRClient, error) {
	return NewSystemSpeechASR(ctx, language, systemASRMic)
}
