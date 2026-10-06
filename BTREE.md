# B-tree vs LSM (marchibtree)

marchibtree is a page-oriented B-tree KV: a write updates a leaf in place,
may split, and may dirty a parent. Recovery is **WAL first, pages later**.

Contrast with the LSM family in this repo group
([marchilogs](https://github.com/imarchuang/marchilogs),
[marchimetrics](https://github.com/imarchuang/marchimetrics),
[marchindex](https://github.com/imarchuang/marchindex)):

| | LSM (marchilogs / metrics) | marchibtree |
|---|---|---|
| Write | append file, later compact | in-place page + WAL |
| Point get | bloom / several files | O(height) pages |
| Range | merge iterators | leaf linked / in-order walk |
| Space reclaim | compaction | delete slots + optional merge |
| Crash | part rename / WAL tip | redo WAL |

## Why pages

The unit of cache and recovery is **4096 bytes**. A PUT that overflows a leaf
splits the page and inserts a separator in the parent — write amplification
is leaf + parent + WAL, not a new SSTable.

## WAL then page

```
PUT k=v
  lock
  WAL append redo
  find leaf, insert / split
  mark dirty
  fsync WAL
  unlock
  (heap.db flushed on checkpoint)
```

After a crash mid-PUT, reopen replays `wal/000001.log` after `checkpoint.json`'s
LSN. You get the old or new value, not a torn page (CRC on each page).

## Split in three pictures

1. **Before** — one leaf holds keys `[a b c d]` and is about to overflow.
2. **WAL** — the insert is durable as a JSONL redo record; heap may still be
   the old leaf.
3. **After** — left `[a b]`, right `[c d]` plus new key, parent separator `c`,
   sibling `next`/`prev` links for range scan.

`GET /debug/tree` prints that layout on a live process.
