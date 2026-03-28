package entry

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io"
	"testing"
	"time"
)

func TestNewInitializesTimestampAndChecksum(t *testing.T) {
	key := []byte("macklin")
	value := []byte("celebrini")
	before := time.Now().Unix()

	e := New(key, value)

	if !bytes.Equal(e.Key, key) {
		t.Fatalf("key mismatch: got %q want %q", e.Key, key)
	}
	if !bytes.Equal(e.Value, value) {
		t.Fatalf("value mismatch: got %q want %q", e.Value, value)
	}
	if e.Timestamp < before || e.Timestamp > time.Now().Unix() {
		t.Fatalf("timestamp out of expected range: got %d", e.Timestamp)
	}

	wantChecksum := checksumFor(e.Timestamp, false, key, value)
	if e.Checksum != wantChecksum {
		t.Fatalf("checksum mismatch: got %d want %d", e.Checksum, wantChecksum)
	}
}

func TestEncodeWritesExpectedBinaryLayout(t *testing.T) {
	e := &Entry{
		Key:       []byte("zayne"),
		Value:     []byte("parekh"),
		Timestamp: 1700000000,
		Tombstone: false,
	}
	e.Checksum = e.calculateChecksum()

	encoded := e.Encode()

	if len(encoded) != HeaderSize+len(e.Key)+len(e.Value) {
		t.Fatalf("encoded length mismatch: got %d want %d", len(encoded), HeaderSize+len(e.Key)+len(e.Value))
	}

	gotChecksum := binary.LittleEndian.Uint32(encoded[0:ChecksumSize])
	if gotChecksum != e.Checksum {
		t.Fatalf("encoded checksum mismatch: got %d want %d", gotChecksum, e.Checksum)
	}

	tsStart := ChecksumSize
	gotTimestamp := int64(binary.LittleEndian.Uint64(encoded[tsStart : tsStart+TimestampSize]))
	if gotTimestamp != e.Timestamp {
		t.Fatalf("encoded timestamp mismatch: got %d want %d", gotTimestamp, e.Timestamp)
	}

	keyLenStart := tsStart + TimestampSize
	gotKeyLen := binary.LittleEndian.Uint32(encoded[keyLenStart : keyLenStart+KeyLenSize])
	if int(gotKeyLen) != len(e.Key) {
		t.Fatalf("encoded key length mismatch: got %d want %d", gotKeyLen, len(e.Key))
	}

	valLenStart := keyLenStart + KeyLenSize
	gotValLen := binary.LittleEndian.Uint32(encoded[valLenStart : valLenStart+ValLenSize])
	if int(gotValLen) != len(e.Value) {
		t.Fatalf("encoded value length mismatch: got %d want %d", gotValLen, len(e.Value))
	}

	tombstoneStart := valLenStart + ValLenSize
	gotTombstone := encoded[tombstoneStart]
	if gotTombstone != 0 {
		t.Fatalf("encoded tombstone mistmatch: got %d want %d", gotTombstone, 0)
	}

	gotKey := encoded[HeaderSize : HeaderSize+len(e.Key)]
	gotVal := encoded[HeaderSize+len(e.Key):]
	if !bytes.Equal(gotKey, e.Key) {
		t.Fatalf("encoded key payload mismatch: got %q want %q", gotKey, e.Key)
	}
	if !bytes.Equal(gotVal, e.Value) {
		t.Fatalf("encoded value payload mismatch: got %q want %q", gotVal, e.Value)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		e    *Entry
	}{
		{
			name: "normal",
			e: &Entry{
				Key:       []byte("zeev"),
				Value:     []byte("buium"),
				Timestamp: 1700000100,
				Tombstone: false,
			},
		},
		{
			name: "tombstone",
			e: &Entry{
				Key:       []byte("zeev"),
				Value:     nil,
				Timestamp: 1700000100,
				Tombstone: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.e.Checksum = tt.e.calculateChecksum()

			decoded, err := Decode(tt.e.Encode())
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			if !bytes.Equal(decoded.Key, tt.e.Key) {
				t.Fatalf("decoded key mismatch: got %q want %q", decoded.Key, tt.e.Key)
			}
			if !bytes.Equal(decoded.Value, tt.e.Value) {
				t.Fatalf("decoded value mismatch: got %q want %q", decoded.Value, tt.e.Value)
			}
			if decoded.Timestamp != tt.e.Timestamp {
				t.Fatalf("decoded timestamp mismatch: got %d want %d", decoded.Timestamp, tt.e.Timestamp)
			}
			if decoded.Checksum != tt.e.Checksum {
				t.Fatalf("decoded checksum mismatch: got %d want %d", decoded.Checksum, tt.e.Checksum)
			}
			if decoded.Tombstone != tt.e.Tombstone {
				t.Fatalf("decoded tombstone mismatch: got %v want %v", decoded.Tombstone, tt.e.Tombstone)
			}
		})
	}
}

