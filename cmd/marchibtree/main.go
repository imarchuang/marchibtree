package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/marchi/marchibtree/store"
)

func main() {
	addr := flag.String("addr", envOr("MARCHIBTREE_ADDR", ":7100"), "listen address")
	dataDir := flag.String("dataDir", envOr("MARCHIBTREE_DATA", "./data"), "data directory (unused in slice 0)")
	pageSize := flag.Int("pageSize", 4096, "page size in bytes")
	syncWAL := flag.Bool("sync", true, "fsync WAL on commit (unused in slice 0)")
	flag.Parse()
	_ = dataDir
	_ = syncWAL

	kv := store.NewMem()
	srv := newServer(kv, *pageSize)
	log.Printf("marchibtree listening on %s (in-memory)", *addr)
	log.Fatal(http.ListenAndServe(*addr, srv))
}

func newServer(kv *store.Mem, pageSize int) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"backend": "memory",
			"pages":   0,
			"root":    0,
			"walLSN":  0,
			"keys":    kv.Len(),
			"pageSize": pageSize,
		})
	})
	mux.HandleFunc("PUT /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			http.Error(w, "empty key", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		kv.Put(key, body)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /kv/{key}", func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		v, ok := kv.Get(key)
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(v)
	})
	return mux
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
