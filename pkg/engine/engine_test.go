package engine

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/memtable"
)

func TestOpenReplaysWAL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	e, err := Open(path)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}

	key := []byte("abbotsford")
	value := []byte("canucks")

	if err := e.Put(key, value); err != nil {
		t.Fatalf("put %q: %v", key, err)
	}

	if err := e.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	e, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			t.Fatalf("close reopened engine: %v", err)
		}
	}()

	got, ok, err := e.Get(key)
	if err != nil {
		t.Fatalf("get %q: %v", key, err)
	}
	if !ok {
		t.Fatalf("expected key %q to exist after replay", key)
	}
	if !bytes.Equal(got, value) {
		t.Fatalf("value = %q, want %q", got, value)
	}
}

func TestDeleteRemovesKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wal.log")

	e, err := Open(path)
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer func() {
		if err := e.Close(); err != nil {
			t.Fatalf("close engine: %v", err)
		}
	}()

	key := []byte("toronto")
	value := []byte("ontario")

	if err := e.Put(key, value); err != nil {
		t.Fatalf("put %q: %v", key, err)
	}

	if err := e.Delete(key); err != nil {
		t.Fatalf("delete %q: %v", key, err)
	}

	got, ok, err := e.Get(key)
	if err != nil {
		t.Fatalf("get %q: %v", key, err)
	}
	if ok {
		t.Fatalf("expected key %q to be deleted, got value %q", key, got)
	}
}

func TestGetPrefersMutableMemtable(t *testing.T) {
	e := &Engine{
		mutable:   memtable.New(),
		immutable: memtable.New(),
	}

	key := []byte("washington")
	oldValue := []byte("redskins")
	newValue := []byte("commanders")

	if err := e.immutable.Put(entry.New(key, oldValue)); err != nil {
		t.Fatalf("put immutable: %v", err)
	}
	if err := e.mutable.Put(entry.New(key, newValue)); err != nil {
		t.Fatalf("put mutable: %v", err)
	}

	got, ok, err := e.Get(key)
	if err != nil {
		t.Fatalf("get %q: %v", key, err)
	}
	if !ok {
		t.Fatalf("expected key %q to exist", key)
	}
	if string(got) != string(newValue) {
		t.Fatalf("value = %q, want %q", got, newValue)
	}
}

func TestRotateMemtable(t *testing.T) {
	e := &Engine{
		mutable: memtable.New(),
	}

	key := []byte("city")
	value := []byte("toronto")

	if err := e.mutable.Put(entry.New(key, value)); err != nil {
		t.Fatalf("put mutable: %v", err)
	}

	oldMutable := e.mutable

	if err := e.rotateMemTable(); err != nil {
		t.Fatalf("rotate memtable: %v", err)
	}

	if e.immutable != oldMutable {
		t.Fatalf("immutable memtable was not set to old mutable")
	}
	if e.mutable == nil {
		t.Fatalf("expected new mutable memtable")
	}
	if e.mutable == oldMutable {
		t.Fatalf("expected fresh mutable memtable after rotation")
	}

	if _, ok := e.mutable.Get(key); ok {
		t.Fatalf("expected fresh mutable memtable to be empty")
	}

	got, ok := e.immutable.Get(key)
	if !ok {
		t.Fatalf("expected key %q in immutable memtable after rotation", key)
	}
	if !bytes.Equal(got.Value, value) {
		t.Fatalf("immutable value = %q, want %q", got.Value, value)
	}
}
