package memtable

import (
	"bytes"
	"errors"
	"testing"

	"github.com/akarki2005/lsm-engine/internal/entry"
)

func TestNewInitializesEmptyMemTable(t *testing.T) {
	mt := New()

	if mt == nil {
		t.Fatal("expected non-nil memtable")
	}
	if mt.head == nil {
		t.Fatal("expected non-nil head node")
	}
	if mt.level != 1 {
		t.Fatalf("initial level mismatch: got %d want %d", mt.level, 1)
	}
	if mt.count != 0 {
		t.Fatalf("initial size mismatch: got %d want %d", mt.count, 0)
	}
	if len(mt.head.successor) != maxLevel {
		t.Fatalf("head successor length mismatch: got %d want %d", len(mt.head.successor), maxLevel)
	}
}

func TestPutCreatesNewEntry(t *testing.T) {
	mt := New()

	e := entry.New([]byte("utah"), []byte("mammoth"))
	if err := mt.Put(e); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if got, want := mt.Len(), 1; got != want {
		t.Fatalf("length mismatch after insert: got %d want %d", got, want)
	}

	got, ok := mt.Get([]byte("utah"))
	if !ok {
		t.Fatal("expected inserted key to exist")
	}
	if !bytes.Equal(got.Key, e.Key) {
		t.Fatalf("key mismatch: got %q want %q", got.Key, e.Key)
	}
	if !bytes.Equal(got.Value, e.Value) {
		t.Fatalf("value mismatch: got %q want %q", got.Value, e.Value)
	}
}

func TestPutReplacesExistingValue(t *testing.T) {
	mt := New()

	first := entry.New([]byte("utah"), []byte("hockey club"))
	second := entry.New([]byte("utah"), []byte("mammoth"))

	if err := mt.Put(first); err != nil {
		t.Fatalf("first Put failed: %v", err)
	}
	if err := mt.Put(second); err != nil {
		t.Fatalf("second Put failed: %v", err)
	}

	if got, want := mt.Len(), 1; got != want {
		t.Fatalf("length mismatch after overwrite: got %d want %d", got, want)
	}

	got, ok := mt.Get([]byte("utah"))
	if !ok {
		t.Fatal("expected overwritten key to exist")
	}
	if !bytes.Equal(got.Value, second.Value) {
		t.Fatalf("value mismatch after overwrite: got %q want %q", got.Value, second.Value)
	}
}

func TestGetReturnsEntryForExistingKey(t *testing.T) {
	mt := New()

	e := entry.New([]byte("winnipeg"), []byte("jets"))
	if err := mt.Put(e); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	got, ok := mt.Get([]byte("winnipeg"))
	if !ok {
		t.Fatal("expected key to exist")
	}
	if !bytes.Equal(got.Key, e.Key) {
		t.Fatalf("key mismatch: got %q want %q", got.Key, e.Key)
	}
	if !bytes.Equal(got.Value, e.Value) {
		t.Fatalf("value mismatch: got %q want %q", got.Value, e.Value)
	}
}

func TestGetReturnsFalseForMissingKey(t *testing.T) {
	mt := New()

	got, ok := mt.Get([]byte("arizona")) // the coyotes are no longer a team!
	if ok {
		t.Fatalf("expected missing key to return false, got entry %+v", got)
	}
	if got != nil {
		t.Fatalf("expected nil entry for missing key, got %+v", got)
	}
}

func TestScanReturnsEntriesInSortedOrder(t *testing.T) {
	mt := New()

	input := []*entry.Entry{
		entry.New([]byte("vancouver"), []byte("canucks")),
		entry.New([]byte("calgary"), []byte("flames")),
		entry.New([]byte("edmonton"), []byte("oilers")),
	}

	for _, e := range input {
		if err := mt.Put(e); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	var gotKeys [][]byte
	var gotValues [][]byte

	if err := mt.Scan(func(e *entry.Entry) error {
		gotKeys = append(gotKeys, append([]byte(nil), e.Key...))
		gotValues = append(gotValues, append([]byte(nil), e.Value...))
		return nil
	}); err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	wantKeys := [][]byte{
		[]byte("calgary"),
		[]byte("edmonton"),
		[]byte("vancouver"),
	}
	wantValues := [][]byte{
		[]byte("flames"),
		[]byte("oilers"),
		[]byte("canucks"),
	}

	if len(gotKeys) != len(wantKeys) {
		t.Fatalf("scan count mismatch: got %d want %d", len(gotKeys), len(wantKeys))
	}

	for i := range wantKeys {
		if !bytes.Equal(gotKeys[i], wantKeys[i]) {
			t.Fatalf("scan key %d mismatch: got %q want %q", i, gotKeys[i], wantKeys[i])
		}
		if !bytes.Equal(gotValues[i], wantValues[i]) {
			t.Fatalf("scan value %d mismatch: got %q want %q", i, gotValues[i], wantValues[i])
		}
	}
}

func TestScanWrapsCallbackError(t *testing.T) {
	mt := New()

	if err := mt.Put(entry.New([]byte("sid"), []byte("crosby"))); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	want := errors.New("callback failed")

	err := mt.Scan(func(e *entry.Entry) error {
		return want
	})
	if err == nil {
		t.Fatal("expected Scan to fail")
	}

	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped callback error, got %v", err)
	}

	if got, wantMsg := err.Error(), "scan callback: callback failed"; got != wantMsg {
		t.Fatalf("unexpected error message: got %q want %q", got, wantMsg)
	}
}

func TestPutGetPreservesTombstone(t *testing.T) {
	m := New()

	e := entry.NewTombstone([]byte("calgary"))
	if err := m.Put(e); err != nil {
		t.Fatalf("put failed: %v", err)
	}

	got, ok := m.Get([]byte("calgary"))
	if !ok {
		t.Fatal("expected key to be present")
	}
	if !got.Tombstone {
		t.Fatal("expected tombstone to be preserved")
	}
}
