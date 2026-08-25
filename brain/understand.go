package brain

// Hallucination-tolerant understanding: fuzzy trigger verbs, intent
// prototypes, and query cleanup. The goal: even badly mangled speech
// ("wht tym iz it", "opena yutab", "paly some music") routes somewhere sane.

import "strings"

// fuzzyHasWord reports whether any token in text is within threshold
// similarity of target.
func fuzzyHasWord(text, target string, threshold float64) bool {
	for _, tok := range strings.Fields(text) {
		if Similarity(tok, target) >= threshold {
			return true
		}
	}
	return false
}

// hasOpenTriggerFuzzy catches "open/launch/play/..." even when misheard.
func hasOpenTriggerFuzzy(text string) bool {
	for trigger := range openTriggers {
		if fuzzyHasWord(text, trigger, 0.78) {
			return true
		}
	}
	return false
}

// intentPrototypes: fuzzy phrases per intent. The best-scoring window wins.
var intentPrototypes = []struct {
	intent     string
	threshold  float64
	prototypes []string
}{
	{"time", 0.66, []string{"what time is it", "current time", "time check", "tell me the time", "clock"}},
	{"date", 0.68, []string{"what is the date", "what day is it", "todays date", "whats today"}},
	{"weather", 0.66, []string{"what is the weather", "weather report", "weather forecast", "temperature outside", "how hot is it", "how cold is it", "is it raining"}},
	{"status", 0.68, []string{"system status", "status report", "run a system scan", "system diagnostics", "systems check", "health check"}},
	{"help", 0.68, []string{"what can you do", "help me", "show your commands", "capabilities", "who are you"}},
	{"notes-read", 0.70, []string{"read my notes", "read notes", "my notes", "what are my notes"}},
	{"focus", 0.72, []string{"focus mode", "do not disturb"}},
	{"thanks", 0.72, []string{"thank you", "many thanks"}},
	{"greeting", 0.72, []string{"hello jarvis", "hey jarvis", "good morning", "good evening", "good afternoon"}},
	{"stop", 0.72, []string{"be quiet", "shut up", "stop talking", "stop speaking", "cancel that"}},
}

// bestIntent returns the closest intent prototype for the transcript.
func bestIntent(text string) (string, float64) {
	words := strings.Fields(text)
	bestIntent, bestScore := "", 0.0

	scorePhrase := func(phrase string) float64 {
		pWords := strings.Fields(phrase)
		best := 0.0
		for i := range words {
			maxJ := min(i+len(pWords)+1, len(words))
			for j := i + 1; j <= maxJ && j-i <= len(pWords)+1; j++ {
				if s := Similarity(strings.Join(words[i:j], " "), phrase); s > best {
					best = s
				}
			}
		}
		return best
	}

	for _, proto := range intentPrototypes {
		for _, phrase := range proto.prototypes {
			if s := scorePhrase(phrase); s >= proto.threshold && s > bestScore {
				bestIntent, bestScore = proto.intent, s
			}
		}
	}
	return bestIntent, bestScore
}

// fillerWords are stripped when extracting a filename query from speech.
var fillerWords = map[string]bool{
	"open": true, "launch": true, "start": true, "run": true, "play": true,
	"watch": true, "stream": true, "listen": true, "go": true, "visit": true,
	"show": true, "pull": true, "bring": true, "the": true, "a": true, "an": true,
	"my": true, "some": true, "please": true, "jarvis": true, "up": true,
	"video": true, "song": true, "music": true, "photo": true, "picture": true,
	"movie": true, "audio": true, "track": true, "clip": true, "file": true,
	"for": true, "me": true, "on": true,
}

// extractFileQuery strips trigger/filler words, leaving the likely filename.
func extractFileQuery(text string) string {
	var keep []string
	for _, w := range strings.Fields(text) {
		if !fillerWords[w] {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}

// mediaCategoryFor detects "play some music" style category requests.
func mediaCategoryFor(text string) (category, folderHint string) {
	switch {
	case fuzzyHasWord(text, "song", 0.8) || fuzzyHasWord(text, "music", 0.8) || fuzzyHasWord(text, "audio", 0.8):
		return "audio", "Music"
	case fuzzyHasWord(text, "video", 0.8) || fuzzyHasWord(text, "movie", 0.8) || fuzzyHasWord(text, "clip", 0.8):
		return "video", "Videos"
	case fuzzyHasWord(text, "photo", 0.8) || fuzzyHasWord(text, "picture", 0.8) || fuzzyHasWord(text, "image", 0.8):
		return "image", "Pictures"
	}
	return "", ""
}

// folderTargets maps spoken folder names to their paths (opened in Explorer).
func folderTarget(text string) (string, bool) {
	home, err := osUserHome()
	if err != nil {
		return "", false
	}
	pairs := []struct {
		word string
		dir  string
	}{
		{"music", "Music"}, {"videos", "Videos"}, {"video", "Videos"},
		{"movies", "Videos"}, {"pictures", "Pictures"}, {"photos", "Pictures"},
		{"downloads", "Downloads"}, {"documents", "Documents"}, {"desktop", "Desktop"},
	}
	for _, p := range pairs {
		if fuzzyHasWord(text, p.word, 0.85) {
			return home + "\\" + p.dir, true
		}
	}
	return "", false
}

// listPhrases: spoken patterns that mean "tell me what is inside X".
var listStrip = map[string]bool{
	"folder": true, "folders": true, "directory": true, "contents": true,
	"content": true, "items": true, "files": true, "file": true, "inside": true,
	"in": true, "of": true, "the": true, "my": true, "whats": true, "what": true,
	"is": true, "list": true, "show": true, "read": true, "me": true, "see": true,
	"how": true, "many": true, "count": true, "much": true, "open": true, "jarvis": true,
	"please": true, "name": true, "names": true, "on": true, "computer": true, "pc": true,
}

// extractListTarget detects a "what's in downloads / read folder projects /
// list files on my desktop" style request and returns the folder query.
// Returns "" when the command is not a listing request.
func extractListTarget(text string) string {
	hit := func(phrases ...string) bool {
		for _, p := range phrases {
			if strings.Contains(text, p) {
				return true
			}
		}
		return false
	}
	isListRequest := hit(
		"whats in", "what is in", "whats inside", "what is inside",
		"list files", "list folders", "list items", "folder contents",
		"contents of", "read folder", "open folder", "show folder",
		"how many files", "how many folders", "how many items",
	)
	// bare known-folder ask: "read downloads", "show me my documents"
	if !isListRequest {
		for _, w := range []string{"downloads", "documents", "desktop", "videos", "music", "pictures", "photos"} {
			if fuzzyHasWord(text, w, 0.85) && hit("read", "show", "scan") {
				isListRequest = true
				break
			}
		}
	}
	if !isListRequest {
		return ""
	}
	// special: whole-computer overview
	if hit("my computer", "the computer", "my pc", "whole pc") || (hit("computer") && hit("files", "folders")) {
		return "~"
	}
	var keep []string
	for _, w := range strings.Fields(text) {
		if !listStrip[w] {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " ")
}
