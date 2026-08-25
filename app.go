package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"jarvis/brain"
	"jarvis/sapi"
	"jarvis/whisper"
)

// App is the Wails-bound application core.
type App struct {
	ctx context.Context

	tts *sapi.TTS
	asr whisper.Engine

	processMu sync.Mutex
}

// NewApp constructs the bound app (no side effects; startup wires the rest).
func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Voice worker first - it reports the armed female voice to the UI.
	tts, err := sapi.NewTTS(func(name string) {
		emit("vox:name", name)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "[tts]", err)
	}
	a.tts = tts
	if tts != nil && tts.Voice() != "" {
		emit("vox:name", tts.Voice())
	}

	// Whisper recognition engine (server spawns lazily on first listen).
	a.asr.Server = &whisper.Server{}
	a.asr.OnSpeech = func(speaking bool) {
		if speaking {
			emit("asr:partial", "\u2026") // show live-capture state in the readout
		}
	}
	a.asr.OnPartial = func(text string) {
		emit("asr:partial", text)
	}
	a.asr.OnFinal = func(text string, conf float64) {
		emit("asr:final", map[string]any{"text": text, "confidence": conf})
		a.process(text)
	}
	a.asr.OnError = func(msg string) {
		emit("asr:error", msg)
	}
}

func (a *App) shutdown(ctx context.Context) {
	a.asr.StopListening()
	if a.tts != nil {
		a.tts.Close()
	}
	a.asr.Server.Shutdown()
}

// ---- bound methods (callable from the frontend) -----------------------------

// Submit processes one command through the brain and speaks the reply.
func (a *App) Submit(raw string) {
	go a.process(raw)
}

// StartListening arms whisper recognition (spawns the model server on first use).
func (a *App) StartListening() error {
	return a.asr.StartListening()
}

// StopListening disarms recognition.
func (a *App) StopListening() {
	a.asr.StopListening()
	emit("asr:stopped", true)
}

// Status returns live telemetry for the UI.
func (a *App) Status() map[string]any {
	voice := ""
	if a.tts != nil {
		voice = a.tts.Voice()
	}
	return map[string]any{
		"targets":   brain.TargetCount(),
		"sites":     len(brain.Sites),
		"apps":      len(brain.Apps),
		"voice":     voice,
		"listening": a.asr.Running(),
		"asr":       "whisper-tiny.en (local)",
		"started":   time.Now().Format(time.RFC3339),
	}
}

// CancelSpeech is barge-in from the UI side.
func (a *App) CancelSpeech() {
	if a.tts != nil {
		a.tts.Cancel()
	}
}

// ---- internals ----------------------------------------------------------------

var processMu sync.Mutex

func (a *App) process(raw string) {
	processMu.Lock()
	defer processMu.Unlock()

	raw = strings.TrimSpace(raw)
	if raw == "" || len([]rune(raw)) < 2 {
		return
	}
	result := brain.Think(raw)

	// stop/quiet intent: cancel speech, don't speak the ack over the silence
	if result.Action == "Speech cancelled" {
		if a.tts != nil {
			a.tts.Cancel()
		}
		emit("agent:reply", map[string]any{"reply": "", "action": result.Action})
		return
	}

	emit("agent:reply", map[string]any{
		"reply":  result.Reply,
		"action": result.Action,
	})
	if a.tts != nil {
		a.tts.Speak(result.Reply)
	}
}
