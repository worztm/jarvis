package brain

import (
	"strings"
	"sync"
)

// Fuzzy matching over every known target. Sliding windows of up to 4 words
// from the transcript compete against all surface forms (site names, aliases,
// app names) using Levenshtein similarity.

type surface struct {
	text      string
	canonical string
	isApp     bool
}

var (
	surfacesOnce sync.Once
	allSurfaces  []surface
)

func buildSurfaces() {
	for name := range Sites {
		allSurfaces = append(allSurfaces, surface{text: name, canonical: name})
	}
	for alias, canon := range Aliases {
		if _, ok := Sites[canon]; ok {
			allSurfaces = append(allSurfaces, surface{text: alias, canonical: canon})
		}
	}
	for name := range Apps {
		allSurfaces = append(allSurfaces, surface{text: name, canonical: name, isApp: true})
	}
}

// Similarity = 1 - normalized edit distance, in [0,1].
func Similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	ra, rb := []rune(a), []rune(b)
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := 0; j <= len(rb); j++ {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			m := prev[j] + 1 // deletion
			if v := cur[j-1] + 1; v < m {
				m = v
			}
			if v := prev[j-1] + cost; v < m {
				m = v
			}
			cur[j] = m
		}
		prev, cur = cur, prev
	}
	return 1 - float64(prev[len(rb)])/float64(max(len(ra), len(rb)))
}

// Match is the best fuzzy hit in a transcript.
type Match struct {
	Canonical string
	IsApp     bool
	Score     float64
}

var openTriggers = map[string]bool{
	"open": true, "launch": true, "start": true, "go": true, "visit": true,
	"show": true, "pull": true, "bring": true, "run": true,
	"play": true, "watch": true, "stream": true, "listen": true,
}

// FindTarget scans sliding word windows for the closest known target.
func FindTarget(text string) Match {
	surfacesOnce.Do(buildSurfaces)
	words := strings.Fields(text)

	var best Match
	for i := range words {
		maxJ := min(i+4, len(words))
		for j := i + 1; j <= maxJ; j++ {
			phrase := strings.Join(words[i:j], " ")
			for _, s := range allSurfaces {
				score := Similarity(phrase, s.text)
				if score > best.Score {
					best = Match{Canonical: s.canonical, IsApp: s.isApp, Score: score}
				}
			}
		}
	}
	return best
}

// HasOpenTrigger reports whether the transcript contains an opening verb.
func HasOpenTrigger(text string) bool {
	for w := range strings.SplitSeq(text, " ") {
		if openTriggers[w] {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
