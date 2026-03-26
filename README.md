<div align="center">

# LSM Storage Engine   

[![codecov](https://codecov.io/gh/akarki2005/lsm-engine/branch/main/graph/badge.svg)](https://codecov.io/gh/akarki2005/lsm-engine)

A log-structured merge-tree (LSM) storage engine for key-value data, written in Go.

</div>

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

Build:

```bash
go build -o lsm-cli ./cmd/cli
```

Operations:

```bash
./lsm-cli -path ./data put <key> <value>
./lsm-cli -path ./data get <key>
./lsm-cli -path ./data delete <key>
```