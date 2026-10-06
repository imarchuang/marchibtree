package btree

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"sync"
)

type KV struct {
	Key   string
	Value []byte
}

type Tree struct {
	mu       sync.Mutex
	pageSize int
	pages    map[PageID]*page
	root     PageID
	nextID   PageID
	height   int
	dir      string
	heap     *os.File
	wal      *os.File
	syncWAL  bool
	dirty    map[PageID]struct{}
	lsn      uint64
}

func New(pageSize int) *Tree {
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	t := &Tree{
		pageSize: pageSize,
		pages:    make(map[PageID]*page),
		root:     0,
		nextID:   1,
		height:   1,
		dirty:    make(map[PageID]struct{}),
	}
	t.pages[0] = newLeaf(0)
	t.markDirty(0)
	return t
}

func (t *Tree) markDirty(id PageID) {
	if t.dirty == nil {
		t.dirty = make(map[PageID]struct{})
	}
	t.dirty[id] = struct{}{}
}

func (t *Tree) LSN() uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lsn
}

func (t *Tree) PageSize() int { return t.pageSize }
func (t *Tree) Root() PageID  { return t.root }
func (t *Tree) Height() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.height
}
func (t *Tree) PageCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pages)
}

func (t *Tree) Put(key string, value []byte) error {
	if key == "" {
		return fmt.Errorf("empty key")
	}
	k := []byte(key)
	v := cloneBytes(value)
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.walAppend("put", key, v); err != nil {
		return err
	}
	return t.putLocked(k, v)
}

func (t *Tree) Get(key string) ([]byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	leaf := t.findLeaf([]byte(key))
	i, ok := leaf.search([]byte(key))
	if !ok {
		return nil, false
	}
	return cloneBytes(leaf.vals[i]), true
}

func (t *Tree) Range(start, end string) []KV {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []KV
	sb := []byte(start)
	eb := []byte(end)
	p := t.leftmostLeaf()
	for p != nil {
		for i, k := range p.keys {
			if start != "" && bytes.Compare(k, sb) < 0 {
				continue
			}
			if end != "" && bytes.Compare(k, eb) >= 0 {
				return out
			}
			out = append(out, KV{Key: string(k), Value: cloneBytes(p.vals[i])})
		}
		if p.next == 0 {
			break
		}
		p = t.pages[p.next]
	}
	return out
}

func (t *Tree) Dump() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var b strings.Builder
	fmt.Fprintf(&b, "root=%d height=%d pages=%d pageSize=%d\n", t.root, t.height, len(t.pages), t.pageSize)
	t.dumpPage(&b, t.root, 0)
	return b.String()
}

func (t *Tree) dumpPage(b *strings.Builder, id PageID, depth int) {
	p := t.pages[id]
	if p == nil {
		fmt.Fprintf(b, "%spage %d <missing>\n", indent(depth), id)
		return
	}
	kind := "leaf"
	if p.kind == KindInternal {
		kind = "internal"
	}
	keys := make([]string, len(p.keys))
	for i, k := range p.keys {
		keys[i] = string(k)
	}
	fmt.Fprintf(b, "%spage %d %s n=%d prev=%d next=%d keys=%v\n",
		indent(depth), p.id, kind, len(p.keys), p.prev, p.next, keys)
	if p.kind == KindInternal {
		for _, kid := range p.kids {
			t.dumpPage(b, kid, depth+1)
		}
	}
}

func indent(n int) string { return strings.Repeat("  ", n) }

func (t *Tree) putLocked(k, v []byte) error {
	path := t.findPath(k)
	leaf := t.pages[path[len(path)-1]]
	if i, ok := leaf.search(k); ok {
		old := leaf.vals[i]
		leaf.vals[i] = v
		t.markDirty(leaf.id)
		if leaf.fits(t.pageSize) {
			return nil
		}
		if len(leaf.keys) == 1 {
			leaf.vals[i] = old
			return fmt.Errorf("key+value too large for page")
		}
		return t.splitUp(path)
	}
	leaf.insertLeaf(k, v)
	t.markDirty(leaf.id)
	if leaf.fits(t.pageSize) {
		return nil
	}
	if len(leaf.keys) == 1 {
		leaf.keys = nil
		leaf.vals = nil
		return fmt.Errorf("key+value too large for page")
	}
	return t.splitUp(path)
}

