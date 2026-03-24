package entry

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"time"
)

const (
	ChecksumSize  = 4
	TimestampSize = 8
	KeyLenSize    = 4
	ValLenSize    = 4
	TombstoneSize = 1
	HeaderSize    = ChecksumSize + TimestampSize + KeyLenSize + ValLenSize + TombstoneSize
)

type Entry struct {
	Key       []byte
	Value     []byte
	Timestamp int64
	Checksum  uint32
	Tombstone bool
}

func newEntry(key, value []byte, tombstone bool) *Entry {
	e := &Entry{
		Key:       key,
		Value:     value,
		Timestamp: time.Now().Unix(),
		Tombstone: tombstone,
	}
	e.Checksum = e.calculateChecksum()
	return e
}

func New(key, value []byte) *Entry {
	return newEntry(key, value, false)
}

func NewTombstone(key []byte) *Entry {
	return newEntry(key, nil, true)
}

// encodes the entry into a binary format for disk storage
func (e *Entry) Encode() []byte {
	keyLen := len(e.Key)
	valLen := len(e.Value)

	buffer := make([]byte, HeaderSize+keyLen+valLen)

	binary.LittleEndian.PutUint32(buffer[0:ChecksumSize], e.Checksum)

	timestampOffset := ChecksumSize
	binary.LittleEndian.PutUint64(buffer[timestampOffset:timestampOffset+TimestampSize], uint64(e.Timestamp))

	keyLenOffset := timestampOffset + TimestampSize
	binary.LittleEndian.PutUint32(buffer[keyLenOffset:keyLenOffset+KeyLenSize], uint32(keyLen))
	valLenOffset := keyLenOffset + KeyLenSize
	binary.LittleEndian.PutUint32(buffer[valLenOffset:valLenOffset+ValLenSize], uint32(valLen))

	tombstoneOffset := valLenOffset + ValLenSize
	if e.Tombstone {
		buffer[tombstoneOffset] = 1
	} else {
		buffer[tombstoneOffset] = 0
	}

	copy(buffer[HeaderSize:HeaderSize+keyLen], e.Key)
	copy(buffer[HeaderSize+keyLen:], e.Value)

	return buffer
}

// reconstructs an entry from binary, used when reading from disk.
func Decode(buffer []byte) (*Entry, error) {
	if len(buffer) < HeaderSize {
		return nil, errors.New("insufficient data for header")
	}

	checksum := binary.LittleEndian.Uint32(buffer[0:ChecksumSize])

	timestampOffset := ChecksumSize
	timestamp := binary.LittleEndian.Uint64(buffer[timestampOffset : timestampOffset+TimestampSize])

	keyLenOffset := timestampOffset + TimestampSize
	keyLen := binary.LittleEndian.Uint32(buffer[keyLenOffset : keyLenOffset+KeyLenSize])
	valLenOffset := keyLenOffset + KeyLenSize
	valLen := binary.LittleEndian.Uint32(buffer[valLenOffset : valLenOffset+ValLenSize])

	tombstoneOffset := valLenOffset + ValLenSize
	tombstone := buffer[tombstoneOffset] == 1

	if len(buffer) < HeaderSize+int(keyLen)+int(valLen) {
		return nil, errors.New("data truncated: either the key or value is missing")
	}

	key := buffer[HeaderSize : HeaderSize+int(keyLen)]
	value := buffer[HeaderSize+int(keyLen) : HeaderSize+int(keyLen)+int(valLen)]

	entry := &Entry{
		Key:       key,
		Value:     value,
		Timestamp: int64(timestamp),
		Checksum:  checksum,
		Tombstone: tombstone,
	}

	if entry.calculateChecksum() != checksum {
		return nil, errors.New("checksum mismatch: data corruption detected")
	}

	return entry, nil
}

// generates a checksum for each data record to ensure data integrity
func (e *Entry) calculateChecksum() uint32 {
	buffer := make([]byte, TimestampSize+TombstoneSize+len(e.Key)+len(e.Value))

	binary.LittleEndian.PutUint64(buffer[0:TimestampSize], uint64(e.Timestamp))

	if e.Tombstone {
		buffer[TimestampSize] = 1
	} else {
		buffer[TimestampSize] = 0
	}

	copy(buffer[TimestampSize+TombstoneSize:TimestampSize+TombstoneSize+len(e.Key)], e.Key)
	copy(buffer[TimestampSize+TombstoneSize+len(e.Key):], e.Value)

	return crc32.ChecksumIEEE(buffer)
}
