package sstable

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/akarki2005/bedrock/internal/entry"
	"github.com/akarki2005/bedrock/internal/memtable"
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

func TestOpenCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	if err := os.WriteFile(path, []byte{1, 2, 3}, 0644); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	_, err := Open(path)
	if err == nil {
		t.Fatalf("expected error for corrupt file")
	}
}

func TestScanEmptyFile(t *testing.T) {
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
		t.Fatalf("Open: %v", err)
	}

	calls := 0
	err = s.Scan(func(e *entry.Entry) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if calls != 0 {
		t.Fatalf("got %d callback calls, want 0", calls)
	}
}

func TestScanValidEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	key1, key2, key3 := "johnny", "elias", "matthew"
	val1, val2, val3 := "gaudreau", "lindholm", "tkachuk"
	entries := []*entry.Entry{
		entry.New([]byte(key1), []byte(val1)),
		entry.New([]byte(key2), []byte(val2)),
		entry.New([]byte(key3), []byte(val3)),
	}

	for _, e := range entries {
		if err := e.WriteTo(f); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	var got []string
	err = s.Scan(func(e *entry.Entry) error {
		got = append(got, string(e.Key))
		return nil
	})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	want := []string{key1, key2, key3}
	if len(got) != len(want) {
		t.Fatalf("got %d keys, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got keys %v, want %v", got, want)
		}
	}
}

func TestScanCallbackError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	key1, value1 := "windsor", "spitfires"
	key2, value2 := "london", "knights"
	entries := []*entry.Entry{
		entry.New([]byte(key1), []byte(value1)),
		entry.New([]byte(key2), []byte(value2)),
	}

	for _, e := range entries {
		if err := e.WriteTo(f); err != nil {
			t.Fatalf("write entry: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	calls := 0
	wantErr := errors.New("stop scan")

	err = s.Scan(func(e *entry.Entry) error {
		calls++
		if string(e.Key) == key2 {
			return wantErr
		}
		return nil
	})
	if err == nil {
		t.Fatalf("expected callback error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want wrapped %v", err, wantErr)
	}
	if calls != 2 {
		t.Fatalf("got %d callback calls, want 2", calls)
	}
}

func TestGetFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	key1, value1 := "hurricane", "katrina"
	if err := entry.New([]byte(key1), []byte(value1)).WriteTo(f); err != nil {
		t.Fatalf("write %v: %v", key1, err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	key := "hurricane"
	got, ok, err := s.Get([]byte(key))
	if err != nil {
		t.Fatalf("get %v: %v", key, err)
	}
	if !ok {
		t.Fatalf("expected key %q to be found", key)
	}
	if got == nil {
		t.Fatalf("expected non-nil entry")
	}
	if string(got.Key) != key {
		t.Fatalf("got key %q, want %q", got.Key, key)
	}

	wantValue := "katrina"
	if string(got.Value) != wantValue {
		t.Fatalf("got value %q, want %q", got.Value, wantValue)
	}
}

func TestGetNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "table.sst")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	key1, value1 := "hurricane", "tortilla"
	if err := entry.New([]byte(key1), []byte(value1)).WriteTo(f); err != nil {
		t.Fatalf("write %v: %v", key1, err)
	}

	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	key := "typhoon"
	got, ok, err := s.Get([]byte(key))
	if err != nil {
		t.Fatalf("get %v: %v", key, err)
	}
	if ok {
		t.Fatalf("expected key %q to be missing", key)
	}
	if got != nil {
		t.Fatalf("expected nil entry for missing key")
	}
}
