package main

import (
	"crypto/sha1"
	"encoding/binary"
	"sort"
	"sync"
)

// HashRing implements consistent hashing with virtual nodes.
type HashRing struct {
	mu           sync.RWMutex
	virtualNodes int
	ring         map[uint32]string // position on ring -> physical node ID
	sortedKeys   []uint32          // sorted ring positions, for binary search
}

func NewHashRing(virtualNodes int) *HashRing {
	return &HashRing{
		virtualNodes: virtualNodes,
		ring:         make(map[uint32]string),
	}
}

func hashKey(key string) uint32 {
	h := sha1.Sum([]byte(key))
	return binary.BigEndian.Uint32(h[:4])
}

// AddNode places a physical node onto the ring at `virtualNodes` positions.
func (hr *HashRing) AddNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	for i := 0; i < hr.virtualNodes; i++ {
		vKey := nodeID + "#" + string(rune(i))
		pos := hashKey(vKey)
		hr.ring[pos] = nodeID
	}
	hr.rebuildSortedKeys()
}

// RemoveNode removes all virtual positions for a physical node.
func (hr *HashRing) RemoveNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	for i := 0; i < hr.virtualNodes; i++ {
		vKey := nodeID + "#" + string(rune(i))
		pos := hashKey(vKey)
		delete(hr.ring, pos)
	}
	hr.rebuildSortedKeys()
}

// GetNode returns which physical node owns the given key.
func (hr *HashRing) GetNode(key string) (string, bool) {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.sortedKeys) == 0 {
		return "", false
	}

	target := hashKey(key)

	// Find the first ring position >= target (walk clockwise).
	idx := sort.Search(len(hr.sortedKeys), func(i int) bool {
		return hr.sortedKeys[i] >= target
	})

	// Wrap around the ring if we went past the end.
	if idx == len(hr.sortedKeys) {
		idx = 0
	}

	return hr.ring[hr.sortedKeys[idx]], true
}

func (hr *HashRing) rebuildSortedKeys() {
	hr.sortedKeys = hr.sortedKeys[:0]
	for k := range hr.ring {
		hr.sortedKeys = append(hr.sortedKeys, k)
	}
	sort.Slice(hr.sortedKeys, func(i, j int) bool {
		return hr.sortedKeys[i] < hr.sortedKeys[j]
	})
}