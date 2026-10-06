package btree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWALRecoversPutWithoutCheckpoint(t *testing.T) {
	dir := t.TempDir()
	tr, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Put("crash", []byte("recovered")); err != nil {
		t.Fatal(err)
	}
	if tr.LSN() == 0 {
		t.Fatal("expected WAL LSN after put")
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "meta.json")); !os.IsNotExist(err) {
		t.Fatalf("checkpoint should not have run, meta err=%v", err)
	}

	tr2, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	v, ok := tr2.Get("crash")
	if !ok || string(v) != "recovered" {
		t.Fatalf("wal redo got %q ok=%v", v, ok)
	}
}

func TestWALRepairsTruncatedHeap(t *testing.T) {
	dir := t.TempDir()
	tr, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Put("keep", []byte("me")); err != nil {
		t.Fatal(err)
	}
	if err := tr.Checkpoint(); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	heap := filepath.Join(dir, "heap.db")
	if err := os.WriteFile(heap, make([]byte, DefaultPageSize), 0o644); err != nil {
		t.Fatal(err)
	}

	tr2, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	v, ok := tr2.Get("keep")
	if !ok || string(v) != "me" {
		t.Fatalf("truncated heap repair got %q ok=%v", v, ok)
	}
}
