package engine

import (
	"fmt"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
	"github.com/akarki2005/lsm-engine/internal/wal"
)

type Engine struct {
	wal      *wal.WAL
	memtable *memtable.MemTable
}

func Open(path string) (*Engine, error) {
	wal, err := wal.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open WAL: %w", err)
	}

	mt := memtable.New()

	if err := wal.Replay(mt.Put); err != nil {
		_ = wal.Close()
		return nil, fmt.Errorf("replay WAL into memtable: %w", err)
	}

	return &Engine{
		wal:      wal,
		memtable: mt,
	}, nil
}

func (e *Engine) Put(key, value []byte) error {
	ent := entry.New(key, value)

	if err := e.wal.Append(ent); err != nil {
		return fmt.Errorf("append to WAL: %w", err)
	}

	if err := e.memtable.Put(ent); err != nil {
		return fmt.Errorf("put into memtable: %w", err)
	}

	return nil
}

func (e *Engine) Get(key []byte) ([]byte, bool, error) {
	ent, ok := e.memtable.Get(key)

	if !ok {
		return nil, false, nil
	}

	value := append([]byte(nil), ent.Value...)
	return value, true, nil
}

func (e *Engine) Delete(key []byte) error {
	entry := entry.NewTombstone(key)

	if err := e.wal.Append(entry); err != nil {
		return fmt.Errorf("append tombstone to WAL: %w", err)
	}

	if err := e.memtable.Put(entry); err != nil {
		return fmt.Errorf("put tombstone into memtable: %w", err)
	}

	return nil
}

func (e *Engine) Close() error {
	if err := e.wal.Close(); err != nil {
		return fmt.Errorf("close WAL: %w", err)
	}
	return nil
}
