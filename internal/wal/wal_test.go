package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/akarki2005/lsm-engine/internal/entry"
)

const recordLenSize = 4

func TestOpenCreatesDirectoryAndFile(t *testing.T) {
	_, path := openTestWAL(t)

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected WAL file to exist: %v", err)
	}
}

func TestAppendWritesLengthPrefixedPayload(t *testing.T) {
	wal, path := openTestWAL(t)

	e := entry.New([]byte("tage"), []byte("thompson"))
	if err := wal.Append(e); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(raw) < recordLenSize {
		t.Fatalf("WAL too short: got %d bytes", len(raw))
	}

	n := binary.LittleEndian.Uint32(raw[:recordLenSize])
	payload := e.Encode()

	if int(n) != len(payload) {
		t.Fatalf("record length mismatch: got %d want %d", n, len(payload))
	}
	if !bytes.Equal(raw[recordLenSize:], payload) {
		t.Fatalf("payload mismatch")
	}
}

func TestReplayReturnsEntriesInOrder(t *testing.T) {
	wal, _ := openTestWAL(t)

	input := []*entry.Entry{
		entry.New([]byte("evan"), []byte("bouchard")),
		entry.New([]byte("jake"), []byte("sanderson")),
		entry.New([]byte("rasmus"), []byte("dahlin")),
	}

	for _, e := range input {
		if err := wal.Append(e); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	var got []*entry.Entry
	// populate got with decoded entries from the wal
	if err := wal.Replay(func(e *entry.Entry) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if len(got) != len(input) {
		t.Fatalf("replayed count mismatch: got %d want %d", len(got), len(input))
	}

	for i := range input {
		if !bytes.Equal(got[i].Key, input[i].Key) {
			t.Fatalf("entry %d key mismatch: got %q want %q", i, got[i].Key, input[i].Key)
		}
		if !bytes.Equal(got[i].Value, input[i].Value) {
			t.Fatalf("entry %d value mismatch: got %q want %q", i, got[i].Value, input[i].Value)
		}
	}
}

func TestReplayFromEmptyWALCallsNothing(t *testing.T) {
	wal, _ := openTestWAL(t)

	count := 0
	if err := wal.Replay(func(e *entry.Entry) error {
		count++
		return nil
	}); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if count != 0 {
		t.Fatalf("got %d callbacks expected 0", count)
	}
}

func TestReplayErrorsOnZeroLengthRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.WriteFile(path, []byte{0, 0, 0, 0}, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	err = w.Replay(func(e *entry.Entry) error {
		t.Fatalf("callback should not be invoked for corrupt zero-length record")
		return nil
	})
	if err == nil {
		t.Fatal("expected replay to fail on zero-length record")
	}

	if got, want := err.Error(), "invalid WAL record length: 0"; got != want {
		t.Fatalf("unexpected error: got %q want %q", got, want)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestReplayWrapsCallbackError(t *testing.T) {
	w, _ := openTestWAL(t)

	if err := w.Append(entry.New([]byte("connor"), []byte("mcdavid"))); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	want := errors.New("callback failed")

	err := w.Replay(func(e *entry.Entry) error {
		return want
	})
	if err == nil {
		t.Fatal("expected replay to return callback error")
	}

	if !errors.Is(err, want) {
		t.Fatalf("expected replay error to wrap callback error: got %v", err)
	}

	if got, wantMsg := err.Error(), "replay callback: callback failed"; got != wantMsg {
		t.Fatalf("unexpected error message: got %q want %q", got, wantMsg)
	}
}

func TestAppendAfterCloseReturnsErrClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	err = w.Append(entry.New([]byte("auston"), []byte("matthews")))
	if err == nil {
		t.Fatal("expected append after close to fail")
	}

	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func TestCloseIdempotency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wal.log")

	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	err = w.Append(entry.New([]byte("auston"), []byte("matthews")))
	if err == nil {
		t.Fatal("expected append after close to fail")
	}

	if !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}

func openTestWAL(t *testing.T) (*WAL, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nested", "wal.log")
	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	t.Cleanup(func() {
		if err := w.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})

	return w, path
}
