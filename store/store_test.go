package store

import "testing"

func TestMemPutGet(t *testing.T) {
	m := NewMem()
	if _, ok := m.Get("missing"); ok {
		t.Fatal("expected miss")
	}
	m.Put("a", []byte("1"))
	v, ok := m.Get("a")
	if !ok || string(v) != "1" {
		t.Fatalf("got %q ok=%v", v, ok)
	}
	m.Put("a", []byte("2"))
	v, ok = m.Get("a")
	if !ok || string(v) != "2" {
		t.Fatalf("overwrite got %q", v)
	}
	if m.Len() != 1 {
		t.Fatalf("len=%d", m.Len())
	}
}