func (t *Tree) splitUp(path []PageID) error {
	for {
		id := path[len(path)-1]
		p := t.pages[id]
		for !p.fits(t.pageSize) {
			sep, right, err := t.splitPage(p)
			if err != nil {
				return err
			}
			t.markDirty(p.id)
			t.markDirty(right.id)
			if len(path) == 1 {
				newRoot := t.allocInternal()
				newRoot.extra = p.id
				newRoot.keys = [][]byte{sep}
				newRoot.kids = []PageID{p.id, right.id}
				t.markDirty(newRoot.id)
				t.root = newRoot.id
				t.height++
				path = []PageID{newRoot.id, p.id}
				continue
			}
			parent := t.pages[path[len(path)-2]]
			parent.insertInternal(sep, right.id)
			t.markDirty(parent.id)
		}
		if len(path) == 1 {
			return nil
		}
		path = path[:len(path)-1]
	}
}

func (t *Tree) splitPage(p *page) (sep []byte, right *page, err error) {
	n := len(p.keys)
	if n < 2 {
		return nil, nil, fmt.Errorf("cannot split page %d with %d keys", p.id, n)
	}
	mid := n / 2
	if p.kind == KindLeaf {
		right = t.allocLeaf()
		right.keys = cloneKeySlice(p.keys[mid:])
		right.vals = cloneKeySlice(p.vals[mid:])
		p.keys = p.keys[:mid]
		p.vals = p.vals[:mid]
		right.prev = p.id
		right.next = p.next
		if p.next != 0 {
			t.pages[p.next].prev = right.id
			t.markDirty(p.next)
		}
		p.next = right.id
		t.markDirty(p.id)
		t.markDirty(right.id)
		sep = cloneBytes(right.keys[0])
		return sep, right, nil
	}
	right = t.allocInternal()
	sep = cloneBytes(p.keys[mid])
	right.keys = cloneKeySlice(p.keys[mid+1:])
	right.kids = append([]PageID(nil), p.kids[mid+1:]...)
	right.extra = right.kids[0]
	p.keys = p.keys[:mid]
	p.kids = p.kids[:mid+1]
	return sep, right, nil
}

func (t *Tree) allocLeaf() *page {
	id := t.nextID
	t.nextID++
	p := newLeaf(id)
	t.pages[id] = p
	t.markDirty(id)
	return p
}

func (t *Tree) allocInternal() *page {
	id := t.nextID
	t.nextID++
	p := newInternal(id)
	t.pages[id] = p
	t.markDirty(id)
	return p
}

func (t *Tree) findPath(k []byte) []PageID {
	var path []PageID
	id := t.root
	for {
		path = append(path, id)
		p := t.pages[id]
		if p.kind == KindLeaf {
			return path
		}
		id = p.childFor(k)
	}
}

func (t *Tree) findLeaf(k []byte) *page {
	p := t.pages[t.root]
	for p.kind != KindLeaf {
		p = t.pages[p.childFor(k)]
	}
	return p
}

func (t *Tree) leftmostLeaf() *page {
	p := t.pages[t.root]
	for p.kind != KindLeaf {
		p = t.pages[p.kids[0]]
	}
	return p
}

func (p *page) search(k []byte) (int, bool) {
	lo, hi := 0, len(p.keys)
	for lo < hi {
		mid := (lo + hi) / 2
		cmp := bytes.Compare(p.keys[mid], k)
		if cmp == 0 {
			return mid, true
		}
		if cmp < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, false
}

func (p *page) childFor(k []byte) PageID {
	i, ok := p.search(k)
	if ok {
		return p.kids[i+1]
	}
	return p.kids[i]
}

func (p *page) insertLeaf(k, v []byte) {
	i, _ := p.search(k)
	p.keys = insertAt(p.keys, i, cloneBytes(k))
	p.vals = insertAt(p.vals, i, cloneBytes(v))
}

func (p *page) insertInternal(sep []byte, right PageID) {
	i, _ := p.search(sep)
	p.keys = insertAt(p.keys, i, cloneBytes(sep))
	p.kids = insertPageAt(p.kids, i+1, right)
}

func insertAt(s [][]byte, i int, v []byte) [][]byte {
	s = append(s, nil)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

func insertPageAt(s []PageID, i int, v PageID) []PageID {
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = v
	return s
}

func cloneKeySlice(in [][]byte) [][]byte {
	out := make([][]byte, len(in))
	for i, k := range in {
		out[i] = cloneBytes(k)
	}
	return out
}
