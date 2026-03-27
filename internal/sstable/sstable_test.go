package sstable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
)

func TestCreateFromMemtableValidPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	m := memtable.New()
	key, value := "guangdong", "tigers"
	if err := m.Put(entry.New([]byte(key), []byte(value))); err != nil {
		t.Fatalf("put %v: %v", key, err)
	}
	key, value = "ni", "hao"
	if err := m.Put(entry.New([]byte(key), []byte(value))); err != nil {
		t.Fatalf("put %v: %v", key, err)
	}

	if err := CreateFromMemTable(path, m); err != nil {
		t.Fatalf("CreateFromMemTable: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected file, got directory")
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty file")
	}
}

func TestCreateFromMemtableEmptyPath(t *testing.T) {
	m := memtable.New()
	key, value := "shanghai", "sharks"
	if err := m.Put(entry.New([]byte(key), []byte(value))); err != nil {
		t.Fatalf("put %v: %v", key, err)
	}

	err := CreateFromMemTable("", m)
	if err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestCreateFromMemtableDirectoryPath(t *testing.T) {
	dir := t.TempDir()

	m := memtable.New()
	key, value := "beijing", "ducks"
	if err := m.Put(entry.New([]byte(key), []byte(value))); err != nil {
		t.Fatalf("put %v: %v", key, err)
	}

	err := CreateFromMemTable(dir, m)
	if err == nil {
		t.Fatalf("expected error for directory path")
	}
}

func TestCreateFromMemTableMissingParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing-parent", "table.sst")

	m := memtable.New()
	key, value := "shenzen", "leopards"
	if err := m.Put(entry.New([]byte(key), []byte(value))); err != nil {
		t.Fatalf("put %v: %v", key, err)
	}

	err := CreateFromMemTable(path, m)
	if err == nil {
		t.Fatalf("expected error for missing parent directory")
	}
}

func TestCreateFromMemTableNilMemTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	err := CreateFromMemTable(path, nil)
	if err == nil {
		t.Fatalf("expected error for nil memtable")
	}
}

func TestOpenValidPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if s == nil {
		t.Fatalf("expected non-nil SSTable")
	}
	if s.path != path {
		t.Fatalf("got path %q, want %q", s.path, path)
	}
}

func TestOpenEmptyPath(t *testing.T) {
	s, err := Open("")
	if err == nil {
		t.Fatalf("expected error for empty path")
	}
	if s != nil {
		t.Fatalf("expected nil SSTable on error")
	}
}

func TestOpenDirPath(t *testing.T) {
	dir := t.TempDir()

	s, err := Open(dir)
	if err == nil {
		t.Fatalf("expected error for directory path")
	}
	if s != nil {
		t.Fatalf("expected nil SSTable on error")
	}
}
