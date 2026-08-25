// Package brain: JARVIS's local reasoning. Fuzzy open commands (sites, apps,
// local files), live weather, math, notes, timers, and hallucination-tolerant
// intent matching.
package brain

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var startedAt = time.Now()

// Result is what the brain decided for one command.
type Result struct {
	Reply  string
	Action string
}

// ---- varied replies (kills the parrot effect) ------------------------------

var (
	replyMu     sync.Mutex
	lastReplies = make([]string, 0, 5)
)

func pick(pool []string) string {
	replyMu.Lock()
	defer replyMu.Unlock()
	for attempt := 0; attempt < 8; attempt++ {
		candidate := pool[randomInt(len(pool))]
		if !contains(lastReplies, candidate) {
			lastReplies = append(lastReplies, candidate)
			if len(lastReplies) > 4 {
				lastReplies = lastReplies[1:]
			}
			return candidate
		}
	}
	lastReplies = append(lastReplies, pool[0])
	return pool[0]
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var rngMu sync.Mutex
var rngState = uint64(time.Now().UnixNano())

func randomInt(n int) int {
	rngMu.Lock()
	defer rngMu.Unlock()
	rngState ^= rngState << 13
	rngState ^= rngState >> 7
	rngState ^= rngState << 17
	return int(rngState % uint64(n))
}

var fallbacks = []string{
	"I don't have a route for that yet, Commander. Try \"open\" something, ask for the weather, or say help.",
	"That one slipped through the net. Say \"help\" and I'll show you my actual tricks.",
	"No match in my local index. You could try: open youtube, play a song, weather in Lagos, or what is 12 times 9.",
	"Unrecognized, but heard perfectly. Rephrase, or ask for help to see what I can do.",
}

var greetings = []string{
	"Online and at your service, Commander.",
	"All systems nominal. What are we doing today?",
	"Ready and listening, Commander.",
	"Here. Fully local, fully yours.",
}

var confirms = []string{
	"Opening %s.",
	"%s, coming right up.",
	"On it - launching %s.",
	"Done. %s is open.",
}

var suggestions = []string{
	"Did you mean %s? Say yes and I will open it.",
	"Closest match in my index is %s. Want it opened?",
}

// ---- pending suggestion memory (yes/no flow) --------------------------------

type pendingSuggestion struct {
	kind string // "site" | "app" | "file"
	key  string
	set  time.Time
}

var (
	pendingMu   sync.Mutex
	pendingSugg *pendingSuggestion
)

func setPending(kind, key string) {
	pendingMu.Lock()
	pendingSugg = &pendingSuggestion{kind: kind, key: key, set: time.Now()}
	pendingMu.Unlock()
}

func popPending() *pendingSuggestion {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	p := pendingSugg
	pendingSugg = nil
	if p != nil && time.Since(p.set) > 3*time.Minute {
		return nil
	}
	return p
}

// ---- regexes ---------------------------------------------------------------

var (
	reTime   = regexp.MustCompile(`\btime\b`)
	reDate   = regexp.MustCompile(`\b(date|day is it)\b|^today\b|whats today`)
	reWeather = regexp.MustCompile(`\b(weather|temperature|forecast)\b|(how )?(hot|cold|raining|rain)( is it| outside)?`)
	reStatus = regexp.MustCompile(`\b(status|report|diagnostic|scan|systems check|health)\b`)
	reCity   = regexp.MustCompile(`(?:weather|temperature|forecast)(?: report)?(?: in| for| at) ([a-z\s]+)$`)
	reTimer  = regexp.MustCompile(`(?:timer|reminder|alarm)(?: for)?(?: (\w+))? ?(\d+) ?(minute|min|second|sec|hour)`)
	reDigit  = regexp.MustCompile(`\d`)
	reMathOp = regexp.MustCompile(`plus|minus|times|divided|over|\+|-|\*|/|power`)
)

// ---- math: tiny recursive-descent evaluator ---------------------------------

type calc struct {
	input string
	pos   int
}

// TryMath extracts and evaluates an arithmetic expression from free text.
func TryMath(text string) (string, bool) {
	expr := strings.NewReplacer(
		"divided by", "/", "multiplied by", "*", "times", "*",
		"over", "/", "minus", "-", "plus", "+", "to the power of", "^",
	).Replace(text)
	expr = strings.Map(func(r rune) rune {
		switch r {
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
			'+', '-', '*', '/', '^', '(', ')', '.':
			return r
		default:
			return ' '
		}
	}, expr)
	expr = strings.Join(strings.Fields(expr), "")
	if len(expr) < 3 || !strings.ContainsAny(expr, "+-*/^") {
		return "", false
	}
	c := &calc{input: expr}
	v, err := c.parseExpr(0)
	if err != nil || c.pos != len(c.input) {
		return "", false
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return "", false
	}
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10), true
	}
	return strconv.FormatFloat(v, 'f', 4, 64), true
}

