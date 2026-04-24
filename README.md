<div align="center">

# Bedrock 

[![codecov](https://codecov.io/gh/akarki2005/bedrock/branch/main/graph/badge.svg)](https://codecov.io/gh/akarki2005/bedrock)

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
git clone git@github.com:akarki2005/bedrock.git
cd bedrock
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
go build -o bedrock-cli ./cmd/cli
```

Operations:

```bash
./bedrock-cli -path ./data put <key> <value>
./bedrock-cli -path ./data get <key>
./bedrock-cli -path ./data delete <key>
```