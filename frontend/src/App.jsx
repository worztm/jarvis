import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Submit, StartListening, StopListening, Status } from './wailsjs/go/main/App'
import { EventsOn } from './wailsjs/runtime/runtime'

const STORAGE_KEY = 'jarvis-log-v3'
const METER_BARS = 56

const QUICK_COMMANDS = [
  { code: 'SYS/01', label: 'SYSTEM SCAN', command: 'Run a complete system scan' },
  { code: 'MEM/02', label: 'READ NOTES', command: 'Read my notes' },
  { code: 'AUTO/03', label: 'FOCUS MODE', command: 'Engage focus mode' },
  { code: 'INF/04', label: 'TIME CHECK', command: 'What time is it' },
]

// Arc reactor = the mic toggle. Rings spin when the channel is live.
function ArcReactor({ active, onClick }) {
  return (
    <button
      type="button"
      className={`reactor ${active ? 'reactor--live' : ''}`}
      onClick={onClick}
      aria-pressed={active}
      title={active ? 'Close channel (ALT+SPACE)' : 'Open channel (ALT+SPACE)'}
    >
      <svg viewBox="0 0 100 100" aria-hidden="true">
        <circle className="reactor-ring" cx="50" cy="50" r="47" fill="none" stroke="currentColor" strokeWidth="1.5" strokeDasharray="10 5" />
        <circle className="reactor-ring reactor-ring--reverse" cx="50" cy="50" r="38" fill="none" stroke="currentColor" strokeWidth="1" strokeDasharray="3 6" />
        <circle className="reactor-ring" cx="50" cy="50" r="29" fill="none" stroke="currentColor" strokeWidth="2.5" strokeDasharray="40 18" />
        <circle className="reactor-core" cx="50" cy="50" r="16" fill="currentColor" opacity="0.9" />
        <circle cx="50" cy="50" r="7" fill="#eafaff" />
      </svg>
    </button>
  )
}

const BOOT_LINE = {
  id: 'boot',
  role: 'assistant',
  text: 'JARVIS online. Native Windows speech engine armed. Speak or type, Commander.',
  time: '--:--:--',
}

function currentTime() {
  return new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(new Date())
}

function readStoredLog() {
  try {
    const stored = JSON.parse(window.localStorage.getItem(STORAGE_KEY) || 'null')
    if (Array.isArray(stored) && stored.length) return stored
  } catch { /* storage optional */ }
  return [BOOT_LINE]
}