func (c *calc) parseExpr(minPrec int) (float64, error) {
	left, err := c.parseAtom()
	if err != nil {
		return 0, err
	}
	for c.pos < len(c.input) {
		op := rune(c.input[c.pos])
		prec := opPrec(op)
		if prec <= minPrec {
			return left, nil
		}
		c.pos++
		right, err := c.parseExpr(prec)
		if err != nil {
			return 0, err
		}
		switch op {
		case '+':
			left += right
		case '-':
			left -= right
		case '*':
			left *= right
		case '/':
			left /= right
		case '^':
			left = math.Pow(left, right)
		}
	}
	return left, nil
}

func opPrec(op rune) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	case '^':
		return 3
	}
	return 0
}

func (c *calc) parseAtom() (float64, error) {
	skipSpace(c)
	if c.pos < len(c.input) && c.input[c.pos] == '(' {
		c.pos++
		v, err := c.parseExpr(0)
		if err != nil {
			return 0, err
		}
		skipSpace(c)
		if c.pos >= len(c.input) || c.input[c.pos] != ')' {
			return 0, fmt.Errorf("unbalanced paren")
		}
		c.pos++
		return v, nil
	}
	start := c.pos
	for c.pos < len(c.input) && (c.input[c.pos] >= '0' && c.input[c.pos] <= '9' || c.input[c.pos] == '.') {
		c.pos++
	}
	if start == c.pos {
		return 0, fmt.Errorf("expected number at %d", c.pos)
	}
	return strconv.ParseFloat(c.input[start:c.pos], 64)
}

func skipSpace(c *calc) {
	for c.pos < len(c.input) && c.input[c.pos] == ' ' {
		c.pos++
	}
}

// ---- launchers ---------------------------------------------------------------

// shellOpen resolves via ShellExecute semantics: protocols, App Paths
// registry, PATH, and file associations all just work.
func shellOpen(target string) error {
	lower := strings.ToLower(target)
	isProtocol := strings.Contains(target, "://") || strings.HasSuffix(lower, ":")
	if isProtocol {
		return startHidden(exec.Command("rundll32", "url.dll,FileProtocolHandler", target))
	}
	// direct exe on PATH
	if !strings.HasSuffix(lower, ".msc") {
		if p, err := exec.LookPath(target); err == nil {
			return startHidden(exec.Command(p))
		}
	}
	// cmd start resolves App Paths registry + file associations
	return startHidden(exec.Command("cmd", "/c", "start", "", target))
}

func OpenURL(rawURL string) error {
	return startHidden(exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL))
}

func LaunchApp(candidates []string) error {
	for _, cand := range candidates {
		if err := shellOpen(cand); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no launcher worked for %v", candidates)
}

func startHidden(cmd *exec.Cmd) error {
	cmd.SysProcAttr = hideWindowAttr()
	return cmd.Start()
}

func hideWindowAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}

// ---- notes -------------------------------------------------------------------

func notesPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	dir := filepath.Join(base, "JARVIS")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "notes.txt")
}

func osUserHome() (string, error) { return os.UserHomeDir() }

// ---- beeper --------------------------------------------------------------------

var user32 = syscall.NewLazyDLL("user32.dll")
var messageBeep = user32.NewProc("MessageBeep")

func beepAlarm(times int) {
	go func() {
		for i := 0; i < times; i++ {
			messageBeep.Call(0x00000030) // MB_ICONEXCLAMATION
			time.Sleep(300 * time.Millisecond)
		}
	}()
}

// ---- the brain ----------------------------------------------------------------

// Think turns a normalized command into a spoken reply + action description.
func Think(raw string) Result {
	text := sanitize(raw)
	now := time.Now()

	// ---- yes/no: execute or drop a pending suggestion ----
	if isAffirmative(text) {
		if p := popPending(); p != nil {
			return executeTarget(p.kind, p.key)
		}
		return Result{"Nothing is pending, Commander. Give me a command.", "Confirm with no pending item"}
	}
	if isNegative(text) {
		if popPending() != nil {
			return Result{"Cancelled. Standing by.", "Suggestion declined"}
		}
	}

	// ---- app closing: "close notepad", "quit spotify" ----
	if hasCloseTrigger(text) {
		return closeResult(text)
	}

	// ---- fuzzy intent prototypes (hallucination-tolerant) ----
	// Open-style commands bypass prototypes: "open notepad" must never
	// fuzzy-match the notes intent ("notepad" ≈ "notes").
	openStyle := hasOpenTriggerFuzzy(text)
	if !openStyle {
		intent, _ := bestIntent(text)
		switch intent {
		case "greeting":
			return Result{pick(greetings), "Greeting acknowledged"}
		case "time":
			return Result{fmt.Sprintf("It is %s.", now.Format("3:04 PM")), "Time reported"}
		case "date":
			return Result{fmt.Sprintf("Today is %s.", now.Format("Monday, January 2, 2006")), "Date reported"}
		case "weather":
			return weatherResult(text)
		case "status":
			return statusResult()
		case "help":
			return capabilitiesResult()
		case "notes-read":
			return readNotesResult()
		case "focus":
			return Result{"Focus protocol engaged. Go build something great.", "Focus mode engaged"}
		case "thanks":
			return Result{pick([]string{"Always, Commander.", "At your service.", "Anytime."}), "Thanks acknowledged"}
		case "stop":
			return Result{Reply: "Standing down. Channel stays open.", Action: "Speech cancelled"}
		}
	}

	// ---- exact fast paths ----
	if reTime.MatchString(text) && containsAny(text, "what", "tell", "current", "is it") {
		return Result{fmt.Sprintf("It is %s.", now.Format("3:04 PM")), "Time reported"}
	}
	if reDate.MatchString(text) {
		return Result{fmt.Sprintf("Today is %s.", now.Format("Monday, January 2, 2006")), "Date reported"}
	}
	if reWeather.MatchString(text) {
		return weatherResult(text)
	}
	if reStatus.MatchString(text) {
		return statusResult()
	}

	// ---- notes ----
	if strings.HasPrefix(text, "note ") || strings.HasPrefix(text, "remember ") ||
		strings.Contains(text, "take a note") || strings.Contains(text, "write this down") || strings.Contains(text, "write down") {
		payload := text
		for _, cut := range []string{"take a note ", "write this down ", "write down ", "note ", "remember "} {
			payload = strings.TrimPrefix(payload, cut)
		}
		stamp := now.Format("2006-01-02 15:04")
		f, err := os.OpenFile(notesPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			fmt.Fprintf(f, "[%s] %s\n", stamp, payload)
			f.Close()
			return Result{fmt.Sprintf("Noted. \"%s\" is committed to memory.", payload), "Note saved"}
		}
		return Result{"I could not reach the notebook on disk.", "Note save failed"}
	}
	if (strings.Contains(text, "read") && strings.Contains(text, "note")) || strings.Contains(text, "my notes") {
		return readNotesResult()
	}

	// ---- timer ----
	if m := reTimer.FindStringSubmatch(text); m != nil {
		amount, _ := strconv.Atoi(m[2])
		unit := m[3]
		var d time.Duration
		var label string
		switch {
		case strings.HasPrefix(unit, "hour"):
			d, label = time.Duration(amount)*time.Hour, fmt.Sprintf("%d hours", amount)
		case strings.HasPrefix(unit, "min"):
			d, label = time.Duration(amount)*time.Minute, fmt.Sprintf("%d minutes", amount)
		default:
			d, label = time.Duration(amount)*time.Second, fmt.Sprintf("%d seconds", amount)
		}
		time.AfterFunc(d, func() { beepAlarm(3) })
		return Result{fmt.Sprintf("Timer armed for %s. I will sound off.", label), fmt.Sprintf("Timer set: %s", label)}
	}

	// ---- math ----
	if reDigit.MatchString(text) && reMathOp.MatchString(text) {
		if result, ok := TryMath(text); ok {
			return Result{fmt.Sprintf("That comes to %s.", result), "Computed by voice"}
		}
	}

	// ---- open / play: sites, apps, local files ----
	hasTrigger := openStyle
	kind, key, score, matched := matchTarget(text)
	_ = kind
	if matched && ((hasTrigger && score >= 0.66) || score >= 0.85) {
		return executeTarget(targetKind(kind, key), key)
	}

	// ---- local media / files / folders ----
	if hasTrigger {
		category, _ := mediaCategoryFor(text)
		query := extractFileQuery(text)
		if query != "" {
			hits := SearchFiles(query, category)
			if len(hits) > 0 {
				best := hits[0]
				if err := shellOpen(best.Path); err == nil {
					verb := map[string]string{"audio": "Playing", "video": "Playing", "image": "Showing"}[category]
					if verb == "" {
						verb = "Opening"
					}
					extra := ""
					if len(hits) > 1 {
						extra = fmt.Sprintf(" (%d other matches)", len(hits)-1)
					}
					return Result{fmt.Sprintf("%s %s.%s", verb, best.Name, extra), fmt.Sprintf("Opened file: %s", best.Name)}
				}
			}
			// no file matched -> try a named folder from the index
			if dirHits := SearchFolders(query); len(dirHits) > 0 {
				best := dirHits[0]
				if err := shellOpen(best.Path); err == nil {
					return Result{fmt.Sprintf("Opening the %s folder.", best.Name), fmt.Sprintf("Opened folder: %s", best.Name)}
				}
			}
		}
		// category with no specific match -> open the media folder itself
		if category != "" {
			if dir, ok := folderTarget(text); ok {
				if err := shellOpen(dir); err == nil {
					return Result{fmt.Sprintf("Opening your %s folder.", filepath.Base(dir)), "Opened media folder"}
				}
			}
		}
		if dir, ok := folderTarget(text); ok {
			if err := shellOpen(dir); err == nil {
				return Result{fmt.Sprintf("Opening the %s folder.", filepath.Base(dir)), "Opened folder"}
			}
		}
	}

	// ---- filesystem listing: "what's in downloads", "read folder projects" ----
	if listQuery := extractListTarget(text); listQuery != "" || strings.Contains(text, "rescan") || strings.Contains(text, "reindex") {
		if strings.Contains(text, "rescan") || strings.Contains(text, "reindex") {
			RequestRescan()
			files, dirs := IndexStats()
			return Result{
				fmt.Sprintf("Rescanning now. I am indexing %d folders holding %d files across your profile.", dirs, files),
				"Filesystem rescan requested",
			}
		}
		if r, ok := listFolderResult(listQuery); ok {
			return r
		}
	}

	// ---- suggestions for near misses ----
	if matched && score >= 0.68 {
		setPending(targetKind(kind, key), key)
		return Result{pick2(suggestions, titleCase(key)), fmt.Sprintf("Fuzzy suggestion: %s", key)}
	}
	if hasTrigger && matched && score >= 0.55 {
		setPending(targetKind(kind, key), key)
		return Result{pick2(suggestions, titleCase(key)), fmt.Sprintf("Fuzzy suggestion: %s", key)}
	}

	// ---- help ----
	if containsAny(text, "help", "commands", "can you do", "who are you") {
		return capabilitiesResult()
	}
	if strings.Contains(text, "focus mode") || strings.Contains(text, "do not disturb") {
		return Result{"Focus protocol engaged. Go build something great.", "Focus mode engaged"}
	}
	if strings.Contains(text, "thank") {
		return Result{pick([]string{"Always, Commander.", "At your service.", "Anytime."}), "Thanks acknowledged"}
	}

	return Result{pick(fallbacks), "Freeform request logged"}
}

