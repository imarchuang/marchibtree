package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/marchi/marchibtree/btree"
)

func main() {
	addr := flag.String("addr", envOr("MARCHIBTREE_ADDR", ":7100"), "listen address")
	dataDir := flag.String("dataDir", envOr("MARCHIBTREE_DATA", "./data"), "data directory (heap.db + meta)")
	pageSize := flag.Int("pageSize", btree.DefaultPageSize, "page size in bytes")
	syncWAL := flag.Bool("sync", true, "fsync WAL on commit")
	flag.Parse()

	tree, err := btree.OpenSync(*dataDir, *pageSize, *syncWAL)
	if err != nil {
		log.Fatalf("open %s: %v", *dataDir, err)
	}
	defer tree.Close()
	log.Printf("marchibtree listening on %s dataDir=%s pageSize=%d", *addr, *dataDir, *pageSize)
	log.Fatal(http.ListenAndServe(*addr, newServer(tree)))
}

func newServer(tree *btree.Tree) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"backend":  "pages",
			"pages":    tree.PageCount(),
			"root":     tree.Root(),
			"height":   tree.Height(),
			"walLSN":   tree.LSN(),
			"pageSize": tree.PageSize(),
			"free":     tree.FreeCount(),
		})
	})
	mux.HandleFunc("GET /kv", func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")
		end := r.URL.Query().Get("end")
		kvs := tree.Range(start, end)
		type row struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		out := make([]row, len(kvs))
		for i, kv := range kvs {
			out[i] = row{Key: kv.Key, Value: string(kv.Value)}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("PUT /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := tree.Put(key, body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("DELETE /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		ok, err := tree.Delete(r.PathValue("key"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		v, ok := tree.Get(r.PathValue("key"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(v)
	})
	mux.HandleFunc("POST /internal/checkpoint", func(w http.ResponseWriter, _ *http.Request) {
		if err := tree.Checkpoint(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /debug/tree", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, tree.Dump())
	})
	return mux
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
