package whisper

// Microphone capture via winmm (waveIn) — plain syscalls, zero dependencies.
// 16 kHz, mono, 16-bit PCM. Buffers are polled for WHDR_DONE and resubmitted.

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	winmm              = syscall.NewLazyDLL("winmm.dll")
	procWaveInOpen     = winmm.NewProc("waveInOpen")
	procWaveInClose    = winmm.NewProc("waveInClose")
	procWaveInStart    = winmm.NewProc("waveInStart")
	procWaveInStop     = winmm.NewProc("waveInStop")
	procWaveInReset    = winmm.NewProc("waveInReset")
	procWaveInAddBuf   = winmm.NewProc("waveInAddBuffer")
	procWaveInPrepHdr  = winmm.NewProc("waveInPrepareHeader")
	procWaveInUnprep   = winmm.NewProc("waveInUnprepareHeader")
)

const (
	waveMapper      = 0xFFFFFFFF
	wHdrDone        = 1
	bufMs           = 100
	bufCount        = 12
)

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

type waveHdr struct {
	LpData         uintptr
	DwBufferLength uint32
	DwBytesRecorded uint32
	DwUser         uintptr
	DwFlags        uint32
	DwLoops        uint32
	LpNext         uintptr
	Reserved       uintptr
}

// Mic owns the capture device. OnAudio receives each raw PCM chunk.
type Mic struct {
	mu       sync.Mutex
	handle   syscall.Handle
	bufs     []*waveHdr
	data     [][]byte
	hdrs     []byte // backing storage for waveHdr structs
	OnAudio  func(pcm []byte)
	capturing bool
}

// Start opens the default input device and begins streaming chunks.
func (m *Mic) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.capturing {
		return nil
	}

	format := waveFormatEx{
		FormatTag:      1, // PCM
		Channels:       1,
		SamplesPerSec:  sampleRate,
		AvgBytesPerSec: sampleRate * 2,
		BlockAlign:     2,
		BitsPerSample:  16,
	}
	var handle syscall.Handle
	res, _, _ := procWaveInOpen.Call(
		uintptr(unsafe.Pointer(&handle)),
		waveMapper,
		uintptr(unsafe.Pointer(&format)),
		0, 0, 0, // CALLBACK_NULL
	)
	if res != 0 {
		return fmt.Errorf("waveInOpen failed (code %d) - is a microphone connected?", res)
	}
	m.handle = handle

	chunk := sampleRate / 1000 * bufMs * 2 // bytes per buffer
	m.bufs = make([]*waveHdr, bufCount)
	m.data = make([][]byte, bufCount)
	m.hdrs = make([]byte, bufCount*int(unsafe.Sizeof(waveHdr{})))

	for i := 0; i < bufCount; i++ {
		m.data[i] = make([]byte, chunk)
		hdr := (*waveHdr)(unsafe.Pointer(&m.hdrs[i*int(unsafe.Sizeof(waveHdr{}))]))
		hdr.LpData = uintptr(unsafe.Pointer(&m.data[i][0]))
		hdr.DwBufferLength = uint32(chunk)
		m.bufs[i] = hdr
		if res, _, _ := procWaveInPrepHdr.Call(uintptr(handle), uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); res != 0 {
			m.release()
			return fmt.Errorf("waveInPrepareHeader failed (code %d)", res)
		}
		if res, _, _ := procWaveInAddBuf.Call(uintptr(handle), uintptr(unsafe.Pointer(hdr)), unsafe.Sizeof(*hdr)); res != 0 {
			m.release()
			return fmt.Errorf("waveInAddBuffer failed (code %d)", res)
		}
	}

	if res, _, _ := procWaveInStart.Call(uintptr(handle)); res != 0 {
		m.release()
		return fmt.Errorf("waveInStart failed (code %d)", res)
	}
	m.capturing = true
	go m.poll()
	return nil
}

// poll drains completed buffers until Stop.
func (m *Mic) poll() {
	hdrSize := unsafe.Sizeof(waveHdr{})
	for {
		m.mu.Lock()
		if !m.capturing {
			m.mu.Unlock()
			return
		}
		handle := m.handle
		for i, hdr := range m.bufs {
			if hdr.DwFlags&wHdrDone == 0 {
				continue
			}
			n := int(hdr.DwBytesRecorded)
			if n > 0 && m.OnAudio != nil {
				chunk := make([]byte, n)
				copy(chunk, m.data[i][:n])
				m.mu.Unlock()
				m.OnAudio(chunk)
				m.mu.Lock()
			}
			// resubmit
			procWaveInUnprep.Call(uintptr(handle), uintptr(unsafe.Pointer(hdr)), hdrSize)
			hdr.DwBufferLength = uint32(len(m.data[i]))
			hdr.DwBytesRecorded = 0
			hdr.DwFlags = 0
			procWaveInPrepHdr.Call(uintptr(handle), uintptr(unsafe.Pointer(hdr)), hdrSize)
			procWaveInAddBuf.Call(uintptr(handle), uintptr(unsafe.Pointer(hdr)), hdrSize)
		}
		m.mu.Unlock()
		time.Sleep(25 * time.Millisecond)
	}
}

// Stop halts capture and releases the device.
func (m *Mic) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.capturing {
		return
	}
	m.capturing = false
	procWaveInStop.Call(uintptr(m.handle))
	procWaveInReset.Call(uintptr(m.handle))
	m.release()
}

func (m *Mic) release() {
	if m.handle == 0 {
		return
	}
	hdrSize := unsafe.Sizeof(waveHdr{})
	for _, hdr := range m.bufs {
		procWaveInUnprep.Call(uintptr(m.handle), uintptr(unsafe.Pointer(hdr)), hdrSize)
	}
	procWaveInClose.Call(uintptr(m.handle))
	m.handle = 0
	m.bufs = nil
}
