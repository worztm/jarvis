package brain

import (
	"strings"
	"testing"
)

func TestFuzzyOpenTargets(t *testing.T) {
	cases := []struct {
		said string
		want string
	}{
		{"open youtab", "youtube"},
		{"you hab", "youtube"},
		{"opan netflicks", "netflix"},
		{"launch git hub", "github"},
		{"play spotify", "spotify"},
		{"open watsap", "whatsapp"},
		{"go to g mail", "gmail"},
	}
	for _, c := range cases {
		m := FindTarget(c.said)
		if m.Canonical != c.want || m.Score < 0.62 {
			t.Errorf("said %q -> got %s (%.2f), want %s", c.said, m.Canonical, m.Score, c.want)
		}
	}
}

func TestThinkRoutes(t *testing.T) {
	if r := Think("what time is it"); r.Action != "Time reported" {
		t.Errorf("time route failed: %+v", r)
	}
	if r := Think("run a system scan"); r.Action != "Status report generated" {
		t.Errorf("status route failed: %+v", r)
	}
	if r := Think("what is 12 times 9"); r.Reply != "That comes to 108." {
		t.Errorf("math failed: %+v", r)
	}
	w := Think("weather in lagos")
	if !strings.HasPrefix(w.Action, "Weather pulled") && w.Action != "Weather lookup failed" {
		t.Errorf("weather unexpected: %+v", w)
	}
}

func TestFallbackVariety(t *testing.T) {
	replies := map[string]bool{}
	for i := 0; i < 5; i++ {
		r := pick(fallbacks)
		replies[r] = true
	}
	if len(replies) < 3 {
		t.Errorf("fallback not varied enough: %d unique in 5 tries", len(replies))
	}
}

func TestSuperUnderstanding(t *testing.T) {
	// mangled time request routes to time intent
	if intent, _ := bestIntent(sanitize("wht tym iz it")); intent != "time" {
		t.Errorf("mangled time not understood: %q", intent)
	}
	if intent, _ := bestIntent(sanitize("wheather report")); intent != "weather" {
		t.Errorf("mangled weather not understood: %q", intent)
	}
	if !isAffirmative(sanitize("yes")) || !isAffirmative(sanitize("yeah open it")) {
		t.Error("affirmatives not detected")
	}
	if !isNegative(sanitize("nope")) {
		t.Error("negative not detected")
	}
	if q := extractFileQuery("open the amazing video please"); q != "amazing" {
		t.Errorf("file query extraction wrong: %q", q)
	}
	// fuzzy trigger: misheard "open"
	if !hasOpenTriggerFuzzy(sanitize("opena youtube")) {
		t.Error("fuzzy open trigger missed")
	}
}

func TestFileSearch(t *testing.T) {
	// read-only: just ensure it doesn't crash and returns sane types
	hits := SearchFiles("nothingmatchesxyz123", "")
	if len(hits) > 0 {
		t.Errorf("unexpected matches for gibberish: %+v", hits)
	}
}
