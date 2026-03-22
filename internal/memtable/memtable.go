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
	entry   *entry.Entry
	forward []*node
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
			forward: make([]*node, maxLevel),
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

	update, curr := m.find(e.Key)

	if curr != nil && bytes.Equal(curr.entry.Key, e.Key) {
		curr.entry = cloneEntry(e)
		return nil
	}

	lvl := m.randomLevel()
	if lvl > m.level {
		for i := m.level; i < lvl; i++ {
			update[i] = m.head
		}
		m.level = lvl
	}

	n := &node{
		entry:   cloneEntry(e),
		forward: make([]*node, lvl),
	}

	for i := 0; i < lvl; i++ {
		n.forward[i] = update[i].forward[i]
		update[i].forward[i] = n
	}

	m.size++
	return nil
}

func (m *MemTable) Get(key []byte) (*entry.Entry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	curr := m.head
	for i := m.level - 1; i >= 0; i-- {
		for curr.forward[i] != nil && bytes.Compare(curr.forward[i].entry.Key, key) < 0 {
			curr = curr.forward[i]
		}
	}

	curr = curr.forward[0]
	if curr != nil && bytes.Equal(curr.entry.Key, key) {
		return cloneEntry(curr.entry), true
	}

	return nil, false
}

func (m *MemTable) Scan(fn func(*entry.Entry) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for curr := m.head.forward[0]; curr != nil; curr = curr.forward[0] {
		if err := fn(cloneEntry(curr.entry)); err != nil {
			return fmt.Errorf("scan callback: %w", err)
		}
	}

	return nil
}

func (m *MemTable) find(key []byte) ([maxLevel]*node, *node) {
	var update [maxLevel]*node
	curr := m.head

	for i := m.level - 1; i >= 0; i-- {
		for curr.forward[i] != nil && bytes.Compare(curr.forward[i].entry.Key, key) < 0 {
			curr = curr.forward[i]
		}
		update[i] = curr
	}
	curr = curr.forward[0]
	return update, curr
}

func (m *MemTable) randomLevel() int {
	lvl := 1
	for lvl < maxLevel && m.rng.Float64() < probability {
		lvl++
	}
	return lvl
}

func cloneEntry(e *entry.Entry) *entry.Entry {
	if e == nil {
		return nil
	}

	key := append([]byte(nil), e.Key...)
	value := append([]byte(nil), e.Value...)

	return entry.New(key, value)
}
