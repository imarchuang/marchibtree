package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marchi/marchibtree/btree"
)

func TestHealthzAndKV(t *testing.T) {
	tree := btree.New(btree.DefaultPageSize)
	srv := httptest.NewServer(newServer(tree))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz status=%d body=%s", res.StatusCode, body)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Fatalf("healthz body=%s", body)
	}

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/kv/hello", strings.NewReader("world"))
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("put status=%d", res.StatusCode)
	}

	res, err = http.Get(srv.URL + "/kv/hello")
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(got) != "world" {
		t.Fatalf("get status=%d body=%q", res.StatusCode, got)
	}

	res, err = http.Get(srv.URL + "/kv/missing")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status=%d", res.StatusCode)
	}
}

func TestDebugTreeAfterSplits(t *testing.T) {
	tree := btree.New(btree.DefaultPageSize)
	srv := httptest.NewServer(newServer(tree))
	defer srv.Close()

	for i := 0; i < 400; i++ {
		k := fmt.Sprintf("k%06d", i)
		req, err := http.NewRequest(http.MethodPut, srv.URL+"/kv/"+k, strings.NewReader("v"))
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNoContent {
			t.Fatalf("put %s status=%d", k, res.StatusCode)
		}
	}
	res, err := http.Get(srv.URL + "/debug/tree")
	if err != nil {
		t.Fatal(err)
	}
	dump, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("debug status=%d", res.StatusCode)
	}
	s := string(dump)
	if !strings.Contains(s, "internal") {
		t.Fatalf("expected split (internal node) in dump:\n%s", s)
	}
	if tree.Height() < 2 {
		t.Fatalf("height=%d", tree.Height())
	}
}
