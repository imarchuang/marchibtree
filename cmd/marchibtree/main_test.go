package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/marchi/marchibtree/store"
)

func TestHealthzAndKV(t *testing.T) {
	kv := store.NewMem()
	srv := httptest.NewServer(newServer(kv, 4096))
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
