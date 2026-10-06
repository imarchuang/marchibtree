package btree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

const (
	DefaultPageSize = 4096
	pageHeaderSize  = 32
	magic           = 0x42543031 // "BT01"
)

type Kind uint16

const (
	KindLeaf Kind = 1 + iota
	KindInternal
)

type PageID uint32

type page struct {
	id    PageID
	kind  Kind
	nKeys uint16
	prev  PageID
	next  PageID
	extra PageID // leftmost child on internal nodes
	keys  [][]byte
	vals  [][]byte // leaf values; nil on internal
	kids  []PageID // internal children; len = nKeys+1
}

func newLeaf(id PageID) *page {
	return &page{id: id, kind: KindLeaf}
}

func newInternal(id PageID) *page {
	return &page{id: id, kind: KindInternal}
}

func (p *page) encodedLen() int {
	n := pageHeaderSize
	for i, k := range p.keys {
		n += 2 + len(k)
		if p.kind == KindLeaf {
			n += 2 + len(p.vals[i])
		} else {
			n += 4 // right child of this key; leftmost is extra
		}
	}
	return n
}

func (p *page) fits(pageSize int) bool {
	return p.encodedLen() <= pageSize
}

func (p *page) encode(pageSize int) ([]byte, error) {
	if pageSize < pageHeaderSize {
		return nil, fmt.Errorf("page size %d too small", pageSize)
	}
	if p.kind == KindInternal && len(p.kids) > 0 {
		p.extra = p.kids[0]
	}
	buf := make([]byte, pageSize)
	binary.BigEndian.PutUint32(buf[0:4], magic)
	binary.BigEndian.PutUint32(buf[4:8], uint32(p.id))
	binary.BigEndian.PutUint16(buf[8:10], uint16(p.kind))
	binary.BigEndian.PutUint16(buf[10:12], uint16(len(p.keys)))
	binary.BigEndian.PutUint32(buf[12:16], uint32(p.prev))
	binary.BigEndian.PutUint32(buf[16:20], uint32(p.next))
	binary.BigEndian.PutUint32(buf[20:24], uint32(p.extra))
	// crc at 24:28 filled later; 28:32 reserved
	off := pageHeaderSize
	for i, k := range p.keys {
		if off+2+len(k) > pageSize {
			return nil, fmt.Errorf("page %d overflow encoding key %d", p.id, i)
		}
		binary.BigEndian.PutUint16(buf[off:off+2], uint16(len(k)))
		off += 2
		copy(buf[off:], k)
		off += len(k)
		if p.kind == KindLeaf {
			v := p.vals[i]
			if off+2+len(v) > pageSize {
				return nil, fmt.Errorf("page %d overflow encoding val %d", p.id, i)
			}
			binary.BigEndian.PutUint16(buf[off:off+2], uint16(len(v)))
			off += 2
			copy(buf[off:], v)
			off += len(v)
		} else {
			if off+4 > pageSize {
				return nil, fmt.Errorf("page %d overflow encoding child %d", p.id, i)
			}
			binary.BigEndian.PutUint32(buf[off:off+4], uint32(p.kids[i+1]))
			off += 4
		}
	}
	sum := crc32.ChecksumIEEE(crcRegion(buf))
	binary.BigEndian.PutUint32(buf[24:28], sum)
	return buf, nil
}

func decodePage(buf []byte) (*page, error) {
	if len(buf) < pageHeaderSize {
		return nil, fmt.Errorf("short page")
	}
	if binary.BigEndian.Uint32(buf[0:4]) != magic {
		return nil, fmt.Errorf("bad magic")
	}
	got := binary.BigEndian.Uint32(buf[24:28])
	want := crc32.ChecksumIEEE(crcRegion(buf))
	if got != want {
		return nil, fmt.Errorf("crc mismatch")
	}
	p := &page{
		id:    PageID(binary.BigEndian.Uint32(buf[4:8])),
		kind:  Kind(binary.BigEndian.Uint16(buf[8:10])),
		nKeys: binary.BigEndian.Uint16(buf[10:12]),
		prev:  PageID(binary.BigEndian.Uint32(buf[12:16])),
		next:  PageID(binary.BigEndian.Uint32(buf[16:20])),
		extra: PageID(binary.BigEndian.Uint32(buf[20:24])),
	}
	off := pageHeaderSize
	n := int(p.nKeys)
	p.keys = make([][]byte, 0, n)
	if p.kind == KindLeaf {
		p.vals = make([][]byte, 0, n)
	} else {
		p.kids = make([]PageID, 0, n+1)
		p.kids = append(p.kids, p.extra)
	}
	for i := 0; i < n; i++ {
		if off+2 > len(buf) {
			return nil, fmt.Errorf("truncated key len")
		}
		kl := int(binary.BigEndian.Uint16(buf[off : off+2]))
		off += 2
		if off+kl > len(buf) {
			return nil, fmt.Errorf("truncated key")
		}
		k := bytes.Clone(buf[off : off+kl])
		off += kl
		p.keys = append(p.keys, k)
		if p.kind == KindLeaf {
			if off+2 > len(buf) {
				return nil, fmt.Errorf("truncated val len")
			}
			vl := int(binary.BigEndian.Uint16(buf[off : off+2]))
			off += 2
			if off+vl > len(buf) {
				return nil, fmt.Errorf("truncated val")
			}
			p.vals = append(p.vals, bytes.Clone(buf[off:off+vl]))
			off += vl
		} else {
			if off+4 > len(buf) {
				return nil, fmt.Errorf("truncated child")
			}
			p.kids = append(p.kids, PageID(binary.BigEndian.Uint32(buf[off:off+4])))
			off += 4
		}
	}
	return p, nil
}

func crcRegion(buf []byte) []byte {
	tmp := make([]byte, len(buf))
	copy(tmp, buf)
	binary.BigEndian.PutUint32(tmp[24:28], 0)
	return tmp
}

func cloneBytes(b []byte) []byte {
	return bytes.Clone(b)
}