func targetKind(kind, key string) string {
	if kind == "" {
		if _, ok := Apps[key]; ok {
			return "app"
		}
		return "site"
	}
	return kind
}

// executeTarget opens a pending/suggested target by kind.
func executeTarget(kind, key string) Result {
	display := titleCase(key)
	switch kind {
	case "app":
		if err := launchAppNamed(key, Apps[key]); err == nil {
			return Result{pick2(confirms, display), fmt.Sprintf("Launched app: %s", key)}
		}
		return Result{fmt.Sprintf("I could not launch %s on this machine.", display), "App launch failed"}
	case "file":
		if err := shellOpen(key); err == nil {
			return Result{fmt.Sprintf("Opening %s.", filepath.Base(key)), fmt.Sprintf("Opened file: %s", filepath.Base(key))}
		}
		return Result{"That file is gone or locked.", "File open failed"}
	default: // site
		if url, ok := Sites[key]; ok {
			if err := OpenURL(url); err == nil {
				return Result{pick2(confirms, display), fmt.Sprintf("Opened website: %s", key)}
			}
		}
	}
	return Result{"That target vanished from my index.", "Open failed"}
}

// matchTarget wraps FindTarget and reports whether anything plausible matched.
func matchTarget(text string) (kind, key string, score float64, matched bool) {
	m := FindTarget(text)
	if m.Canonical == "" {
		return "", "", m.Score, false
	}
	return kindOf(m), m.Canonical, m.Score, true
}

