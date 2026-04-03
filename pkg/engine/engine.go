package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
	"github.com/akarki2005/lsm-engine/internal/sstable"
	"github.com/akarki2005/lsm-engine/internal/wal"
)

const defaultFlushThreshold = 4 << 20

type Engine struct {
	dir            string
	wal            *wal.WAL
	mutable        *memtable.MemTable
	immutable      *memtable.MemTable
	sstables       []*sstable.SSTable
	flushThreshold int
}

func Open(path string) (*Engine, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create engine dir: %w", err)
	}

	tables, err := loadSSTables(path)
	if err != nil {
		return nil, fmt.Errorf("load SSTables: %w", err)
	}

	walPath := filepath.Join(path, "wal.log")
	wal, err := wal.Open(walPath)
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
		mutable:        mt,
		sstables:       tables,
		flushThreshold: defaultFlushThreshold,
	}, nil
}

func (e *Engine) Put(key, value []byte) error {
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
	e.mutable = memtable.New()

	return nil
}

func (e *Engine) flushImmutable() error {
	if e.immutable == nil {
		return nil
	}

	finalPath := e.nextSSTablePath()
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

	e.sstables = append([]*sstable.SSTable{table}, e.sstables...)
	e.immutable = nil

	return nil
}

func (e *Engine) nextSSTablePath() string {
	return filepath.Join(e.dir, fmt.Sprintf("sst-%03d.db", len(e.sstables)+1))
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
