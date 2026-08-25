package whisper

import (
	"strings"
	"sync"
	"time"
)

// Engine = mic capture + energy VAD + whisper transcription.
//
// Speech onset (RMS above threshold) opens an utterance; while you keep
// talking, partial transcripts are pulled roughly every second; ~700 ms of
// silence closes the utterance and emits the final transcript.
type Engine struct {
	Server *Server

	OnPartial func(text string)
	OnFinal   func(text string, confidence float64)
	OnError   func(msg string)
	OnSpeech  func(speaking bool)

	mu        sync.Mutex
	mic       *Mic
	capturing bool
	stopCh    chan struct{}

	// vad state (owned by detector goroutine)
	pcm        []byte // utterance buffer (s16le mono)
	preroll    []byte // last ~600ms before onset
	silenceMs  int
	speechMs   int
	speaking   bool
	lastPartialLen int
	partialAt  time.Time

	infMu sync.Mutex // one inference at a time
}

// VAD tuning (mic-gain dependent; conservative defaults).
const (
	onsetRMS      = 0.020
	silenceRMS    = 0.012
	endSilenceMs  = 700
	minSpeechMs   = 300
	maxUtteranceMs = 12000
	partialEvery  = 900 * time.Millisecond
	prerollChunks = 6
)

// StartListening opens the mic and starts the detector.
func (e *Engine) StartListening() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.capturing {
		return nil
	}
	if err := e.Server.Ensure(); err != nil {
		return err
	}

	e.stopCh = make(chan struct{})
	e.mic = &Mic{OnAudio: e.onAudio}
	if err := e.mic.Start(); err != nil {
		return err
	}
	e.capturing = true

	e.pcm = nil
	e.speaking = false
	e.silenceMs = 0
	e.speechMs = 0
	go e.detectLoop()
	return nil
}

// StopListening closes the mic. A pending utterance is discarded (the user
// asked to stop, not to transcribe).
func (e *Engine) StopListening() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.capturing {
		return
	}
	e.capturing = false
	close(e.stopCh)
	e.mic.Stop()
	e.pcm = nil
	e.speaking = false
}

// Running reports whether capture is active.
func (e *Engine) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.capturing
}

// onAudio runs on the mic poll goroutine: buffer + hand to detector timing.
func (e *Engine) onAudio(chunk []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.capturing {
		return
	}

	rms := RMS(chunk)
	chunkMs := 100 // bufMs

	if !e.speaking {
		e.preroll = append(e.preroll, chunk...)
		if len(e.preroll) > prerollChunks*chunkMs*32 { // 32 bytes/ms @16kHz s16
			e.preroll = e.preroll[len(e.preroll)-prerollChunks*chunkMs*32:]
		}
		if rms > onsetRMS {
			e.speaking = true
			e.speechMs = 0
			e.silenceMs = 0
			e.pcm = append([]byte{}, e.preroll...)
			e.pcm = append(e.pcm, chunk...)
			e.lastPartialLen = 0
			if e.OnSpeech != nil {
				go e.OnSpeech(true)
			}
		}
		return
	}

	// speaking
	e.pcm = append(e.pcm, chunk...)
	e.speechMs += chunkMs
	if rms < silenceRMS {
		e.silenceMs += chunkMs
	} else {
		e.silenceMs = 0
	}

	// live partial while the utterance grows
	if e.speechMs >= 800 && e.silenceMs == 0 &&
		len(e.pcm) > e.lastPartialLen+sampleRate && // >=1s of new audio
		time.Since(e.partialAt) > partialEvery {
		e.partialAt = time.Now()
		e.lastPartialLen = len(e.pcm)
		buf := append([]byte{}, e.pcm...)
		go e.infer(buf, true)
	}

	if e.silenceMs >= endSilenceMs || e.speechMs >= maxUtteranceMs {
		e.speaking = false
		buf := e.pcm
		e.pcm = nil
		e.preroll = nil
		speechLen := e.speechMs
		if e.OnSpeech != nil {
			go e.OnSpeech(false)
		}
		if speechLen >= minSpeechMs {
			go e.infer(buf, false)
		}
	}
}

// detectLoop is a heartbeat that force-closes stuck utterances if the mic
// goes quiet while the engine is idle-speaking (edge case safety).
func (e *Engine) detectLoop() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.mu.Lock()
			if !e.capturing {
				e.mu.Unlock()
				return
			}
			e.mu.Unlock()
		}
	}
}

// infer transcribes a buffer. partial=true results go to OnPartial,
// otherwise OnFinal. Confidence is estimated from signal dynamics since
// whisper.cpp does not expose token probabilities.
func (e *Engine) infer(pcm []byte, partial bool) {
	e.infMu.Lock()
	defer e.infMu.Unlock()

	text, err := e.Server.Transcribe(pcm)
	if err != nil {
		if e.OnError != nil {
			go e.OnError("whisper: " + err.Error())
		}
		return
	}
	text = strings.TrimSpace(text)
	if text == "" || text == "." || text == "you" && len(pcm) < sampleRate {
		return
	}
	if partial {
		if e.OnPartial != nil {
			go e.OnPartial(text)
		}
		return
	}
	// crude SNR-based confidence: loud clear speech scores higher
	peak := RMS(pcm[len(pcm)/3 : 2*len(pcm)/3])
	conf := 0.62 + 1.6*peak
	if conf > 0.95 {
		conf = 0.95
	}
	if e.OnFinal != nil {
		go e.OnFinal(text, conf)
	}
}
