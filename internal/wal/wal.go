package wal

import (
	"sync"
	"os"
	"path/filepath"
	"github.com/akarki2005/lsm-engine/internal/entry"
	"fmt"
	"encoding/binary"
	"io"
	"errors"
)

const (
	PERMISSIONS_RW = 0o644
	PERMISSIONS_RWX = 0o755
)

var ErrClosed = errors.New("WAL is closed")

type WAL struct {
	mutex	sync.Mutex
	file 	*os.File
	path 	string
	closed	bool
}

func Open(path string) (*WAL, error) {
	dir := filepath.Dir(path)
	if d_err := os.MkdirAll(dir, PERMISSIONS_RWX); d_err != nil {
		return nil, fmt.Errorf("Create WAL directory %q: %w", dir, d_err)
	}

	file, f_err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, PERMISSIONS_RW)
	if f_err != nil {
		return nil, fmt.Errorf("Open WAL file %q: %w", path, f_err)
	}

	return &WAL{
		file: file,
		path: path,
	}, nil
}


func (wal *WAL) Append(e *entry.Entry) error {
	wal.mutex.Lock()
	defer wal.mutex.Unlock()

	if wal.closed { return ErrClosed}

	payload := e.Encode()

	// store payload length as metadata for each wal record
	var bufferLen [4]byte
	binary.LittleEndian.PutUint32(bufferLen[:], uint32(len(payload)))

	if _, err := wal.file.Write(bufferLen[:]); err != nil {
		return fmt.Errorf("write WAL record length: %w", err)
	}
	if _, err := wal.file.Write(payload); err != nil {
		return fmt.Errorf("write WAL record payload: %w", err)
	}

	return nil
}


func (wal *WAL) Replay(fn func(*entry.Entry) error) error {
	wal.mutex.Lock()
	defer wal.mutex.Unlock()

	if wal.closed { return ErrClosed }

	if _, err := wal.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("seek WAL start: %w", err)
	}
	defer wal.file.Seek(0, io.SeekEnd)

	var bufferLen [4]byte

	for {
		_, err := io.ReadFull(wal.file, bufferLen[:])

		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}

		if err != nil {
			return fmt.Errorf("read WAL record length: %w", err)
		}

		n := binary.LittleEndian.Uint32(bufferLen[:])
		if n == 0 {
			return fmt.Errorf("invalid WAL record length: 0")
		}

		payload := make([]byte, n)
		if _, err := io.ReadFull(wal.file, payload); err != nil {
			if err == io.ErrUnexpectedEOF {
				return nil
			}
			return fmt.Errorf("read WAL record payload: %w", err)
		}

		e, err := entry.Decode(payload)
		if err != nil {
			return fmt.Errorf("decode WAL record: %w", err)
		}

		if err := fn(e); err != nil {
			return fmt.Errorf("replay callback: %w", err)
		}
	}
}


func (wal *WAL) Close() error {
	wal.mutex.Lock()
	defer wal.mutex.Unlock()

	if wal.closed {return nil}

	if err := wal.file.Sync(); err != nil {
		return fmt.Errorf("sync WAL on close: %w", err)
	}

	if err := wal.file.Close(); err != nil {
		return fmt.Errorf("close WAL file: %w", err)
	}

	wal.closed = true

	return nil
}