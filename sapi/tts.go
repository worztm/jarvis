// Package sapi drives the native Windows SAPI5 speech engines through two
// persistent PowerShell workers: one for recognition (asr.go), one for
// synthesis (this file). No cloud.
package sapi

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	_ "embed"
)

//go:embed scripts/tts.ps1
var ttsScript string

// TTS speaks text through the default SAPI voice (female preferred).
type TTS struct {
	mu      sync.Mutex
	cmd     *ttsProcess
	stdin   chan string
	started bool

	voiceMu sync.RWMutex
	voice   string
}

type ttsProcess struct {
	kill  func() error
	stdin ioWriter
}

type ioWriter = interface{ Write(p []byte) (int, error) }

// NewTTS spawns the synthesis worker; onVoice fires once the voice is armed.
func NewTTS(onVoice func(name string)) (*TTS, error) {
	scriptPath, err := writeTempScript("jarvis_tts.ps1", ttsScript)
	if err != nil {
		return nil, err
	}
	cmd := powershell(scriptPath)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start tts worker: %w", err)
	}

	tts := &TTS{
		stdin: make(chan string, 32),
		cmd: &ttsProcess{
			kill:  func() error { return cmd.Process.Kill() },
			stdin: stdinPipe,
		},
		started: true,
	}

	go func() { // writer pump
		for line := range tts.stdin {
			if _, err := stdinPipe.Write([]byte(line + "\n")); err != nil {
				return
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "VOICE|"):
				name := strings.TrimPrefix(line, "VOICE|")
				tts.voiceMu.Lock()
				tts.voice = name
				tts.voiceMu.Unlock()
				if onVoice != nil {
					onVoice(name)
				}
			case strings.HasPrefix(line, "ERR|"):
				fmt.Fprintln(os.Stderr, "[tts]", line)
			}
		}
	}()

	return tts, nil
}

func (t *TTS) send(line string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.started {
		return
	}
	select {
	case t.stdin <- line:
	default: // wedged worker -> drop, speech is best-effort
	}
}

// Speak cancels current playback and says text (built-in barge-in).
func (t *TTS) Speak(text string) { t.send("SAY|" + sanitize(text)) }

// Cancel stops playback immediately.
func (t *TTS) Cancel() { t.send("CANCEL|") }

// Voice returns the armed voice name ("" until reported).
func (t *TTS) Voice() string {
	t.voiceMu.RLock()
	defer t.voiceMu.RUnlock()
	return t.voice
}

// Close shuts the worker down gracefully.
func (t *TTS) Close() {
	t.send("QUIT|")
	t.mu.Lock()
	if t.started {
		close(t.stdin)
		t.started = false
	}
	t.mu.Unlock()
	if t.cmd != nil && t.cmd.kill != nil {
		_ = t.cmd.kill()
	}
}

func sanitize(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "|", "/")
}

func powershell(scriptPath string, args ...string) *exec.Cmd {
	full := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath}, args...)
	cmd := exec.Command("powershell", full...) //nolint:gosec // path we wrote ourselves
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

func writeTempScript(name, content string) (string, error) {
	dir, err := os.MkdirTemp("", "jarvis-sapi")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", err
	}
	return path, nil
}
