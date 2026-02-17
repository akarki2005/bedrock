# LSM Storage Engine

A log-structured merge-tree (LSM) storage engine for key-value data, implemented in Go.

## Design & Implementation

This project is an exploration of storage engine primitives, specifically focusing on the trade-offs between write throughput and read latency.

- **MemTable (Red-Black Tree)**: Uses a Red-Black Tree for in-memory indexing. I chose an RBT over a Skip List to enforce deterministic operation bounds, prioritizing tail latency over probabilistic average-case performance.

- **Write-Ahead Log (WAL)**: Every mutation is appended to a WAL before being committed to the MemTable. Uses `os.O_APPEND` and `f.Sync()` to ensure durability and crash-consistency.

- **SSTables**: Once the MemTable reaches its threshold, it is flushed to disk as an immutable Sorted String Table. Each SSTable includes a Bloom filter and a sparse index to minimize disk seeks during lookups.

- **Leveled Compaction**: Implements a leveled compaction strategy (similar to LevelDB). While this increases write amplification, it maintains strict bounds on read/space amplification by ensuring non-overlapping key ranges.

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
│   └── engine/            # API entry points
├── go.mod                 
└── README.md
```              

## Constraints & Trade-offs

**Zero Dependencies**: Built using only the Go standard library (os, io, binary).

**Byte-oriented**: All keys and values are handled as []byte to avoid GC overhead and unnecessary string allocations.

**Write Amplification**: The choice of leveled compaction assumes a workload where read performance and storage efficiency are more critical than raw write speed.

## Testing

All tests should be run from the root directory.

Run all tests:

```bash
go test ./...
```

Verbose output:

```bash
go test -v ./...
```

Run tests for a specific package:

```bash
go test ./{package_name}
```

Run a single test by name:

```bash
go test ./{package_name} -run {test_name}
```

### Quality checks

Run with race detector:

```bash
go test -race ./...
```

Collect coverage:

```bash
go test -cover ./...
```