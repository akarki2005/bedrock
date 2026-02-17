package entry

import (
	"encoding/binary"
	"hash/crc32"
	"time"
	"errors"
)

const (
	ChecksumSize = 4
	TimestampSize = 8
	KeyLenSize = 4
	ValLenSize = 4
	HeaderSize = ChecksumSize + TimestampSize + KeyLenSize + ValLenSize
)

type Entry struct {
	Key			[]byte
	Value		[]byte
	Timestamp	int64
	Checksum	uint32
}

// initializes a database record/entry with timestamp and checksum
func New(key, value []byte) *Entry {
	e := &Entry{
		Key: key,
		Value: value,
		Timestamp: time.Now().Unix(),
	}
	e.Checksum = e.calculateChecksum()
	return e
}

// encodes the entry into a binary format for disk storage
func (e *Entry) Encode() []byte {
	keyLen := len(e.Key)
	valLen := len(e.Value)

	buffer := make([]byte, HeaderSize + keyLen + valLen)

	binary.LittleEndian.PutUint32(buffer[0:ChecksumSize], e.Checksum)

	timestampOffset := ChecksumSize
	binary.LittleEndian.PutUint64(buffer[timestampOffset:timestampOffset + TimestampSize], uint64(e.Timestamp))

	keyLenOffset := timestampOffset + TimestampSize
	binary.LittleEndian.PutUint32(buffer[keyLenOffset:keyLenOffset + KeyLenSize], uint32(keyLen))
	valLenOffset := keyLenOffset + KeyLenSize
	binary.LittleEndian.PutUint32(buffer[valLenOffset:valLenOffset + ValLenSize], uint32(valLen))

	copy(buffer[HeaderSize:HeaderSize + keyLen], e.Key)
	copy(buffer[HeaderSize + keyLen:], e.Value)

	return buffer
}

// reconstructs an entry from binary, used when reading from disk.
func Decode(buffer []byte) (*Entry, error) {
	if len(buffer) < HeaderSize {
		return nil, errors.New("insufficient data for header")
	}

	checksum := binary.LittleEndian.Uint32(buffer[0:ChecksumSize])

	timestampOffset := ChecksumSize
	timestamp := binary.LittleEndian.Uint64(buffer[timestampOffset:timestampOffset + TimestampSize])

	keyLenOffset := timestampOffset + TimestampSize
	keyLen := binary.LittleEndian.Uint32(buffer[keyLenOffset:keyLenOffset + KeyLenSize])
	valLenOffset := keyLenOffset + KeyLenSize
	valLen := binary.LittleEndian.Uint32(buffer[valLenOffset:valLenOffset + ValLenSize])

	if len(buffer) < HeaderSize + int(keyLen) + int(valLen) {
		return nil, errors.New("data truncated: either the key or value is missing")
	}

	key := buffer[HeaderSize:HeaderSize + int(keyLen)]
	value := buffer[HeaderSize + int(keyLen):HeaderSize + int(keyLen) + int(valLen)]

	entry := &Entry{
		Key: key,
		Value: value,
		Timestamp: int64(timestamp),
		Checksum: checksum,
	}

	if entry.calculateChecksum() != checksum {
		return nil, errors.New("checksum mismatch: data corruption detected")
	}

	return entry, nil
}

// generates a checksum for each data record to ensure data integrity
func (e *Entry) calculateChecksum() uint32 {
	buffer := make([]byte, TimestampSize + len(e.Key) + len(e.Value))
	
	binary.LittleEndian.PutUint64(buffer[0:TimestampSize], uint64(e.Timestamp))
	
	copy(buffer[TimestampSize:TimestampSize + len(e.Key)], e.Key)
	copy(buffer[TimestampSize + len(e.Key):], e.Value)
	
	return crc32.ChecksumIEEE(buffer)
}