func kindOf(m Match) string {
	if m.IsApp {
		return "app"
	}
	return "site"
}

func weatherResult(text string) Result {
	city := ""
	if m := reCity.FindStringSubmatch(text); m != nil {
		city = strings.TrimSpace(m[1])
	}
	w, err := GetWeather(city)
	if err != nil {
		return Result{"The weather service did not answer. Check the connection and try again.", "Weather lookup failed"}
	}
	return Result{
		fmt.Sprintf("%s: %s, %d degrees celsius, feels like %d. Humidity %d percent, wind %d kilometers per hour.",
			w.Place, w.Desc, w.TempC, w.FeelsC, w.Humid, w.Wind),
		fmt.Sprintf("Weather pulled: %s", w.Place),
	}
}

func statusResult() Result {
	up := time.Since(startedAt)
	return Result{
		fmt.Sprintf("All systems nominal. Uptime %d hours %d minutes %d seconds, %d cores, "+
			"whisper local speech engine, %d open targets indexed.",
			int(up.Hours()), int(up.Minutes())%60, int(up.Seconds())%60, runtime.NumCPU(), TargetCount()),
		"Status report generated",
	}
}

func capabilitiesResult() Result {
	return Result{
		fmt.Sprintf("I run entirely on this machine. I can open over %d websites and %d apps - even if you pronounce them "+
			"badly - play any video, song, or photo on this computer by name, open any folder in your profile and read its "+
			"contents out loud, pull live weather for any city, tell time and date, do math by voice, take and read notes, "+
			"set timers, run system reports, and close any running app by name - gracefully, or with force if it refuses. "+
			"Try: open youtube. What's in downloads? Close notepad. Or: weather in Lagos.",
			len(Sites), len(Apps)),
		"Capabilities reported",
	}
}

