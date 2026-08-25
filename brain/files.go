package brain

// File AND folder search across the user's profile so voice can open local
// videos, music, photos, documents, and any folder by (fuzzy) name, and read
// out directory contents.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileHit struct {
	Path  string
	Name  string
	Score float64
}

type FolderHit struct {
	Path  string
	Name  string
	Score float64
}

var (
	fsMu         sync.Mutex
	fileIndex    []string
	dirIndex     []string
	fsIndexed    time.Time
	fsIndexing   bool
	fsRescanFlag bool
)

// mediaDirs returns the root folders worth indexing.
func mediaDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dirs := []string{"Videos", "Music", "Pictures", "Downloads", "Desktop", "Documents"}
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		p := filepath.Join(home, d)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

// RequestRescan forces the next index access to rebuild.
func RequestRescan() {
	fsMu.Lock()
	fsRescanFlag = true
	fsMu.Unlock()
}

// IndexStats reports how much is indexed.
func IndexStats() (files, dirs int) {
	ensureIndex()
	fsMu.Lock()
	defer fsMu.Unlock()
	return len(fileIndex), len(dirIndex)
}

// ensureIndex walks media folders (depth-limited) at most once per 3 minutes,
// collecting both files and directories.
func ensureIndex() {
	fsMu.Lock()
	rescan := fsRescanFlag
	fsRescanFlag = false
	fresh := fileIndex != nil && time.Since(fsIndexed) < 3*time.Minute && !rescan
	indexing := fsIndexing
	haveFiles := fileIndex != nil
	haveDirs := dirIndex != nil
	fsMu.Unlock()

	if fresh || indexing || (haveFiles && haveDirs && !rescan) {
		return
	}

	fsMu.Lock()
	if fsIndexing { // another goroutine got there first
		fsMu.Unlock()
		return
	}
	fsIndexing = true
	fsMu.Unlock()

	go func() {
		var files, dirs []string
		for _, root := range mediaDirs() {
			dirs = append(dirs, root)
			filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					name := strings.ToLower(d.Name())
					if name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || name == "appdata" {
						return filepath.SkipDir
					}
					rel, _ := filepath.Rel(root, path)
					if strings.Count(rel, string(filepath.Separator)) >= 4 {
						return filepath.SkipDir
					}
					if path != root && len(dirs) < 4000 {
						dirs = append(dirs, path)
					}
					return nil
				}
				if len(files) < 8000 {
					files = append(files, path)
				}
				return nil
			})
		}
		fsMu.Lock()
		fileIndex = files
		dirIndex = dirs
		fsIndexed = time.Now()
		fsIndexing = false
		fsMu.Unlock()
	}()

	// wait briefly so a brand-new session can still answer on first ask
	for i := 0; i < 40; i++ {
		time.Sleep(100 * time.Millisecond)
		fsMu.Lock()
		done := !fsIndexing && fileIndex != nil
		fsMu.Unlock()
		if done {
			break
		}
	}
}

var mediaExtCategories = map[string][]string{
	"audio": {".mp3", ".wav", ".flac", ".m4a", ".ogg", ".aac", ".wma"},
	"video": {".mp4", ".mkv", ".avi", ".mov", ".webm", ".wmv", ".m4v"},
	"image": {".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".heic"},
	"doc":   {".pdf", ".docx", ".doc", ".txt", ".pptx", ".xlsx"},
}

func extCategory(ext string) string {
	ext = strings.ToLower(ext)
	for cat, exts := range mediaExtCategories {
		for _, e := range exts {
			if e == ext {
				return cat
			}
		}
	}
	return ""
}

func scoreName(query, name string, qTokens []string) float64 {
	score := Similarity(query, name)
	for _, qt := range qTokens {
		if len(qt) < 3 {
			continue
		}
		if strings.Contains(name, qt) {
			s := 0.55 + 0.4*(float64(len(qt))/float64(len(name)))
			if s > score {
				score = s
			}
		} else {
			for _, nt := range strings.Fields(name) {
				if sim := Similarity(qt, nt); sim > 0.8 {
					s := 0.5 + 0.35*(sim-0.8)*5
					if s > score {
						score = s
					}
				}
			}
		}
	}
	return score
}

func topHits(hits []FileHit, n int) []FileHit {
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > n {
		hits = hits[:n]
	}
	return hits
}

// SearchFiles fuzzy-matches a spoken query against indexed filenames.
func SearchFiles(query, wantCategory string) []FileHit {
	query = sanitize(query)
	if query == "" {
		return nil
	}
	ensureIndex()
	fsMu.Lock()
	files := fileIndex
	fsMu.Unlock()
	if files == nil {
		return nil
	}
	qTokens := strings.Fields(query)
	var hits []FileHit

	for _, path := range files {
		name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		lname := strings.ToLower(name)
		ext := filepath.Ext(path)
		if wantCategory != "" && extCategory(ext) != wantCategory {
			continue
		}
		if score := scoreName(query, lname, qTokens); score >= 0.52 {
			hits = append(hits, FileHit{Path: path, Name: filepath.Base(path), Score: score})
		}
	}
	return topHits(hits, 5)
}

// SearchFolders fuzzy-matches a spoken query against indexed folder names.
func SearchFolders(query string) []FileHit {
	query = sanitize(query)
	if query == "" {
		return nil
	}
	ensureIndex()
	fsMu.Lock()
	dirs := dirIndex
	fsMu.Unlock()
	if dirs == nil {
		return nil
	}
	qTokens := strings.Fields(query)
	var hits []FileHit
	for _, path := range dirs {
		lname := strings.ToLower(filepath.Base(path))
		if score := scoreName(query, lname, qTokens); score >= 0.55 {
			hits = append(hits, FileHit{Path: path, Name: filepath.Base(path), Score: score})
		}
	}
	return topHits(hits, 5)
}

// ListDir returns the immediate children of a folder, directories first,
// then files, each alphabetically.
func ListDir(path string) (dirs, files []string, err error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
		} else {
			files = append(files, name)
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	return dirs, files, nil
}
