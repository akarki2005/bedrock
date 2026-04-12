package engine

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/akarki2005/lsm-engine/internal/compaction"
	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
	"github.com/akarki2005/lsm-engine/internal/sstable"
	"github.com/akarki2005/lsm-engine/internal/wal"
)

const defaultFlushThreshold = 4 << 20
const baseLevelThreshold = 4

type Engine struct {
	mu             sync.RWMutex
	dir            string
	wal            *wal.WAL
	walImmutable   *wal.WAL
	mutable        *memtable.MemTable
	immutable      *memtable.MemTable
	sstables       [][]*sstable.SSTable
	flushThreshold int
	walID          int
	walImmutableID int
	nextSSTableID  int
}

type sstableFile struct {
	level int
	id    int
	name  string
}

func Open(path string) (*Engine, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create engine dir: %w", err)
	}

	tables, maxID, err := loadSSTables(path)
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
		nextSSTableID:  maxID,
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

	for level, _ := range e.sstables {
		for _, table := range e.sstables[level] {
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

	id, finalPath := e.nextSSTablePath(0)
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

	e.ensureLevel(0)
	e.sstables[0] = append([]*sstable.SSTable{table}, e.sstables[0]...)

	if err := e.maybeCompact(); err != nil {
		return err
	}

	e.immutable = nil
	e.nextSSTableID = id

	return nil
}

func (e *Engine) nextSSTablePath(level int) (int, string) {
	id := e.nextSSTableID + 1
	path := filepath.Join(e.dir, fmt.Sprintf("l%d-sst-%03d.db", level, id))
	return id, path
}

func (e *Engine) nextWALPath() (int, string) {
	id := e.walID + 1
	path := filepath.Join(e.dir, fmt.Sprintf("wal-%03d.log", id))
	return id, path
}

func (e *Engine) compactLevel(level int) error {
	e.ensureLevel(level + 1)

	plan, err := e.planCompaction(level)
	if err != nil {
		return fmt.Errorf("plan compaction: %w", err)
	}
	if plan == nil {
		return nil
	}

	chunks, err := compaction.Run(plan)
	if err != nil {
		return fmt.Errorf("Run compaction: %w", err)
	}

	outputs, err := e.writeOutputs(plan.Level()+1, chunks)
	if err != nil {
		return fmt.Errorf("write outputs: %w", err)
	}

	if err := e.applyCompaction(plan, outputs); err != nil {
		return fmt.Errorf("apply compaction: %w", err)
	}

	return nil
}

func (e *Engine) planCompaction(level int) (*compaction.Plan, error) {
	if level < 0 || level >= len(e.sstables) {
		return nil, fmt.Errorf("invalid level %d", level)
	}

	if len(e.sstables[level]) == 0 {
		return nil, nil
	}

	input := e.sstables[level][0]
	inputs := []*sstable.SSTable{input}

	var overlapTables []*sstable.SSTable
	if level+1 < len(e.sstables) {
		for _, table := range e.sstables[level+1] {
			if overlaps(input, table) {
				overlapTables = append(overlapTables, table)
			}
		}
	}

	return compaction.NewPlan(level, inputs, overlapTables), nil
}

func (e *Engine) writeOutputs(level int, chunks [][]*entry.Entry) ([]*sstable.SSTable, error) {

	var outputs []*sstable.SSTable
	var createdPaths []string

	for _, chunk := range chunks {
		id, finalPath := e.nextSSTablePath(level)
		tempPath := finalPath + ".tmp"

		if err := sstable.CreateFromEntries(tempPath, chunk); err != nil {
			for _, path := range createdPaths {
				_ = os.Remove(path)
			}
			return nil, fmt.Errorf("create output sstable: %w", err)
		}

		if err := os.Rename(tempPath, finalPath); err != nil {
			_ = os.Remove(tempPath)
			for _, path := range createdPaths {
				_ = os.Remove(path)
			}
			return nil, fmt.Errorf("rename output sstable: %w", err)
		}

		table, err := sstable.Open(finalPath)
		if err != nil {
			_ = os.Remove(finalPath)
			for _, path := range createdPaths {
				_ = os.Remove(path)
			}
			return nil, fmt.Errorf("open output sstable: %w", err)
		}

		outputs = append(outputs, table)
		createdPaths = append(createdPaths, finalPath)
		e.nextSSTableID = id
	}

	return outputs, nil
}

func (e *Engine) applyCompaction(plan *compaction.Plan, outputs []*sstable.SSTable) error {
	if plan == nil {
		return fmt.Errorf("nil compaction plan")
	}

	level := plan.Level()
	nextLevel := level + 1

	e.ensureLevel(nextLevel)

	e.sstables[level] = removeTables(e.sstables[level], plan.Inputs())
	e.sstables[nextLevel] = removeTables(e.sstables[nextLevel], plan.Overlaps())

	e.sstables[nextLevel] = append(outputs, e.sstables[nextLevel]...)

	for _, table := range plan.Inputs() {
		if err := os.Remove(table.Path()); err != nil {
			return fmt.Errorf("remove compacted input sstable: %w", err)
		}
	}

	for _, table := range plan.Overlaps() {
		if err := os.Remove(table.Path()); err != nil {
			return fmt.Errorf("remove compacted input sstable: %w", err)
		}
	}

	return nil
}

func (e *Engine) ensureLevel(level int) {
	for len(e.sstables) <= level {
		e.sstables = append(e.sstables, nil)
	}
}

func (e *Engine) shouldCompact(level int) bool {
	if level < 0 || level >= len(e.sstables) {
		return false
	}
	return len(e.sstables[level]) > levelThreshold(level)
}

// cascading compaction algorithm
func (e *Engine) maybeCompact() error {
	for level := 0; level < len(e.sstables); level++ {
		for e.shouldCompact(level) {
			if err := e.compactLevel(level); err != nil {
				return err
			}
		}
	}
	return nil
}

func loadSSTables(dir string) ([][]*sstable.SSTable, int, error) {

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, fmt.Errorf("Read engine dir: %w", err)
	}

	var files []sstableFile
	maxLevel := -1
	maxID := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		var level, id int
		name := entry.Name()

		if _, err := fmt.Sscanf(name, "l%d-sst-%03d.db", &level, &id); err != nil {
			continue
		}

		files = append(files, sstableFile{
			level: level,
			id:    id,
			name:  name,
		})

		if level > maxLevel {
			maxLevel = level
		}
		if id > maxID {
			maxID = id
		}
	}

	tables := make([][]*sstable.SSTable, maxLevel+1)

	sort.Slice(files, func(i, j int) bool {
		if files[i].level != files[j].level {
			return files[i].level < files[j].level
		}
		return files[i].id > files[j].id
	})

	for _, f := range files {
		path := filepath.Join(dir, f.name)

		table, err := sstable.Open(path)
		if err != nil {
			return nil, 0, fmt.Errorf("open sstable %q: %w", f.name, err)
		}

		tables[f.level] = append(tables[f.level], table)
	}

	return tables, maxID, nil
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

func overlaps(a, b *sstable.SSTable) bool {
	return !(bytes.Compare(a.MaxKey(), b.MinKey()) < 0 || bytes.Compare(b.MaxKey(), a.MinKey()) < 0)
}

func removeTables(tables []*sstable.SSTable, toRemove []*sstable.SSTable) []*sstable.SSTable {
	removeSet := make(map[*sstable.SSTable]struct{}, len(toRemove))
	for _, table := range toRemove {
		removeSet[table] = struct{}{}
	}

	result := make([]*sstable.SSTable, 0, len(tables))
	for _, table := range tables {
		if _, ok := removeSet[table]; ok {
			continue
		}
		result = append(result, table)
	}

	return result
}

func levelThreshold(level int) int {
	return baseLevelThreshold + level*2
}
