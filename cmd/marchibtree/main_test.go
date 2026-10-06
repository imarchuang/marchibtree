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

func TestHTTPCheckpointReopen(t *testing.T) {
	dir := t.TempDir()
	tree, err := btree.Open(dir, btree.DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(newServer(tree))
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/kv/persist", strings.NewReader("yes"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("put status=%d", res.StatusCode)
	}
	res, err = http.Post(srv.URL+"/internal/checkpoint", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("checkpoint status=%d", res.StatusCode)
	}
	srv.Close()
	tree.Close()

	tree2, err := btree.Open(dir, btree.DefaultPageSize)
	if err != nil {
		t.Fatal(err)
	}
	defer tree2.Close()
	v, ok := tree2.Get("persist")
	if !ok || string(v) != "yes" {
		t.Fatalf("reopen got %q ok=%v", v, ok)
	}
}

func TestHTTPDelete(t *testing.T) {
	tree := btree.New(btree.DefaultPageSize)
	srv := httptest.NewServer(newServer(tree))
	defer srv.Close()
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/kv/x", strings.NewReader("1"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	req, err = http.NewRequest(http.MethodDelete, srv.URL+"/kv/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d", res.StatusCode)
	}
	res, err = http.Get(srv.URL + "/kv/x")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete status=%d", res.StatusCode)
	}
}

func TestHTTPRange(t *testing.T) {
	tree := btree.New(btree.DefaultPageSize)
	srv := httptest.NewServer(newServer(tree))
	defer srv.Close()
	for i := 0; i < 20; i++ {
		k := fmt.Sprintf("k%02d", i)
		req, err := http.NewRequest(http.MethodPut, srv.URL+"/kv/"+k, strings.NewReader("v"))
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	res, err := http.Get(srv.URL + "/kv?start=k05&end=k08")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("range status=%d body=%s", res.StatusCode, body)
	}
	s := string(body)
	if !strings.Contains(s, `"key":"k05"`) || !strings.Contains(s, `"key":"k07"`) {
		t.Fatalf("range body=%s", s)
	}
	if strings.Contains(s, `"key":"k08"`) || strings.Contains(s, `"key":"k04"`) {
		t.Fatalf("end exclusive / start inclusive failed: %s", s)
	}
}