func readNotesResult() Result {
	data, err := os.ReadFile(notesPath())
	if err == nil {
		lines := nonEmptyLines(string(data))
		if len(lines) > 0 {
			return Result{fmt.Sprintf("Latest note: %s", lines[len(lines)-1]), "Notes recalled"}
		}
	}
	return Result{"Your notebook is empty.", "No notes found"}
}

// listFolderResult reads a directory and speaks its contents.
// query "~" means the whole indexed profile overview.
func listFolderResult(query string) (Result, bool) {
	home, _ := osUserHome()

	if query == "~" {
		files, dirs := IndexStats()
		return Result{
			fmt.Sprintf("Across your Videos, Music, Pictures, Downloads, Desktop, and Documents: %d folders holding %d files, all indexed and openable by name.", dirs, files),
			"Filesystem overview reported",
		}, true
	}

	var path, display string
	if dir, ok := folderTarget(query); ok {
		path, display = dir, filepath.Base(dir)
	} else if hits := SearchFolders(query); len(hits) > 0 {
		path, display = hits[0].Path, hits[0].Name
	} else {
		return Result{}, false
	}

	dirs, files, err := ListDir(path)
	if err != nil {
		return Result{fmt.Sprintf("I could not read the %s folder.", display), "Directory read failed"}, true
	}
	total := len(dirs) + len(files)
	if total == 0 {
		return Result{fmt.Sprintf("The %s folder is completely empty.", display), fmt.Sprintf("Listed folder: %s (empty)", display)}, true
	}

	const cap = 8
	var parts []string
	for _, d := range dirs {
		if len(parts) >= cap {
			break
		}
		parts = append(parts, d+" folder")
	}
	for _, f := range files {
		if len(parts) >= cap {
			break
		}
		parts = append(parts, f)
	}
	listing := strings.Join(parts, ", ")
	suffix := ""
	if total > len(parts) {
		suffix = fmt.Sprintf(", and %d more", total-len(parts))
	}
	reply := fmt.Sprintf("%s has %d items: %s%s.", display, total, listing, suffix)
	if home != "" && !isHomeTopLevel(path, home) {
		rel, _ := filepath.Rel(home, filepath.Dir(path))
		reply = fmt.Sprintf("%s inside %s has %d items: %s%s.", display, rel, total, listing, suffix)
	}
	return Result{reply, fmt.Sprintf("Listed folder: %s (%d items)", display, total)}, true
}

func isHomeTopLevel(path, home string) bool {
	return filepath.Dir(path) == home
}

// pick2 formats a template pool with one argument while avoiding recent repeats.
func pick2(pool []string, arg string) string {
	templated := make([]string, len(pool))
	for i, p := range pool {
		templated[i] = fmt.Sprintf(p, arg)
	}
	return pick(templated)
}

// ---- helpers -------------------------------------------------------------------

func sanitize(raw string) string {
	lower := strings.ToLower(raw)
	var b strings.Builder
	for _, r := range lower {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '\'' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(s, n) {
			return true
		}
	}
	return false
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func isAffirmative(text string) bool {
	words := strings.Fields(text)
	if len(words) > 3 {
		return false
	}
	for _, w := range words {
		if contains([]string{"yes", "yeah", "yep", "yup", "sure", "ok", "okay", "do", "it", "go"}, w) {
			return true
		}
		if Similarity(w, "yes") >= 0.75 {
			return true
		}
	}
	return containsAny(text, "open it", "do it", "go ahead", "open that")
}

func isNegative(text string) bool {
	words := strings.Fields(text)
	if len(words) > 3 {
		return false
	}
	for _, w := range words {
		if contains([]string{"no", "nope", "cancel", "nevermind", "forget", "nah"}, w) {
			return true
		}
	}
	return false
}
