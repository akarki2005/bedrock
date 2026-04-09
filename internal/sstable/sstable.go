package sstable

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
)

type SSTable struct {
	path   string
	minKey []byte
	maxKey []byte
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

func Open(path string) (*SSTable, error) {
	if path == "" {
		return nil, errors.New("empty path")
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat sstable: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("sstable path points to a directory")
	}

	s := &SSTable{path: path}

	first := true
	if err := s.scanFile(func(e *entry.Entry) error {
		if first {
			s.minKey = append([]byte(nil), e.Key...)
			first = false
		}
		s.maxKey = append([]byte(nil), e.Key...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("load sstable metadata: %w", err)
	}

	return s, nil
}

func (s *SSTable) Scan(fn func(*entry.Entry) error) error {
	return s.scanFile(fn)
}

func (s *SSTable) Get(key []byte) (*entry.Entry, bool, error) {
	file, err := os.Open(s.path)
	if err != nil {
		return nil, false, fmt.Errorf("open sstable: %w", err)
	}
	defer file.Close()

	for {
		e, err := entry.ReadFrom(file)
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read entry: %w", err)
		}

		cmp := bytes.Compare(e.Key, key)

		if cmp == 0 {
			return e, true, nil
		}
		if cmp > 0 { // we're past the point where we'd have seen the key
			return nil, false, nil
		}
	}
}

func (s *SSTable) MinKey() []byte {
	return append([]byte(nil), s.minKey...)
}

func (s *SSTable) MaxKey() []byte {
	return append([]byte(nil), s.maxKey...)
}

func (s *SSTable) scanFile(fn func(*entry.Entry) error) error {
	file, err := os.Open(s.path)
	if err != nil {
		return fmt.Errorf("open sstable: %w", err)
	}
	defer file.Close()

	for {
		e, err := entry.ReadFrom(file)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read entry: %w", err)
		}

		if err := fn(e); err != nil {
			return err
		}
	}
}
