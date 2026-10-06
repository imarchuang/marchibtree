# marchibtree

Educational page-oriented **B-tree KV** in Go (DDIA ch.3 contrast with the LSM
family: [marchilogs](https://github.com/imarchuang/marchilogs),
[marchimetrics](https://github.com/imarchuang/marchimetrics)).

**Not Postgres / not InnoDB.** Fixed-size 4 KiB pages, in-place updates, WAL redo,
point get and range scan. No SQL, no MVCC.

See [PLAN.md](PLAN.md) for slices.

## Slice 0

In-memory HTTP map. Persistence, pages, and WAL land in later slices.

```bash
go test ./...
go run ./cmd/marchibtree -addr=:7100
curl -X PUT localhost:7100/kv/a -d 1
curl localhost:7100/kv/a
curl localhost:7100/healthz
```

**Docker:**

```bash
docker build -t marchibtree:local .
docker run --rm -p 7100:7100 marchibtree:local
```

Flags: `-addr=:7100`, `-dataDir`, `-pageSize=4096`, `-sync=true`.
