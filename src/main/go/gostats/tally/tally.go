// Package tally keeps per-player counts. It knows nothing about the game, so it is safe to use from any
// goroutine and can be tested with plain go test.
package tally

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// Entry is one row of the scoreboard.
type Entry struct {
	Player string `json:"player"`
	Blocks int64  `json:"blocks"`
}

// Tally counts blocks per player.
type Tally struct {
	mu     sync.Mutex
	counts map[string]int64
	dirty  bool
}

// New returns an empty Tally.
func New() *Tally { return &Tally{counts: map[string]int64{}} }

// Add records one block broken by player and returns their new total.
func (t *Tally) Add(player string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts[player]++
	t.dirty = true
	return t.counts[player]
}

// Count returns player's total.
func (t *Tally) Count(player string) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts[player]
}

// Top returns the n highest totals, highest first, with ties in name order. n <= 0 returns everyone.
func (t *Tally) Top(n int) []Entry {
	t.mu.Lock()
	entries := make([]Entry, 0, len(t.counts))
	for player, blocks := range t.counts {
		entries = append(entries, Entry{player, blocks})
	}
	t.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Blocks != entries[j].Blocks {
			return entries[i].Blocks > entries[j].Blocks
		}
		return entries[i].Player < entries[j].Player
	})

	if n > 0 && len(entries) > n {
		entries = entries[:n]
	}
	return entries
}

// Save writes the counts to path if they changed since the last successful Save. The rename means a crash
// mid-write leaves the previous file intact.
func (t *Tally) Save(path string) error {
	t.mu.Lock()
	if !t.dirty {
		t.mu.Unlock()
		return nil
	}
	data, err := json.MarshalIndent(t.counts, "", "  ")
	t.dirty = false
	t.mu.Unlock()

	if err == nil {
		err = writeFile(path, data)
	}
	if err != nil {
		// These counts never reached the disk, so the next Save has to try again.
		t.mu.Lock()
		t.dirty = true
		t.mu.Unlock()
	}
	return err
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load replaces the counts with those in path. A missing file means a fresh start.
func (t *Tally) Load(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	counts := map[string]int64{}
	if err := json.Unmarshal(data, &counts); err != nil {
		return err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts = counts
	t.dirty = false
	return nil
}
