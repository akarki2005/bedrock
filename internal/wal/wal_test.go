package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"github.com/akarki2005/lsm-engine/internal/entry"
)

const recordLenSize = 4

func TestOpenCreatesDirectoryAndFile(t *testing.T) {
	wal, path := openTestWAL(t)

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
		if !bytes.Equal(got[i].key, input[i].key) {
			t.Fatalf("entry %d key mismatch: got %q want %q", i, got[i].Key, input[i].Key)
		}
		if !bytes.Equal(got[i].value, input[i].value) {
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

}


func TestReplayWrapsCallbackError(t *testing.T) {

}


func TestAppendAfterCloseReturnsErrClosed(t *testing.T) {

}


func TestCloseIdempotency(t *testing.T) {

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