export default function App() {
  const [command, setCommand] = useState('')
  const [log, setLog] = useState(readStoredLog)
  const [events, setEvents] = useState([])
  const [listening, setListening] = useState(false)
  const [linkState, setLinkState] = useState('LINK')
  const [partial, setPartial] = useState('')
  const [lastAction, setLastAction] = useState(null)
  const [lastConf, setLastConf] = useState(null)
  const [speechState, setSpeechState] = useState('idle')
  const [voxName, setVoxName] = useState('ARMING...')
  const [targets, setTargets] = useState(null)
  const [levels, setLevels] = useState(() => Array(METER_BARS).fill(0))
  const [clock, setClock] = useState(currentTime)

  const inputRef = useRef(null)
  const logEndRef = useRef(null)
  const analyserRef = useRef(null)
  const streamRef = useRef(null)

  useEffect(() => {
    const t = window.setInterval(() => setClock(currentTime()), 1000)
    return () => window.clearInterval(t)
  }, [])

  useEffect(() => {
    try { window.localStorage.setItem(STORAGE_KEY, JSON.stringify(log.slice(-40))) } catch { /* optional */ }
  }, [log])

  const pushEvent = useCallback((code, text) => {
    setEvents((current) => [{ id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`, code, text, time: currentTime() }, ...current].slice(0, 12))
  }, [])

  // ---- wire every backend event -------------------------------------------
  useEffect(() => {
    const offs = []
    offs.push(EventsOn('vox:name', (name) => setVoxName((name || 'DEFAULT').toUpperCase().slice(0, 22))))
    offs.push(EventsOn('asr:ready', () => {
      setLinkState('LIVE')
      pushEvent('MIC', 'SAPI RECOGNIZER ARMED')
    }))
    offs.push(EventsOn('asr:partial', (text) => {
      setSpeechState('active')
      setPartial(text)
    }))
    offs.push(EventsOn('asr:final', ({ text, confidence }) => {
      setPartial('')
      setLastConf(confidence ?? null)
      pushEvent('IN', text.toUpperCase())
      setLog((current) => [...current, { id: `u-${Date.now()}-${Math.random().toString(36).slice(2, 5)}`, role: 'user', text, time: currentTime() }])
    }))
    offs.push(EventsOn('agent:reply', ({ reply, action }) => {
      setLastAction(action || null)
      pushEvent('OUT', reply.length > 58 ? `${reply.slice(0, 58).toUpperCase()}...` : reply.toUpperCase())
      setLog((current) => [...current, { id: `a-${Date.now()}-${Math.random().toString(36).slice(2, 5)}`, role: 'assistant', text: reply, time: currentTime() }])
    }))
    offs.push(EventsOn('asr:error', (msg) => {
      // intentional stops are reported as info, not faults
      if (String(msg).includes('unexpectedly')) {
        setLinkState('FAULT')
        pushEvent('ERR', String(msg).toUpperCase())
      } else {
        pushEvent('MIC', String(msg).toUpperCase())
      }
    }))
    offs.push(EventsOn('asr:stopped', () => {
      setLinkState('LINK')
      setPartial('')
    }))

    // initial telemetry
    Status?.().then((s) => {
      if (!s) return
      setTargets(s.targets ?? null)
      if (s.voice && !s.voice.startsWith('<')) setVoxName(s.voice.toUpperCase().slice(0, 22))
    }).catch(() => {})

    return () => offs.forEach((off) => typeof off === 'function' && off())
  }, [pushEvent])

  // ---- VU meter: own mic tap, purely visual --------------------------------
  useEffect(() => {
    if (!listening) {
      setLevels(Array(METER_BARS).fill(0))
      analyserRef.current = null
      streamRef.current?.getTracks().forEach((track) => track.stop())
      streamRef.current = null
      return undefined
    }
    let raf = 0
    let cancelled = false
    ;(async () => {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        if (cancelled) { stream.getTracks().forEach((t) => t.stop()); return }
        streamRef.current = stream
        const ctx = new AudioContext({ latencyHint: 'interactive' })
        const source = ctx.createMediaStreamSource(stream)
        const analyser = ctx.createAnalyser()
        analyser.fftSize = 512
        source.connect(analyser)
        analyserRef.current = analyser
        const buf = new Float32Array(analyser.fftSize)

        const tick = () => {
          raf = requestAnimationFrame(tick)
          const current = analyserRef.current
          if (!current) return
          current.getFloatTimeDomainData(buf)
          let sum = 0
          for (let i = 0; i < buf.length; i++) sum += buf[i] * buf[i]
          const rms = Math.sqrt(sum / buf.length)
          const level = Math.min(1, rms * 6)
          setLevels((arr) => [...arr.slice(1), level])
        }
        raf = requestAnimationFrame(tick)
      } catch { /* meter is cosmetic */ }
    })()
    return () => {
      cancelled = true
      cancelAnimationFrame(raf)
    }
  }, [listening])

  const submitCommand = useCallback(async (value) => {
    const raw = (value ?? command).trim()
    if (!raw) return
    setCommand('')
    setPartial('')
    pushEvent('IN', raw.toUpperCase())
    setLog((current) => [...current, { id: `u-${Date.now()}`, role: 'user', text: raw, time: currentTime() }])
    try { await Submit(raw) } catch { pushEvent('ERR', 'BACKEND UNREACHABLE') }
  }, [command, pushEvent])

  const toggleListening = useCallback(async () => {
    if (listening) {
      StopListening()
      setListening(false)
      setSpeechState('idle')
      return
    }
    try {
      await StartListening()
      setListening(true)
      pushEvent('MIC', 'AUDIO CHANNEL OPEN / SAPI STREAMING')
    } catch (error) {
      pushEvent('ERR', String(error?.message || 'RECOGNIZER FAILED TO ARM').toUpperCase())
      setLinkState('FAULT')
    }
  }, [listening, pushEvent])

  useEffect(() => {
    const onKey = (event) => {
      if (event.altKey && event.code === 'Space') { event.preventDefault(); toggleListening() }
      if (event.key === '/' && document.activeElement?.tagName !== 'INPUT') { event.preventDefault(); inputRef.current?.focus() }
      if (event.key === 'Escape') inputRef.current?.blur()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [toggleListening])

  useEffect(() => {
    logEndRef.current?.scrollIntoView({ block: 'end' })
  }, [log])

  const linkTone = listening ? 'live' : linkState === 'FAULT' ? 'fault' : 'idle'
  const peak = Math.max(...levels)
  const processing = partial === '' && speechState === 'idle'

  return (
    <div className="deck">
      <div className="scanlines" aria-hidden="true" />
      <div className="grain" aria-hidden="true" />

      {/* ================= MASTER HEADER ================= */}
      <header className="master-header">
        <div className="brand">
          <span className="brand-mark">JARVIS<sup>&reg;</sup></span>
          <span className="brand-sub">TACTICAL SPEECH INTERFACE</span>
        </div>
        <ArcReactor active={listening} onClick={toggleListening} />
        <div className="header-telemetry">
          <span className="ht-cell"><samp>LOCAL</samp> {clock}</span>
          <span className="ht-cell"><samp>TGTS</samp> {targets ?? '---'}</span>
          <span className="ht-cell ht-cell--state" data-tone={linkTone}><i className="dot" aria-hidden="true" />{listening ? (speechState === 'active' ? 'HEARING' : 'REC') : linkState}</span>
        </div>
      </header>
      <div className="hazard-stripe" aria-hidden="true" />

      {/* ================= UPTIME TICKER ================= */}
      <div className="ticker-row" aria-hidden="true">
        <span>WHISPER-TINY.EN /// GGML LOCAL INFERENCE /// ALL PROCESSING ON-DEVICE /// NO TELEMETRY LEAVES THIS UNIT /// REV 4.0 &copy; LOCAL WORKS</span>
      </div>

      {/* ================= MAIN GRID ================= */}
      <main className="main-grid">

        {/* --- LEFT: title + console + meter --- */}
        <section className="zone zone--command">
          <p className="zone-tag">[ COMMAND DECK ]</p>
          <h1 className="macro-type">VOICE<br />CONSOLE</h1>

          <div className="console-block">
            <form className="command-line" onSubmit={(e) => { e.preventDefault(); submitCommand() }}>
              <label className="prompt" htmlFor="cmd">&gt;&gt;&gt;</label>
              <input
                id="cmd"
                ref={inputRef}
                value={command}
                onChange={(e) => { setCommand(e.target.value); setPartial('') }}
                placeholder={listening ? 'LISTENING...' : 'TYPE COMMAND / PRESS /'}
                autoComplete="off"
                spellCheck="false"
              />
              <button type="button" className={`mic-switch ${listening ? 'mic-switch--on' : ''}`} onClick={toggleListening} aria-pressed={listening}>
                {listening ? '[ MIC // LIVE ]' : '[ MIC ]'}
              </button>
              <button type="submit" className="exec-switch" disabled={!command.trim()}>EXEC</button>
            </form>

            <output className="live-readout" htmlFor="cmd">
              {partial ? <><span className="readout-tag">ASR&gt;</span> {partial}<span className="caret" aria-hidden="true">_</span></>
                : !processing ? <><span className="readout-tag">AGT&gt;</span> PROCESSING...</>
                  : <><span className="readout-tag readout-tag--dim">RDY&gt;</span> {listening ? 'CONTINUOUS CAPTURE // SPEAK FREELY // NEW COMMAND INTERRUPTS SPEECH' : 'CHANNEL STANDBY \u2014 ALT+SPACE FOR VOICE'}</>}
            </output>

            <div className="vu" role="img" aria-label={`Microphone level ${Math.round(peak * 100)} percent`}>
              {levels.map((level, index) => (
                <span key={index} data-hot={level > 0.55 || undefined} style={{ height: `${Math.max(3, Math.round(level * 100))}%` }} />
              ))}
            </div>
            <div className="vu-meta">
              <span>SIG/PCM16-16K</span>
              <span>{listening ? `PEAK ${Math.round(peak * 100)}%` : 'MUTED'}</span>
              <span>NATIVE WINDOWS AUDIO</span>
            </div>
          </div>

          <div className="quick-strip">
            {QUICK_COMMANDS.map((qc) => (
              <button key={qc.code} type="button" className="quick-cmd" onClick={() => submitCommand(qc.command)}>
                <samp>{qc.code}</samp> {qc.label}
              </button>
            ))}
          </div>
        </section>

        {/* --- RIGHT: route resolution + event stream --- */}
        <aside className="zone zone--telemetry">
          <p className="zone-tag">[ ROUTE RESOLUTION ]</p>
          <dl className="route-table">
            <div><dt>ACTION</dt><dd>{lastAction ? lastAction.toUpperCase() : '-- AWAITING --'}</dd></div>
            <div><dt>CONF</dt><dd><data value={lastConf != null && lastConf > 0 ? Math.round(lastConf * 100) : 0}>{lastConf != null && lastConf > 0 ? `${Math.round(lastConf * 100)}%` : '---'}</data></dd></div>
            <div><dt>BRAIN</dt><dd>LOCAL GO CORE</dd></div>
            <div><dt>EARS</dt><dd>WHISPER-TINY.EN</dd></div>
          </dl>

          <p className="zone-tag">[ EVENT STREAM ]</p>
          <ul className="event-stream">
            {events.length === 0 && <li className="ev-empty">// NO EVENTS THIS SESSION_</li>}
            {events.map((event) => (
              <li key={event.id}>
                <time>{event.time}</time>
                <kbd data-kind={event.code}>{event.code}</kbd>
                <span>{event.text}</span>
              </li>
            ))}
          </ul>
        </aside>

        {/* --- BOTTOM: full-width transmission log --- */}
        <section className="zone zone--log">
          <div className="log-head">
            <p className="zone-tag">[ TRANSMISSION LOG ]</p>
            <button type="button" className="purge" onClick={() => { setLog([BOOT_LINE]); setEvents([]); try { window.localStorage.removeItem(STORAGE_KEY) } catch { /* optional */ } }}>
              PURGE BUFFER ///
            </button>
          </div>
          <ol className="transmission-log">
            {log.map((entry) => (
              <li key={entry.id} className={`tx tx--${entry.role}`}>
                <time>{entry.time}</time>
                <strong>{entry.role === 'assistant' ? 'JARVIS' : 'CMDR'}</strong>
                <p>{entry.text}</p>
              </li>
            ))}
            <li ref={logEndRef} aria-hidden="true" />
          </ol>
        </section>
      </main>

      {/* ================= FOOTER ================= */}
      <footer className="deck-footer">
        <span>JARVIS&reg; UNIT MS-01</span>
        <span className="foot-dot" aria-hidden="true" />
        <span>EARS: WHISPER LOCAL</span>
        <span className="foot-dot" aria-hidden="true" />
        <span>BRAIN: LOCAL GO</span>
        <span className="foot-dot" aria-hidden="true" />
        <span>VOX: {voxName}</span>
        <span className="foot-spacer" />
        <span>ALT+SPACE VOICE / FOCUS INPUT</span>
      </footer>
    </div>
  )
}
