package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileIndexer maintains a pre-built index of all filenames under a root
// directory, skipping dot files/folders. Rescans periodically in the background.
type FileIndexer struct {
	roots    []string
	logger   *slog.Logger
	interval time.Duration

	mu    sync.RWMutex
	paths []string // all indexed paths (with root prefix for disambiguation)
}

func NewFileIndexer(roots []string, interval time.Duration, logger *slog.Logger) *FileIndexer {
	return &FileIndexer{
		roots:    roots,
		logger:   logger,
		interval: interval,
	}
}

// Run starts the background indexing loop. Blocks until ctx is cancelled.
func (fi *FileIndexer) Run(ctx context.Context) {
	fi.scan()
	ticker := time.NewTicker(fi.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi.scan()
		}
	}
}

func (fi *FileIndexer) scan() {
	start := time.Now()
	var paths []string

	for _, root := range fi.roots {
		filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			name := d.Name()

			if strings.HasPrefix(name, ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if d.IsDir() {
				switch name {
				case "node_modules", "__pycache__", "venv", ".venv", "dist", "build", "target":
					return filepath.SkipDir
				}
				return nil
			}

			// Store path relative to root
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}
			paths = append(paths, rel)
			return nil
		})
	}

	fi.mu.Lock()
	fi.paths = paths
	fi.mu.Unlock()

	fi.logger.Info("file index updated", "roots", fi.roots, "files", len(paths), "duration", time.Since(start).Round(time.Millisecond))
}

// Search returns paths matching the query (case-insensitive substring match
// on each query term). Results are sorted by relevance: exact filename matches
// first, then shorter paths, then alphabetical.
func (fi *FileIndexer) Search(query string, limit int) []string {
	if query == "" || limit <= 0 {
		return nil
	}

	fi.mu.RLock()
	paths := fi.paths
	fi.mu.RUnlock()

	queryLower := strings.ToLower(query)
	terms := strings.Fields(queryLower)

	type scored struct {
		path  string
		score int
	}

	var matches []scored
	for _, p := range paths {
		pLower := strings.ToLower(p)

		// All terms must match
		allMatch := true
		for _, term := range terms {
			if !strings.Contains(pLower, term) {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}

		// Score: lower is better
		score := len(p) // prefer shorter paths
		base := strings.ToLower(filepath.Base(p))
		if strings.Contains(base, queryLower) {
			score -= 1000 // strong boost for filename match
		}
		if base == queryLower {
			score -= 2000 // exact filename match
		}

		matches = append(matches, scored{path: p, score: score})
	}

	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return matches[i].path < matches[j].path
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.path
	}
	return result
}

// Count returns the number of indexed files.
func (fi *FileIndexer) Count() int {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return len(fi.paths)
}
