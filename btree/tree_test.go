package btree

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestPutGetOverwrite(t *testing.T) {
	tr := New(DefaultPageSize)
	if _, ok := tr.Get("a"); ok {
		t.Fatal("expected miss")
	}
	if err := tr.Put("a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	v, ok := tr.Get("a")
	if !ok || string(v) != "1" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
	if err := tr.Put("a", []byte("2")); err != nil {
		t.Fatal(err)
	}
	v, ok = tr.Get("a")
	if !ok || string(v) != "2" {
		t.Fatalf("overwrite %q", v)
	}
}

func TestInsertUntilHeightAtLeast2(t *testing.T) {
	tr := New(DefaultPageSize)
	n := 0
	for tr.Height() < 2 {
		n++
		k := fmt.Sprintf("k%06d", n)
		if err := tr.Put(k, []byte("v"+k)); err != nil {
			t.Fatal(err)
		}
		if n > 10_000 {
			t.Fatalf("height still %d after %d inserts", tr.Height(), n)
		}
	}
	if tr.PageCount() < 3 {
		t.Fatalf("expected split pages, got %d height=%d", tr.PageCount(), tr.Height())
	}
	for i := 1; i <= n; i++ {
		k := fmt.Sprintf("k%06d", i)
		v, ok := tr.Get(k)
		if !ok || string(v) != "v"+k {
			t.Fatalf("missing %s after height=%d n=%d", k, tr.Height(), n)
		}
	}
	dump := tr.Dump()
	if dump == "" {
		t.Fatal("empty dump")
	}
}

func TestRangeOrderedAfterShuffledInsert(t *testing.T) {
	tr := New(DefaultPageSize)
	n := 400
	perm := rand.Perm(n)
	for _, i := range perm {
		k := fmt.Sprintf("k%06d", i)
		if err := tr.Put(k, []byte{byte(i % 256)}); err != nil {
			t.Fatal(err)
		}
	}
	got := tr.Range("", "")
	if len(got) != n {
		t.Fatalf("range len=%d want %d", len(got), n)
	}
	for i, kv := range got {
		want := fmt.Sprintf("k%06d", i)
		if kv.Key != want {
			t.Fatalf("index %d key=%s want %s", i, kv.Key, want)
		}
	}
	mid := tr.Range("k000100", "k000110")
	if len(mid) != 10 {
		t.Fatalf("mid range len=%d keys=%v", len(mid), keysOf(mid))
	}
	if mid[0].Key != "k000100" || mid[9].Key != "k000109" {
		t.Fatalf("mid=%v", keysOf(mid))
	}
}

func TestPageEncodeDecodeRoundTrip(t *testing.T) {
	tr := New(DefaultPageSize)
	for i := 0; i < 50; i++ {
		if err := tr.Put(fmt.Sprintf("k%02d", i), []byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for id, p := range tr.pages {
		buf, err := p.encode(tr.pageSize)
		if err != nil {
			t.Fatalf("encode %d: %v", id, err)
		}
		if len(buf) != DefaultPageSize {
			t.Fatalf("encoded len=%d", len(buf))
		}
		got, err := decodePage(buf)
		if err != nil {
			t.Fatalf("decode %d: %v", id, err)
		}
		if got.id != p.id || got.kind != p.kind || len(got.keys) != len(p.keys) {
			t.Fatalf("page %d meta mismatch", id)
		}
		for i := range p.keys {
			if string(got.keys[i]) != string(p.keys[i]) {
				t.Fatalf("key mismatch page %d i=%d", id, i)
			}
		}
	}
}

func keysOf(kvs []KV) []string {
	s := make([]string, len(kvs))
	for i, kv := range kvs {
		s[i] = kv.Key
	}
	return s
}
