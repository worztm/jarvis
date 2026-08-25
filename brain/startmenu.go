package brain

// Start Menu shortcut index: makes ANY installed Windows app launchable by
// fuzzy name (WhatsApp, Telegram, Brave, Spotify, games...) by resolving its
// .lnk shortcut, which ShellExecute executes natively.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	smMu      sync.Mutex
	smIndex   map[string]string // "telegram" -> C:\...\Telegram.lnk
	smIndexed time.Time
)

func startMenuDirs() []string {
	var dirs []string
	if pd := os.Getenv("ProgramData"); pd != "" {
		dirs = append(dirs, filepath.Join(pd, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	if ad := os.Getenv("APPDATA"); ad != "" {
		dirs = append(dirs, filepath.Join(ad, "Microsoft", "Windows", "Start Menu", "Programs"))
	}
	return dirs
}

// startMenuApps scans (max once per 10 min) Start Menu .lnk shortcuts.
func startMenuApps() map[string]string {
	smMu.Lock()
	defer smMu.Unlock()
	if smIndex != nil && time.Since(smIndexed) < 10*time.Minute {
		return smIndex
	}
	index := make(map[string]string)
	for _, dir := range startMenuDirs() {
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				rel, _ := filepath.Rel(dir, path)
				if strings.Count(rel, string(filepath.Separator)) >= 3 {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".lnk") {
				return nil
			}
			name := strings.ToLower(strings.TrimSuffix(filepath.Base(path), ".lnk"))
			if _, exists := index[name]; !exists {
				index[name] = path
			}
			return nil
		})
	}
	if len(index) > 0 {
		smIndex = index
		smIndexed = time.Now()
	}
	return smIndex
}

// findStartMenuApp fuzzy-matches an app name against indexed shortcuts.
func findStartMenuApp(name string) (string, bool) {
	index := startMenuApps()
	if len(index) == 0 {
		return "", false
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return "", false
	}
	bestPath, bestScore := "", 0.0
	for key, path := range index {
		score := Similarity(name, key)
		// token containment: "brave browser" vs "brave"
		for _, kt := range strings.Fields(key) {
			for _, qt := range strings.Fields(name) {
				if len(qt) >= 4 && strings.Contains(kt, qt) {
					s := 0.72 + 0.2*(float64(len(qt))/float64(len(kt)))
					if s > score {
						score = s
					}
				}
			}
		}
		if score > bestScore {
			bestPath, bestScore = path, score
		}
	}
	if bestScore >= 0.62 {
		return bestPath, true
	}
	return "", false
}

// launchAppNamed tries, in order: explicit candidates (paths/protocols/PATH),
// Start Menu shortcuts, then a raw ShellExecute of the name.
func launchAppNamed(key string, candidates []string) error {
	for _, cand := range candidates {
		if err := shellOpen(cand); err == nil {
			return nil
		}
	}
	if lnk, ok := findStartMenuApp(key); ok {
		if err := shellOpen(lnk); err == nil {
			return nil
		}
	}
	if err := shellOpen(key); err == nil {
		return nil
	}
	return fmt.Errorf("no launcher resolved for %q", key)
}
