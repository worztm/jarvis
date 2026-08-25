// Package whisper: local speech recognition via whisper.cpp's prebuilt
// server (whisper-server.exe + ggml-tiny.en). The model stays loaded in the
// server process; Go talks to it over 127.0.0.1 HTTP. Fully offline.
package whisper

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const (
	serverPort    = 8919
	serverAddr    = "127.0.0.1:8919"
	sampleRate    = 16000
	startupBudget = 30 * time.Second
)

// Server manages the whisper-server.exe child process.
type Server struct {
	mu  sync.Mutex
	cmd *exec.Cmd
}

// ServerPath resolves whisper assets relative to the running exe, falling
// back to the working directory (dev mode).
func assetPath(rel string) (string, error) {
	var bases []string
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Join(filepath.Dir(exe), "..", "whisper"))
	}
	bases = append(bases, "whisper")
	for _, base := range bases {
		p := filepath.Join(base, rel)
		if _, err := os.Stat(p); err == nil {
			return filepath.Abs(p)
		}
	}
	return "", fmt.Errorf("whisper asset not found: %s", rel)
}

// Ensure starts the server if it isn't running and waits until it answers.
func (s *Server) Ensure() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		// still alive?
		if tcpUp() {
			return nil
		}
		_ = s.cmd.Process.Kill()
		s.cmd = nil
	}
	if tcpUp() {
		return nil // someone else started it (dev)
	}

	exe, err := assetPath(filepath.Join("bin", "whisper-server.exe"))
	if err != nil {
		return err
	}
	model, err := assetPath(filepath.Join("models", "ggml-tiny.en.bin"))
	if err != nil {
		return err
	}

	cmd := exec.Command(exe, "-m", model, "--host", "127.0.0.1", "--port", fmt.Sprint(serverPort))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	logFile, err := os.OpenFile(filepath.Join(os.TempDir(), "jarvis-whisper-server.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start whisper server: %w", err)
	}
	s.cmd = cmd

	deadline := time.Now().Add(startupBudget)
	for time.Now().Before(deadline) {
		if !tcpUp() {
			time.Sleep(250 * time.Millisecond)
			continue
		}
		go func() { _ = cmd.Wait() }() // reap on exit
		return nil
	}
	return fmt.Errorf("whisper server did not become ready within %s", startupBudget)
}

// Shutdown kills the server process.
func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		s.cmd = nil
	}
}

func tcpUp() bool {
	conn, err := net.DialTimeout("tcp", serverAddr, 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Transcribe sends 16-bit mono 16 kHz PCM to the server, returns text.
func (s *Server) Transcribe(pcm16 []byte) (string, error) {
	if len(pcm16) < 3200 { // <100ms of audio: nothing to say
		return "", nil
	}
	wav := buildWav(pcm16, sampleRate)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(wav); err != nil {
		return "", err
	}
	_ = mw.WriteField("response_format", "json")
	if err := mw.Close(); err != nil {
		return "", err
	}

	resp, err := httpClient.Post("http://"+serverAddr+"/inference", mw.FormDataContentType(), &body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper server %s: %s", resp.Status, string(b))
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// buildWav wraps raw PCM16 mono data in a minimal RIFF/WAV container.
func buildWav(pcm []byte, rate int) []byte {
	dataLen := len(pcm)
	buf := bytes.NewBuffer(make([]byte, 0, 44+dataLen))
	write := func(v any) { _ = binary.Write(buf, binary.LittleEndian, v) }

	buf.WriteString("RIFF")
	write(uint32(36 + dataLen))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	write(uint32(16))
	write(uint16(1))          // PCM
	write(uint16(1))          // mono
	write(uint32(rate))       // sample rate
	write(uint32(rate * 2))   // byte rate
	write(uint16(2))          // block align
	write(uint16(16))         // bits per sample
	buf.WriteString("data")
	write(uint32(dataLen))
	buf.Write(pcm)
	return buf.Bytes()
}

// RMS computes root-mean-square of 16-bit mono PCM (little endian).
func RMS(pcm []byte) float64 {
	n := len(pcm) / 2
	if n == 0 {
		return 0
	}
	var sum float64
	for i := 0; i+1 < len(pcm); i += 2 {
		s := int16(binary.LittleEndian.Uint16(pcm[i:]))
		f := float64(s) / 32768
		sum += f * f
	}
	return math.Sqrt(sum / float64(n))
}
