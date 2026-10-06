package btree

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type walRec struct {
	LSN   uint64 `json:"lsn"`
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

func walPath(dir string) string {
	return filepath.Join(dir, "wal", "000001.log")
}

func (t *Tree) openWAL() error {
	if t.dir == "" {
		return nil
	}
	f, err := os.OpenFile(walPath(t.dir), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	t.wal = f
	return nil
}

func (t *Tree) walAppend(op, key string, value []byte) error {
	if t.wal == nil {
		return nil
	}
	t.lsn++
	rec := walRec{LSN: t.lsn, Op: op, Key: key}
	if value != nil {
		rec.Value = base64.StdEncoding.EncodeToString(value)
	}
	b, err := json.Marshal(rec)
	if err != nil {
		t.lsn--
		return err
	}
	if _, err := t.wal.Write(append(b, '\n')); err != nil {
		return err
	}
	if t.syncWAL {
		return t.wal.Sync()
	}
	return nil
}

func (t *Tree) replayWAL(afterLSN uint64) error {
	if t.dir == "" {
		return nil
	}
	f, err := os.Open(walPath(t.dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec walRec
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("wal decode: %w", err)
		}
		if rec.LSN <= afterLSN {
			continue
		}
		if err := t.applyWAL(rec); err != nil {
			return err
		}
		if rec.LSN > t.lsn {
			t.lsn = rec.LSN
		}
	}
	return sc.Err()
}

func (t *Tree) applyWAL(rec walRec) error {
	switch rec.Op {
	case "put":
		v, err := base64.StdEncoding.DecodeString(rec.Value)
		if err != nil {
			return fmt.Errorf("wal value: %w", err)
		}
		return t.putLocked([]byte(rec.Key), v)
	default:
		return fmt.Errorf("unknown wal op %q", rec.Op)
	}
}
