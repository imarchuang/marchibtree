# marchibtree

Educational page-oriented **B-tree KV** in Go (DDIA ch.3 contrast with the LSM
family: [marchilogs](https://github.com/imarchuang/marchilogs),
[marchimetrics](https://github.com/imarchuang/marchimetrics)).

**Not Postgres / not InnoDB.** Fixed-size 4 KiB pages, in-place updates, WAL redo,
point get and range scan. No SQL, no MVCC.

See [PLAN.md](PLAN.md) for slices and [BTREE.md](BTREE.md) for LSM vs B-tree.

## Quick start

```bash
go test ./...
go run ./cmd/marchibtree -addr=:7100 -dataDir=./data
curl -X PUT localhost:7100/kv/a -d 1
curl localhost:7100/kv/a
curl 'localhost:7100/kv?start=a&end=z'
curl localhost:7100/healthz
curl localhost:7100/debug/tree
```

Crash demo: kill `-9` and run the same command again — WAL redo restores `a=1`
without a checkpoint.

**Docker:**

```bash
docker compose up --build
# listens on http://localhost:7100
curl -X PUT localhost:7100/kv/a -d 1
curl localhost:7100/kv/a
```

Flags: `-addr=:7100`, `-dataDir`, `-pageSize=4096`, `-sync=true`.

## API

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | pages, root, wal LSN |
| PUT | `/kv/{key}` | body = value |
| GET | `/kv/{key}` | 200 or 404 |
| DELETE | `/kv/{key}` | 204 or 404 |
| GET | `/kv?start=&end=` | range scan (`end` exclusive) |
| POST | `/internal/checkpoint` | flush dirty pages |
| GET | `/debug/tree` | indent dump of keys per page |
