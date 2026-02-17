package entry

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
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

	wantChecksum := checksumFor(e.Timestamp, key, value)
	if e.Checksum != wantChecksum {
		t.Fatalf("checksum mismatch: got %d want %d", e.Checksum, wantChecksum)
	}
}

func TestEncodeWritesExpectedBinaryLayout(t *testing.T) {
	e := &Entry{
		Key:       []byte("zayne"),
		Value:     []byte("parekh"),
		Timestamp: 1700000000,
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
	original := &Entry{
		Key:       []byte("zeev"),
		Value:     []byte("buium"),
		Timestamp: 1700000100,
	}
	original.Checksum = original.calculateChecksum()

	decoded, err := Decode(original.Encode())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if !bytes.Equal(decoded.Key, original.Key) {
		t.Fatalf("decoded key mismatch: got %q want %q", decoded.Key, original.Key)
	}
	if !bytes.Equal(decoded.Value, original.Value) {
		t.Fatalf("decoded value mismatch: got %q want %q", decoded.Value, original.Value)
	}
	if decoded.Timestamp != original.Timestamp {
		t.Fatalf("decoded timestamp mismatch: got %d want %d", decoded.Timestamp, original.Timestamp)
	}
	if decoded.Checksum != original.Checksum {
		t.Fatalf("decoded checksum mismatch: got %d want %d", decoded.Checksum, original.Checksum)
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

func TestChecksumChangesWhenPayloadChanges(t *testing.T) {
	a := &Entry{Key: []byte("connor"), Value: []byte("mcdavid"), Timestamp: 1700000300}
	b := &Entry{Key: []byte("connor"), Value: []byte("bedard"), Timestamp: 1700000300}

	if a.calculateChecksum() == b.calculateChecksum() {
		t.Fatal("expected checksum to change when value changes")
	}
}

func checksumFor(timestamp int64, key, value []byte) uint32 {
	buf := make([]byte, TimestampSize+len(key)+len(value))
	binary.LittleEndian.PutUint64(buf[:TimestampSize], uint64(timestamp))
	copy(buf[TimestampSize:TimestampSize+len(key)], key)
	copy(buf[TimestampSize+len(key):], value)
	return crc32.ChecksumIEEE(buf)
}
