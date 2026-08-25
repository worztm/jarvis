package brain

// App closing: "close notepad", "quit spotify", "kill telegram".
// Graceful close first (lets apps prompt to save), forced kill as fallback.

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var closeTriggers = map[string]bool{
	"close": true, "quit": true, "exit": true, "kill": true,
	"terminate": true, "end": true,
}

func hasCloseTrigger(text string) bool {
	for t := range closeTriggers {
		if fuzzyHasWord(text, t, 0.8) {
			return true
		}
	}
	return false
}

// closeStrip: words removed when extracting which app to close.
var closeStrip = map[string]bool{
	"close": true, "quit": true, "exit": true, "kill": true,
	"terminate": true, "end": true, "the": true, "a": true, "an": true,
	"app": true, "application": true, "please": true, "jarvis": true,
	"now": true, "it": true, "down": true,
}

func extractCloseQuery(text string) string {
	var keep []string
	for _, w := range strings.Fields(text) {
		if !closeStrip[w] {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}

// closeOverrides maps app keys to their real process image names where the
// guessable ones are wrong.
var closeOverrides = map[string][]string{
	"whatsapp":  {"WhatsApp.exe", "WhatsApp.Root.exe"},
	"telegram":  {"Telegram.exe"},
	"brave":     {"brave.exe"},
	"chrome":    {"chrome.exe"},
	"edge":      {"msedge.exe"},
	"spotify desktop": {"Spotify.exe"},
}

// closeCandidates derives process image names for an app key.
func closeCandidates(key string) []string {
	var names []string
	add := func(n string) {
		n = strings.ToLower(n)
		for _, existing := range names {
			if existing == n {
				return
			}
		}
		names = append(names, n)
	}
	for _, n := range closeOverrides[key] {
		add(n)
	}
	for _, cand := range Apps[key] {
		base := filepath.Base(cand)
		if strings.HasSuffix(base, ".exe") && !strings.Contains(base, "%") {
			add(base)
		}
	}
	collapsed := strings.ReplaceAll(key, " ", "") + ".exe"
	add(collapsed)
	return names
}

// processRunning checks the Windows task list for an image name.
func processRunning(image string) bool {
	cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image)
	cmd.SysProcAttr = hideWindowAttr()
	out, err := cmd.Output()
	return err == nil && strings.Contains(strings.ToLower(string(out)), strings.ToLower(image))
}

// CloseApp gracefully closes the first running candidate, forcing if needed.
// Returns the closed image name and success.
func CloseApp(key string) (string, bool) {
	for _, name := range closeCandidates(key) {
		if !processRunning(name) {
			continue
		}
		// graceful: posts WM_CLOSE so apps can save
		_ = startHidden(exec.Command("taskkill", "/IM", name))
		for i := 0; i < 6; i++ {
			time.Sleep(400 * time.Millisecond)
			if !processRunning(name) {
				return name, true
			}
		}
		// stubborn -> force the whole tree down
		_ = startHidden(exec.Command("taskkill", "/IM", name, "/T", "/F"))
		time.Sleep(600 * time.Millisecond)
		return name, !processRunning(name)
	}
	return "", false
}

// closeResult builds the full response for a close command.
func closeResult(text string) Result {
	query := extractCloseQuery(text)
	if query == "" || query == "everything" || query == "all" {
		return Result{"Name the app and I will end it. I do not do scorched earth.", "Close request missing target"}
	}

	// refuse to nuke the shell
	for _, guarded := range []string{"explorer", "file explorer", "files"} {
		if Similarity(query, guarded) >= 0.8 {
			return Result{"Closing Explorer takes your entire desktop with it, Commander. Request denied - with respect.", "Explorer close refused"}
		}
	}

	m := FindTarget(query)
	if m.Canonical != "" && !m.IsApp {
		// "close spotify" -> the site matched, but look for an installed app
		// with a similar name before refusing.
		for appKey := range Apps {
			score := Similarity(query, appKey)
			for _, qt := range strings.Fields(appKey) {
				for _, w := range strings.Fields(query) {
					if len(w) >= 4 && strings.Contains(qt, w) {
						s := 0.75 + 0.2*(float64(len(w))/float64(len(qt)))
						if s > score {
							score = s
						}
					}
				}
			}
			if score >= 0.62 {
				m = Match{Canonical: appKey, IsApp: true, Score: score}
				break
			}
		}
	}
	switch {
	case m.Canonical == "":
		return Result{fmt.Sprintf("I don't have %s in my app index.", titleCase(query)), "Close target unknown"}
	case !m.IsApp:
		return Result{"That one is a website, not an app - nothing to close. Want me to stop talking instead?", "Cannot close a website"}
	}

	name, ok := CloseApp(m.Canonical)
	display := titleCase(m.Canonical)
	if !ok {
		if name == "" {
			return Result{fmt.Sprintf("%s is not even running.", display), fmt.Sprintf("Close skipped: %s not running", display)}
		}
		return Result{fmt.Sprintf("%s refused to die. It may still be running.", display), fmt.Sprintf("Force close failed: %s", display)}
	}
	return Result{pick2(closeReplies, display), fmt.Sprintf("Closed app: %s (%s)", m.Canonical, name)}
}

var closeReplies = []string{
	"Closing %s.",
	"%s terminated.",
	"Done. %s is closed.",
	"%s has been shut down cleanly.",
}
