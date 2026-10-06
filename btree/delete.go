package btree

func (t *Tree) Delete(key string) (bool, error) {
	if key == "" {
		return false, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	k := []byte(key)
	leaf := t.findLeaf(k)
	if _, ok := leaf.search(k); !ok {
		return false, nil
	}
	if err := t.walAppend("delete", key, nil); err != nil {
		return false, err
	}
	return t.deleteLocked(key), nil
}

func (t *Tree) deleteLocked(key string) bool {
	k := []byte(key)
	path := t.findPath(k)
	leaf := t.pages[path[len(path)-1]]
	i, ok := leaf.search(k)
	if !ok {
		return false
	}
	leaf.keys = append(leaf.keys[:i], leaf.keys[i+1:]...)
	leaf.vals = append(leaf.vals[:i], leaf.vals[i+1:]...)
	t.markDirty(leaf.id)
	if len(leaf.keys) == 0 && leaf.id != t.root {
		t.freeEmptyLeaf(path)
	}
	return true
}

func (t *Tree) freeEmptyLeaf(path []PageID) {
	leaf := t.pages[path[len(path)-1]]
	if leaf.prev != 0 {
		if prev := t.pages[leaf.prev]; prev != nil {
			prev.next = leaf.next
			t.markDirty(prev.id)
		}
	}
	if leaf.next != 0 {
		if next := t.pages[leaf.next]; next != nil {
			next.prev = leaf.prev
			t.markDirty(next.id)
		}
	}
	if len(path) < 2 {
		return
	}
	parent := t.pages[path[len(path)-2]]
	ci := -1
	for i, kid := range parent.kids {
		if kid == leaf.id {
			ci = i
			break
		}
	}
	if ci < 0 {
		return
	}
	parent.kids = append(parent.kids[:ci], parent.kids[ci+1:]...)
	if ci == 0 {
		if len(parent.keys) > 0 {
			parent.keys = parent.keys[1:]
		}
	} else if ci-1 < len(parent.keys) {
		parent.keys = append(parent.keys[:ci-1], parent.keys[ci:]...)
	}
	if len(parent.kids) > 0 {
		parent.extra = parent.kids[0]
	}
	t.markDirty(parent.id)
	t.release(leaf.id)

	if parent.id == t.root && len(parent.keys) == 0 && len(parent.kids) == 1 {
		child := parent.kids[0]
		t.root = child
		t.height--
		t.release(parent.id)
	}
}

func (t *Tree) release(id PageID) {
	delete(t.pages, id)
	t.free = append(t.free, id)
	t.markDirty(id)
}
