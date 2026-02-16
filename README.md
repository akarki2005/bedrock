# LSM Storage Engine

A log-structured merge-tree (LSM) storage engine for key-value data, implemented in Go.

## Design & Implementation

This project is an exploration of storage engine primitives, specifically focusing on the trade-offs between write throughput and read latency.

- **MemTable (Red-Black Tree)**: Uses a Red-Black Tree for in-memory indexing. I chose an RBT over a Skip List to enforce deterministic operation bounds, prioritizing tail latency over probabilistic average-case performance.

- **Write-Ahead Log (WAL)**: Every mutation is appended to a WAL before being committed to the MemTable. Uses `os.O_APPEND` and `f.Sync()` to ensure durability and crash-consistency.

- **SSTables**: Once the MemTable reaches its threshold, it is flushed to disk as an immutable Sorted String Table. Each SSTable includes a Bloom filter and a sparse index to minimize disk seeks during lookups.

- **Leveled Compaction**: Implements a leveled compaction strategy (similar to LevelDB). While this increases write amplification, it maintains strict bounds on read/space amplification by ensuring non-overlapping key ranges.

## Project Structure

`internal/tree/`: Red-Black Tree logic (State Machine).

`internal/wal/`: Append-only log management.

`internal/sstable/`: Binary serialization and disk-to-memory mapping.

`internal/compaction/`: Leveled merge-sort workers.

`package/engine/`: Main API entry points.

## Constraints & Trade-offs

**Zero Dependencies**: Built using only the Go standard library (os, io, binary).

**Byte-oriented**: All keys and values are handled as []byte to avoid GC overhead and unnecessary string allocations.

**Write Amplification**: The choice of leveled compaction assumes a workload where read performance and storage efficiency are more critical than raw write speed.