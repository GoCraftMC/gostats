package tally

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestTopOrdersByCountThenName(t *testing.T) {
	tally := New()
	for _, player := range []string{"bob", "amy", "cid", "cid", "amy", "cid"} {
		tally.Add(player)
	}

	top := tally.Top(0)
	want := []Entry{{"cid", 3}, {"amy", 2}, {"bob", 1}}
	if len(top) != len(want) {
		t.Fatalf("got %v", top)
	}
	for i := range want {
		if top[i] != want[i] {
			t.Errorf("row %d: got %v, want %v", i, top[i], want[i])
		}
	}

	if limited := tally.Top(2); len(limited) != 2 {
		t.Errorf("Top(2) returned %d rows", len(limited))
	}
}

func TestConcurrentAdds(t *testing.T) {
	tally := New()
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for range 1000 {
				tally.Add("amy")
			}
		})
	}
	wg.Wait()

	if got := tally.Count("amy"); got != 32000 {
		t.Errorf("got %d", got)
	}
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "stats.json")
	first := New()
	first.Add("amy")
	first.Add("amy")
	if err := first.Save(path); err != nil {
		t.Fatal(err)
	}

	second := New()
	if err := second.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := second.Count("amy"); got != 2 {
		t.Errorf("got %d", got)
	}
}

func TestSaveSkipsUnchangedData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	tally := New()
	if err := tally.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("an unchanged tally wrote a file: %v", err)
	}
}

func TestFailedSaveIsRetried(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tally := New()
	tally.Add("amy")
	if err := tally.Save(filepath.Join(blocker, "stats.json")); err == nil {
		t.Fatal("saving under a file should fail")
	}

	good := filepath.Join(dir, "stats.json")
	if err := tally.Save(good); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(good); err != nil {
		t.Errorf("the retry wrote nothing: %v", err)
	}
}

func TestLoadMissingFileIsAFreshStart(t *testing.T) {
	tally := New()
	if err := tally.Load(filepath.Join(t.TempDir(), "none.json")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadCorruptFileFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := New().Load(path); err == nil {
		t.Error("corrupt data loaded without an error")
	}
}
