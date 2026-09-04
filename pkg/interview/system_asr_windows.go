//go:build windows

package interview

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type systemASRInput string

const (
	systemASRPCM systemASRInput = "pcm"
	systemASRMic systemASRInput = "microphone"
)

// SystemSpeechASR wraps Windows' built-in System.Speech recognizer. It keeps
// audio and recognized text on the device and emits the same event shape used
// by the DashScope client.
type SystemSpeechASR struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	events chan TranscriptEvent
	errs   chan error
	done   chan struct{}
	mu     sync.Mutex
	readWG sync.WaitGroup
	closed bool
}

func NewSystemSpeechASR(ctx context.Context, language string, input systemASRInput) (*SystemSpeechASR, error) {
	culture, err := systemSpeechCulture(language)
	if err != nil {
		return nil, err
	}
	cmd := newSystemSpeechCommand()
	cmd.Env = append(os.Environ(), "Q_SOLVER_SYSTEM_ASR_CULTURE="+culture, "Q_SOLVER_SYSTEM_ASR_INPUT="+string(input))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 Windows 系统语音识别: %w", err)
	}
	if input == systemASRMic {
		_ = stdin.Close()
		stdin = nil
	}
	c := &SystemSpeechASR{cmd: cmd, stdin: stdin, events: make(chan TranscriptEvent, 32), errs: make(chan error, 4), done: make(chan struct{})}
	c.readWG.Add(2)
	go c.readEvents(stdout)
	go c.readErrors(stderr)
	go func() {
		err := cmd.Wait()
		c.readWG.Wait()
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if err != nil && !closed {
			c.report(fmt.Errorf("Windows 系统语音识别已退出: %w", err))
		}
		close(c.done)
		close(c.events)
		close(c.errs)
	}()
	go func() {
		<-ctx.Done()
		_ = c.Close()
	}()
	return c, nil
}

func newSystemSpeechCommand() *exec.Cmd {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", systemSpeechScript)
	// System.Speech is hosted in a helper process. It is not an interactive
	// terminal, so creating a console window would only interrupt the desktop UI.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

func systemSpeechCulture(language string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "", "zh", "zh-cn":
		return "zh-CN", nil
	default:
		return "", fmt.Errorf("Windows 系统语音识别未配置语言 %q", language)
	}
}

func (c *SystemSpeechASR) SendAudio(pcm []byte) error {
	if len(pcm) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("Windows 系统语音识别已关闭")
	}
	if c.stdin == nil {
		return nil // The microphone-backed recognizer owns the default device itself.
	}
	_, err := c.stdin.Write(pcm)
	return err
}
func (c *SystemSpeechASR) Events() <-chan TranscriptEvent { return c.events }
func (c *SystemSpeechASR) Errors() <-chan error           { return c.errs }
func (c *SystemSpeechASR) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	stdin, process := c.stdin, c.cmd.Process
	c.mu.Unlock()
	if stdin != nil {
		_ = stdin.Close()
	}
	if process != nil {
		_ = process.Kill()
	}
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		return errors.New("Windows 系统语音识别未在预期时间内停止")
	}
	return nil
}
func (c *SystemSpeechASR) readEvents(reader io.Reader) {
	defer c.readWG.Done()
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		var raw struct {
			Kind string `json:"kind"`
			Text string `json:"text"`
		}
		if json.Unmarshal(scanner.Bytes(), &raw) != nil || strings.TrimSpace(raw.Text) == "" {
			continue
		}
		kind := EventPartial
		if raw.Kind == "final" {
			kind = EventFinal
		}
		select {
		case c.events <- TranscriptEvent{Kind: kind, Text: raw.Text, Timestamp: time.Now()}:
		case <-c.done:
			return
		}
	}
}
func (c *SystemSpeechASR) readErrors(reader io.Reader) {
	defer c.readWG.Done()
	data, _ := io.ReadAll(io.LimitReader(reader, 4096))
	if text := strings.TrimSpace(string(data)); text != "" {
		c.report(errors.New(text))
	}
}
func (c *SystemSpeechASR) report(err error) {
	select {
	case c.errs <- err:
	default:
	}
}

func NewSystemLoopbackASR(ctx context.Context, language string) (ASRClient, error) {
	return NewSystemSpeechASR(ctx, language, systemASRPCM)
}
func NewSystemMicrophoneASR(ctx context.Context, language string) (ASRClient, error) {
	return NewSystemSpeechASR(ctx, language, systemASRMic)
}

const systemSpeechScript = `$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Speech
Add-Type @'
using System;
using System.IO;

public sealed class QSolverSeekableInputStream : Stream
{
    private readonly Stream inner;
    private long position;

    public QSolverSeekableInputStream(Stream inner) { this.inner = inner; }
    public override bool CanRead { get { return inner.CanRead; } }
    public override bool CanSeek { get { return true; } }
    public override bool CanWrite { get { return false; } }
    public override long Length { get { return long.MaxValue; } }
    public override long Position
    {
        get { return position; }
        set
        {
            if (value != 0 && value != position) throw new NotSupportedException();
            position = value;
        }
    }
    public override void Flush() { }
    public override int Read(byte[] buffer, int offset, int count)
    {
        int read = inner.Read(buffer, offset, count);
        position += read;
        return read;
    }
    public override long Seek(long offset, SeekOrigin origin)
    {
        if ((origin == SeekOrigin.Begin && offset == 0) || (origin == SeekOrigin.Current && offset == 0)) return position;
        throw new NotSupportedException();
    }
    public override void SetLength(long value) { throw new NotSupportedException(); }
    public override void Write(byte[] buffer, int offset, int count) { throw new NotSupportedException(); }
}
'@
$culture = [System.Globalization.CultureInfo]::GetCultureInfo($env:Q_SOLVER_SYSTEM_ASR_CULTURE)
$engine = [System.Speech.Recognition.SpeechRecognitionEngine]::new($culture)
$engine.LoadGrammar([System.Speech.Recognition.DictationGrammar]::new())
if ($env:Q_SOLVER_SYSTEM_ASR_INPUT -eq 'pcm') {
  $format = [System.Speech.AudioFormat.SpeechAudioFormatInfo]::new(16000, [System.Speech.AudioFormat.AudioBitsPerSample]::Sixteen, [System.Speech.AudioFormat.AudioChannel]::Mono)
  $input = [QSolverSeekableInputStream]::new([Console]::OpenStandardInput())
  $engine.SetInputToAudioStream($input, $format)
} else {
  $engine.SetInputToDefaultAudioDevice()
}
function Write-Recognition($kind, $text) {
  if (![string]::IsNullOrWhiteSpace($text)) {
    [Console]::Out.WriteLine((@{ kind = $kind; text = $text } | ConvertTo-Json -Compress))
    [Console]::Out.Flush()
  }
}
$engine.add_SpeechHypothesized({ param($sender, $event) Write-Recognition 'partial' $event.Result.Text })
$engine.add_SpeechRecognized({ param($sender, $event) Write-Recognition 'final' $event.Result.Text })
$engine.RecognizeAsync([System.Speech.Recognition.RecognizeMode]::Multiple)
while ($true) { Start-Sleep -Milliseconds 200 }`
