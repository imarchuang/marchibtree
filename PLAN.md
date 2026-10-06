# marchibtree — page-oriented B-tree KV MVP

Educational B-tree engine in Go. Completes DDIA Ch3 vs the LSM family
([marchilogs](../marchilogs), [marchimetrics](../marchimetrics),
[marchindex](../marchindex) segments).

**Not Postgres / not InnoDB / not BoltDB clone.** We borrow **fixed-size
pages, node split, WAL + redo, point + range scan** — not SQL, MVCC
(that is [marchisql](../marchisql)), or buffer-pool replacement science.

---

## Learning goal

1. Why B-trees **update in place**: a write hits a leaf page, may split,
   may dirty a parent — random IO, predictable point-get.
2. **Page** as the unit of cache and recovery (4 KiB).
3. **WAL first:** log the change, then write the page (or checkpoint later).
   Crash → replay WAL → tree consistent.
4. Write amplification: one logical PUT can dirty leaf + internal + WAL.
5. Honest contrast with LSM: no memtable/compaction; reads don’t fan out
   across files; range scan is sibling pointers (or parent walk).

**Pass bar:** after a crash mid-PUT, reopen and show either the old or new
value, never a torn page; explain a split in 3 diagrams (before / WAL / after).

---

## Concepts we keep (and drop)

| B-tree / PG | marchibtree v0 | Deferred |
|---|---|---|
| Page size | 4096 bytes, header + slots | overflow pages |
| Node | keys + child/page or key+value | variable length prefix compression |
| Fill factor | split at overflow | concurrent SMO (latch crabbing) |
| WAL | JSONL or binary records, fsync on commit | full ARIES (undo, CLR) |
| Buffer pool | mmap or map[pageID]*page, flush dirty | clock sweep, pin counts |
| Freelist | reuse deleted pages | VACUUM / defrag |
| Concurrency | single writer mutex in v0 | latches, WAL concurrent flush |

**Non-goals:** SQL, secondary indexes, MVCC (compose with marchisql later),
compression, checksums beyond a simple crc.

---

## Core loop

```text
PUT k=v
  lock
  WAL append {put, k, v, page?}   # at least the logical redo
  find leaf (root → children)
  insert slot; if full: split leaf, insert separator in parent
  mark pages dirty
  fsync WAL
  unlock
  (pages flushed lazily or on checkpoint)

GET k
  walk root → leaf, binary search slots
```

---

## On-disk layout

```text
{dataDir}/
  meta.json          # rootPageID, pageSize, nextPageID
  heap.db            # concatenated 4KiB pages, pageID = offset/4096
  wal/
    000001.log       # redo records
  checkpoint.json    # last durable LSN
```

Page header (v0):

```text
pageID | type(leaf|internal) | nKeys | prev | next | crc
keys[] | values[] or childPageIDs[]
```

---

## API

| Method | Path | Purpose |
|---|---|---|
| GET | `/healthz` | pages, root, wal LSN |
| PUT | `/kv/{key}` | body = value |
| GET | `/kv/{key}` | 200 or 404 |
| GET | `/kv?start=&end=` | range scan |
| POST | `/internal/checkpoint` | flush dirty pages, advance checkpoint |
| GET | `/debug/tree` | indent dump of keys per page (small trees) |

Flags: `-addr=:7100`, `-dataDir`, `-pageSize=4096`, `-sync=true`.

---

## MVP slices

### Slice 0 — skeleton
HTTP + in-memory map. Prove tests/docker.

### Slice 1 — pages + in-memory tree
Allocate pages, split, GET/PUT, `/debug/tree`. Still no WAL.
Test: insert 1..N until height ≥ 2; range scan ordered.

### Slice 2 — persist heap.db
Load pages from file; checkpoint writes all dirty pages.
Test: PUT, checkpoint, reopen.

### Slice 3 — WAL + crash
Every PUT appends redo before returning; reopen replays after checkpoint.
Test: write WAL then kill before page flush; reopen recovers value.
Sabotage: truncate heap page → WAL still repairs.

### Slice 4 — delete + freelist
Delete key, merge or steal optional (v0 can underflow until later).
Test: delete then GET 404; page reused.

### Slice 5 — polish
Range query HTTP, `BTREE.md` vs LSM table, docker demo.

---

## Demo (graduation)

```bash
go run ./cmd/marchibtree -dataDir=./data
curl -X PUT localhost:7100/kv/a -d 1
# kill -9; restart
curl localhost:7100/kv/a          # 1
curl 'localhost:7100/debug/tree'  # see splits
```

---

## LSM vs this (keep in BTREE.md)

| | LSM (marchilogs / metrics) | marchibtree |
|---|---|---|
| Write | append file, later compact | in-place page + WAL |
| Point get | bloom / several files | O(height) pages |
| Range | merge iterators | leaf linked / in-order walk |
| Space reclaim | compaction | delete slots + optional merge |
| Crash | part rename / WAL tip | redo WAL |

Start at **slice 0** on `feat/skeleton`.