func TestDecodeRejectsShortHeader(t *testing.T) {
	_, err := Decode(make([]byte, HeaderSize-1))
	if err == nil {
		t.Fatal("expected error for short header, got nil")
	}
}

func TestDecodeRejectsTruncatedPayload(t *testing.T) {
	e := &Entry{
		Key:       []byte("cale"),
		Value:     []byte("makar"),
		Timestamp: 1700000200,
	}
	e.Checksum = e.calculateChecksum()
	encoded := e.Encode()

	_, err := Decode(encoded[:len(encoded)-1])
	if err == nil {
		t.Fatal("expected error for truncated payload, got nil")
	}
}

func TestWriteToWritesEncodedEntry(t *testing.T) {
	key := []byte("auston")
	value := []byte("matthews")

	e := &Entry{
		Key:       key,
		Value:     value,
		Timestamp: 1700000400,
		Tombstone: false,
	}
	e.Checksum = e.calculateChecksum()

	var buf bytes.Buffer
	if err := e.WriteTo(&buf); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	if !bytes.Equal(buf.Bytes(), e.Encode()) {
		t.Fatal("written bytes do not match encoded entry")
	}
}

func TestReadFromRoundTrip(t *testing.T) {
	key := []byte("william")
	value := []byte("nylander")

	e := &Entry{
		Key:       key,
		Value:     value,
		Timestamp: 1700000500,
		Tombstone: false,
	}
	e.Checksum = e.calculateChecksum()

	var buf bytes.Buffer
	if err := e.WriteTo(&buf); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := ReadFrom(&buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if !bytes.Equal(got.Key, e.Key) {
		t.Fatalf("key mismatch: got %q want %q", got.Key, e.Key)
	}
	if !bytes.Equal(got.Value, e.Value) {
		t.Fatalf("value mismatch: got %q want %q", got.Value, e.Value)
	}
	if got.Timestamp != e.Timestamp {
		t.Fatalf("timestamp mismatch: got %d want %d", got.Timestamp, e.Timestamp)
	}
	if got.Checksum != e.Checksum {
		t.Fatalf("checksum mismatch: got %d want %d", got.Checksum, e.Checksum)
	}
	if got.Tombstone != e.Tombstone {
		t.Fatalf("tombstone mismatch: got %v want %v", got.Tombstone, e.Tombstone)
	}
}

func TestReadFromReturnsEOFForEmptyReader(t *testing.T) {
	var buf bytes.Buffer

	_, err := ReadFrom(&buf)
	if err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestChecksumChangesWhenPayloadChanges(t *testing.T) {
	a := &Entry{Key: []byte("connor"), Value: []byte("mcdavid"), Timestamp: 1700000300}
	b := &Entry{Key: []byte("connor"), Value: []byte("bedard"), Timestamp: 1700000300}

	if a.calculateChecksum() == b.calculateChecksum() {
		t.Fatal("expected checksum to change when value changes")
	}
}

func checksumFor(timestamp int64, tombstone bool, key, value []byte) uint32 {
	buf := make([]byte, TimestampSize+TombstoneSize+len(key)+len(value))
	binary.LittleEndian.PutUint64(buf[:TimestampSize], uint64(timestamp))

	if tombstone {
		buf[TimestampSize] = 1
	} else {
		buf[TimestampSize] = 0
	}

	copy(buf[TimestampSize+TombstoneSize:TimestampSize+TombstoneSize+len(key)], key)
	copy(buf[TimestampSize+TombstoneSize+len(key):], value)
	return crc32.ChecksumIEEE(buf)
}
