package btree

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type metaFile struct {
	RootPageID PageID `json:"rootPageID"`
	PageSize   int    `json:"pageSize"`
	NextPageID PageID `json:"nextPageID"`
	Height     int    `json:"height"`
}

type checkpointFile struct {
	LSN uint64 `json:"lsn"`
}

func Open(dir string, pageSize int) (*Tree, error) {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "wal"), 0o755); err != nil {
		return nil, err
	}
	metaPath := filepath.Join(dir, "meta.json")
	heapPath := filepath.Join(dir, "heap.db")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t := New(pageSize)
		t.dir = dir
		f, err := os.OpenFile(heapPath, os.O_RDWR|os.O_CREATE, 0o644)
		if err != nil {
			return nil, err
		}
		t.heap = f
		return t, nil
	}
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var meta metaFile
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	if meta.PageSize != 0 && meta.PageSize != pageSize {
		pageSize = meta.PageSize
	}
	f, err := os.OpenFile(heapPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	t := &Tree{
		pageSize: pageSize,
		pages:    make(map[PageID]*page),
		root:     meta.RootPageID,
		nextID:   meta.NextPageID,
		height:   meta.Height,
		dir:      dir,
		heap:     f,
		dirty:    make(map[PageID]struct{}),
	}
	if t.height < 1 {
		t.height = 1
	}
	if t.nextID == 0 {
		t.nextID = 1
	}
	buf := make([]byte, pageSize)
	for id := PageID(0); id < t.nextID; id++ {
		off := int64(id) * int64(pageSize)
		n, err := f.ReadAt(buf, off)
		if err != nil && err != io.EOF {
			_ = f.Close()
			return nil, fmt.Errorf("read page %d: %w", id, err)
		}
		if n < pageSize {
			continue
		}
		p, err := decodePage(buf)
		if err != nil {
			continue
		}
		t.pages[p.id] = p
	}
	if _, ok := t.pages[t.root]; !ok {
		_ = f.Close()
		return nil, fmt.Errorf("root page %d missing from heap", t.root)
	}
	if cp, err := os.ReadFile(filepath.Join(dir, "checkpoint.json")); err == nil {
		var c checkpointFile
		if json.Unmarshal(cp, &c) == nil {
			t.lsn = c.LSN
		}
	}
	return t, nil
}

func (t *Tree) Checkpoint() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.checkpointLocked()
}

func (t *Tree) checkpointLocked() error {
	if t.dir == "" || t.heap == nil {
		return nil
	}
	for id := range t.dirty {
		p, ok := t.pages[id]
		if !ok {
			continue
		}
		buf, err := p.encode(t.pageSize)
		if err != nil {
			return err
		}
		off := int64(id) * int64(t.pageSize)
		if _, err := t.heap.WriteAt(buf, off); err != nil {
			return err
		}
	}
	if err := t.heap.Sync(); err != nil {
		return err
	}
	meta := metaFile{
		RootPageID: t.root,
		PageSize:   t.pageSize,
		NextPageID: t.nextID,
		Height:     t.height,
	}
	if err := writeJSONAtomic(filepath.Join(t.dir, "meta.json"), meta); err != nil {
		return err
	}
	if err := writeJSONAtomic(filepath.Join(t.dir, "checkpoint.json"), checkpointFile{LSN: t.lsn}); err != nil {
		return err
	}
	t.dirty = make(map[PageID]struct{})
	return nil
}

func (t *Tree) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.heap != nil {
		_ = t.heap.Close()
		t.heap = nil
	}
	return nil
}

func writeJSONAtomic(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
