package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
	"github.com/akarki2005/lsm-engine/internal/sstable"
	"github.com/akarki2005/lsm-engine/internal/wal"
)

const defaultFlushThreshold = 4 << 20

type Engine struct {
	mu             sync.RWMutex
	dir            string
	wal            *wal.WAL
	walImmutable   *wal.WAL
	mutable        *memtable.MemTable
	immutable      *memtable.MemTable
	sstables       []*sstable.SSTable
	flushThreshold int
	walID          int
	walImmutableID int
	nextSSTableID  int
}

func Open(path string) (*Engine, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create engine dir: %w", err)
	}

	tables, err := loadSSTables(path)
	if err != nil {
		return nil, fmt.Errorf("load SSTables: %w", err)
	}

	wal, walID, err := loadActiveWAL(path)
	if err != nil {
		return nil, fmt.Errorf("open WAL: %w", err)
	}

	mt := memtable.New()

	if err := wal.Replay(mt.Put); err != nil {
		_ = wal.Close()
		return nil, fmt.Errorf("replay WAL into memtable: %w", err)
	}

	return &Engine{
		dir:            path,
		wal:            wal,
		walID:          walID,
		mutable:        mt,
		sstables:       tables,
		flushThreshold: defaultFlushThreshold,
	}, nil
}

func (e *Engine) Put(key, value []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ent := entry.New(key, value)

	if err := e.wal.Append(ent); err != nil {
		return fmt.Errorf("append to WAL: %w", err)
	}

	if err := e.mutable.Put(ent); err != nil {
		return fmt.Errorf("put into memtable: %w", err)
	}

	if e.mutable.Bytes() >= e.flushThreshold {
		if err := e.rotateMemTable(); err != nil {
			return fmt.Errorf("rotate memtable: %w", err)
		}

		if err := e.flushImmutable(); err != nil {
			return fmt.Errorf("flush immutable memtable: %w", err)
		}
	}

	return nil
}

func (e *Engine) Get(key []byte) ([]byte, bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if ent, ok := e.mutable.Get(key); ok {
		if ent.Tombstone {
			return nil, false, nil
		}
		return append([]byte(nil), ent.Value...), true, nil
	}

	if e.immutable != nil {
		if ent, ok := e.immutable.Get(key); ok {
			if ent.Tombstone {
				return nil, false, nil
			}
			return append([]byte(nil), ent.Value...), true, nil
		}
	}

	for _, table := range e.sstables {
		ent, ok, err := table.Get(key)
		if err != nil {
			return nil, false, fmt.Errorf("get from SSTable: %w", err)
		}
		if ok {
			if ent.Tombstone {
				return nil, false, nil
			}
			return append([]byte(nil), ent.Value...), true, nil
		}
	}

	return nil, false, nil
}

func (e *Engine) Delete(key []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry := entry.NewTombstone(key)

	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("append tombstone to WAL: %w", err)
	}

	if err := e.mutable.Put(entry); err != nil {
		return fmt.Errorf("put tombstone into memtable: %w", err)
	}

	if e.mutable.Bytes() >= e.flushThreshold {
		if err := e.rotateMemTable(); err != nil {
			return fmt.Errorf("rotate memtable: %w", err)
		}

		if err := e.flushImmutable(); err != nil {
			return fmt.Errorf("flush immutable memtable: %w", err)
		}
	}

	return nil
}

func (e *Engine) Close() error {
	if err := e.wal.Close(); err != nil {
		return fmt.Errorf("close WAL: %w", err)
	}
	return nil
}

func (e *Engine) rotateMemTable() error {
	if e.immutable != nil {
		return fmt.Errorf("immutable memtable already exists")
	}

	e.immutable = e.mutable
	e.walImmutable = e.wal
	e.walImmutableID = e.walID

	e.mutable = memtable.New()

	id, walPath := e.nextWALPath()
	w, err := wal.Open(walPath)
	if err != nil {
		return fmt.Errorf("Open WAL: %w", err)
	}
	e.wal = w
	e.walID = id

	return nil
}

func (e *Engine) flushImmutable() error {
	if e.immutable == nil {
		return nil
	}

	id, finalPath := e.nextSSTablePath()
	tempPath := finalPath + ".tmp"

	err := sstable.CreateFromMemTable(tempPath, e.immutable)
	if err != nil {
		return fmt.Errorf("create SSTable from immutable memtable: %w", err)
	}

	err = os.Rename(tempPath, finalPath)
	if err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("rename temp SSTable: %w", err)
	}

	table, err := sstable.Open(finalPath)
	if err != nil {
		_ = os.Remove(finalPath)
		return fmt.Errorf("open flushed SSTABLE: %w", err)
	}

	if err := e.walImmutable.Close(); err != nil {
		return fmt.Errorf("Close WAL: %w", err)
	}

	oldPath := filepath.Join(e.dir, fmt.Sprintf("wal-%03d.log", e.walImmutableID))
	if err := os.Remove(oldPath); err != nil {
		return fmt.Errorf("remove immutable WAL: %w", err)
	}

	e.walImmutable = nil

	e.sstables = append([]*sstable.SSTable{table}, e.sstables...)
	e.immutable = nil
	e.nextSSTableID = id

	return nil
}

func (e *Engine) nextSSTablePath() (int, string) {
	id := e.nextSSTableID + 1
	path := filepath.Join(e.dir, fmt.Sprintf("sst-%03d.db", id))
	return id, path
}

func (e *Engine) nextWALPath() (int, string) {
	id := e.walID + 1
	path := filepath.Join(e.dir, fmt.Sprintf("wal-%03d.log", id))
	return id, path
}

func loadSSTables(dir string) ([]*sstable.SSTable, error) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("Read engine dir: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "sst-") || !strings.HasSuffix(name, ".db") {
			continue
		}

		names = append(names, name)
	}

	sort.Strings(names)

	// go in reverse order so newest SSTables come first
	tables := make([]*sstable.SSTable, 0, len(names))
	for i := len(names) - 1; i >= 0; i-- {
		path := filepath.Join(dir, names[i])

		table, err := sstable.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open SSTable: %w", err)
		}

		tables = append(tables, table)
	}

	return tables, nil
}

func loadActiveWAL(dir string) (*wal.WAL, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("read engine dir: %w", err)
	}

	maxID := -1

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "wal-") || !strings.HasSuffix(name, ".log") {
			continue
		}

		var id int
		_, err := fmt.Sscanf(name, "wal-%03d.log", &id)
		if err != nil {
			return nil, 0, fmt.Errorf("parse WAL filename %q: %w", name, err)
		}

		if id > maxID {
			maxID = id
		}
	}

	if maxID == -1 {
		maxID = 1
	}

	path := filepath.Join(dir, fmt.Sprintf("wal-%03d.log", maxID))
	wal, err := wal.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open active WAL: %w", err)
	}

	return wal, maxID, nil
}
