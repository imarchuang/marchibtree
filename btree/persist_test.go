package btree

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPutCheckpointReopen(t *testing.T) {
	dir := t.TempDir()
	tr, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := tr.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "heap.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "meta.json")); err != nil {
		t.Fatal(err)
	}

	tr2, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	v, ok := tr2.Get("a")
	if !ok || string(v) != "1" {
		t.Fatalf("reopen got %q ok=%v", v, ok)
	}
}

func TestCheckpointAfterSplitReopen(t *testing.T) {
	dir := t.TempDir()
	tr, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for tr.Height() < 2 {
		n++
		k := fmt.Sprintf("k%06d", n)
		if err := tr.Put(k, []byte("v")); err != nil {
			t.Fatal(err)
		}
	}
	if err := tr.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	wantH := tr.Height()
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	tr2, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	if tr2.Height() != wantH {
		t.Fatalf("height %d want %d", tr2.Height(), wantH)
	}
	for i := 1; i <= n; i++ {
		k := fmt.Sprintf("k%06d", i)
		if _, ok := tr2.Get(k); !ok {
			t.Fatalf("missing %s after reopen", k)
		}
	}
}
