# LSM Storage Engine

A log-structured merge-tree (LSM) storage engine for key-value data, implemented in Go.

## Project Structure

```text
.
├── cmd/
│   └── lsm-cli/           # CLI for testing/debugging
├── internal/              
│   ├── compaction/        # Background workers for merges
│   ├── entry/             # Serialization/deserialization
│   ├── memtable/          # In-memory state
│   ├── sstable/           # Disk-to-memory mapping
│   └── wal/               # Append-only write-ahead log
├── pkg/                   
│   └── engine/            # API entry point
├── go.mod                 
└── README.md
```    

## Installation

```bash
git clone git@github.com:akarki2005/lsm-engine.git
cd lsm-engine
```

## Testing

Run all tests:

```bash
go test ./...
```

Verbose output:

```bash
go test -v ./...
```

## CLI Usage

```bash
go run ./cmd/cli -path ./data put <key> <value>
go run ./cmd/cli -path ./data get <key>
```