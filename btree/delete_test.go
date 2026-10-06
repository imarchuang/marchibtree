package btree

import (
	"fmt"
	"testing"
)

func TestDeleteThenGet404(t *testing.T) {
	tr := New(DefaultPageSize)
	if err := tr.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	ok, err := tr.Delete("a")
	if err != nil || !ok {
		t.Fatalf("delete ok=%v err=%v", ok, err)
	}
	if _, hit := tr.Get("a"); hit {
		t.Fatal("expected miss after delete")
	}
	ok, err = tr.Delete("a")
	if err != nil || ok {
		t.Fatalf("second delete ok=%v err=%v", ok, err)
	}
}

func TestFreelistReusesPage(t *testing.T) {
	tr := New(DefaultPageSize)
	n := 0
	for tr.Height() < 2 {
		n++
		if err := tr.Put(fmt.Sprintf("k%06d", n), []byte("v")); err != nil {
			t.Fatal(err)
		}
	}
	high := tr.NextPageID()
	pagesBefore := tr.PageCount()
	for i := 1; i <= n; i++ {
		if _, err := tr.Delete(fmt.Sprintf("k%06d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if tr.FreeCount() == 0 {
		t.Fatalf("expected freelist after emptying leaves, pages=%d->%d height=%d", pagesBefore, tr.PageCount(), tr.Height())
	}
	for i := 1; i <= n; i++ {
		if err := tr.Put(fmt.Sprintf("z%06d", i), []byte("v")); err != nil {
			t.Fatal(err)
		}
	}
	if tr.NextPageID() > high {
		t.Fatalf("nextID grew %d -> %d; expected reuse from freelist (free=%d)", high, tr.NextPageID(), tr.FreeCount())
	}
}

func TestDeleteWALReplay(t *testing.T) {
	dir := t.TempDir()
	tr, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	if err := tr.Put("gone", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Delete("gone"); err != nil {
		t.Fatal(err)
	}
	if err := tr.Close(); err != nil {
		t.Fatal(err)
	}
	tr2, err := Open(dir, DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer tr2.Close()
	if _, ok := tr2.Get("gone"); ok {
		t.Fatal("deleted key survived WAL replay")
	}
}
