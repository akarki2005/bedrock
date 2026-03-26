package sstable

import (
	"errors"
	"fmt"
	"os"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
)

type SSTable struct {
	path string
}

func CreateFromMemTable(path string, m *memtable.MemTable) error {
	if m == nil {
		return errors.New("nil memtable")
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}

	success := false
	defer func() {
		_ = file.Close()
		if !success {
			_ = os.Remove(path)
		}
	}()

	err = m.Scan(func(e *entry.Entry) error {
		if err := e.WriteTo(file); err != nil {
			return fmt.Errorf("write entry to sstable: %w", err)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("flush memtable to sstable: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync sstable file: %w", err)
	}

	success = true
	return nil
}

/*
func Open(path string) (*SSTable, error) {

}

func (s *SSTable) Scan(fn func(*entry.Entry) error) error {

}

func (s *SSTable) Get(key []byte) (*entry.Entry, bool, error) {

}
*/
