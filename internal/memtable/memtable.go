package memtable

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/akarki2005/lsm-engine/internal/entry"
)

const (
	maxLevel    = 16
	probability = 0.5
)

type node struct {
	entry     *entry.Entry
	successor []*node
}

type MemTable struct {
	mu    sync.RWMutex
	head  *node
	level int
	size  int
	rng   *rand.Rand
}

func New() *MemTable {
	return &MemTable{
		head: &node{
			successor: make([]*node, maxLevel),
		},
		level: 1,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *MemTable) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.size
}

func (m *MemTable) Put(e *entry.Entry) error {
	if e == nil {
		return fmt.Errorf("put nil entry")
	}
	if e.Key == nil {
		return fmt.Errorf("put entry with nil key")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	predecessor, curr := m.find(e.Key)

	if curr != nil && bytes.Equal(curr.entry.Key, e.Key) {
		curr.entry = entry.CloneEntry(e)
		return nil
	}

	lvl := m.randomLevel()
	if lvl > m.level {
		for i := m.level; i < lvl; i++ {
			predecessor[i] = m.head
		}
		m.level = lvl
	}

	n := &node{
		entry:     entry.CloneEntry(e),
		successor: make([]*node, lvl),
	}

	for i := 0; i < lvl; i++ {
		n.successor[i] = predecessor[i].successor[i]
		predecessor[i].successor[i] = n
	}

	m.size++
	return nil
}

func (m *MemTable) Get(key []byte) (*entry.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	curr := m.head
	for i := m.level - 1; i >= 0; i-- {
		for curr.successor[i] != nil && bytes.Compare(curr.successor[i].entry.Key, key) < 0 {
			curr = curr.successor[i]
		}
	}

	curr = curr.successor[0]
	if curr != nil && bytes.Equal(curr.entry.Key, key) {
		return entry.CloneEntry(curr.entry), true
	}

	return nil, false
}

func (m *MemTable) Scan(fn func(*entry.Entry) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for curr := m.head.successor[0]; curr != nil; curr = curr.successor[0] {
		if err := fn(entry.CloneEntry(curr.entry)); err != nil {
			return fmt.Errorf("scan callback: %w", err)
		}
	}

	return nil
}

func (m *MemTable) find(key []byte) ([maxLevel]*node, *node) {
	var predecessor [maxLevel]*node
	curr := m.head

	for i := m.level - 1; i >= 0; i-- {
		for curr.successor[i] != nil && bytes.Compare(curr.successor[i].entry.Key, key) < 0 {
			curr = curr.successor[i]
		}
		predecessor[i] = curr
	}
	curr = curr.successor[0]
	return predecessor, curr
}

func (m *MemTable) randomLevel() int {
	lvl := 1
	for lvl < maxLevel && m.rng.Float64() < probability {
		lvl++
	}
	return lvl
}
