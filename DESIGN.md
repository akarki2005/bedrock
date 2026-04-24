# Bedrock System Design

## Functional Requirements

- Support `Put(key, value)` to insert a new key-value pair or overwrite an existing value for a key
- Support `Get(key)` to retrieve the most recent value associated with a key
- Support `Delete(key)` to logically remove a key so that subsequent reads return not found
- `Delete(key)` is idempotent; after it succeeds, 
the key is considered absent regardless of whether it previously existed
- Use last-write-wins semantics, so the most recent `Put` or `Delete` for a key `k` determines the value returned by subsequent `Get(k)` operations
- Return not found when reading a key that has never existed or been deleted
- Guarantee durability for acknowledged writes: once `Put` or `Delete` returns success, the update must survive process crashes and be recoverable on restart.
- Recover database state on startup by replaying persisted write-ahead log records that were not yet flushed to disk tables.
- Range scans and ordered iteration are out of scope for v1.

## Non-Functional Requirements

- The engine should support datasets larger than available memory by persisting data to disk
- The engine should handle a sustained write-heavy workload efficiently on a single node
- The engine should continue serving reads and writes as in-memory data structures are flushed to immutable on-disk tables
- Optimize for write throughput rather than minimizing write amplification
- Favor sequential disk I/O over random disk I/O whenever possible
- Provide strong durability guarantees for acknowledged writes
- Crash recovery must be provided through WAL replay so that in-memory updates not yet flushed to SSTables are not lost
- The engine should provide relatively low latency for point reads, but accept read amplification as a tradeoff for better write performance
- The engine should use memory efficiently
- The implementation should prioritize simplicity, correctness and maintainability over advanced optimizations
- The scope is limited to a single-node engine; replication, sharding and fault tolerance at a distributed level are all out of scope

## API Design

### `Open(path string) (*Engine, error)`

Opens an existing database at the given filesystem path or creates a new one if none exists.

Behaviour:

- Initializes in-memory state
- Opens/creates the WAL
- Loads existing SSTables from disk
- Replays WAL records needed for crash recovery
- Returns a ready-to-use database handle

Error if:

- Path is invalid
- Required files cannot be opened
- On-disk state is corrupted
- WAL recovery fails

### `Put(key, value []byte) error`

Inserts a new key-value pair or overwrites the existing value for that key.

Behaviour:

- Appends the write to the WAL before acknowledging success
- Updates the memtable *after* the WAL append succeeds
- Uses last-write-wins semantics
- If `Put` returns `nil`, the write is durable and can survive process crash

Error if:

- Key is invalid
- Database is closed
- WAL append or sync fails

### `Get(key []byte) ([]byte, error)`

Returns the most recent value associated with the key.

Behaviour:

- Searches active memtable first, then checks immutable/flushing memtable, then finally searches SSTables from newest to oldest
- If newest visible record is a tombstone, return not found

### `Delete(key []byte) error`

Logically removes a key by recording a tombstone.

Behaviour:

- Appends a tombstone record to the WAL before acknowledging success
- Updates the memtable with the tombstone *after* WAL append succeeds
- Does not physically remove older values immediately
- Is idempotent
- If `Delete` returns `nil`, future reads should treat the key as absent, and that delete must survive a process crash

Error if:

- key is invalid
- Database is closed
- WAL append or sync fails

### `Close() error`

Shuts down the database and releases resources.

Behaviour:

- Prevents new operations from being accepted
- Flushes or finalizes any state required for a clean shutdown
- Closes open file handles

Error if:

- Pending state cannot be finalized correctly
- Files cannot be closed cleanly

## Core Components

The engine's intended v1 architecture consists of 5 main components: the Engine, the WAL, the memtable, and SSTables.

### Engine

The Engine, living in `engine.go`, is the top-level coordinator and public API entry point. It exposes operations such as `Open`, `Put`, `Get`, `Delete` and `Close`, and is responsible for coordinating interactions between the WAL, memtable, and on-disk SSTables. It enforces lifecycle constraints (e.g. preventing operations on a closed database) and defines overall read and write flows.

### Write-Ahead Log (WAL)

The WAL provides durability for all acknowledged writes. Each `Put` and `Delete` operation is appended to the WAL before being applied to the memtable. 

The WAL is append-only and uses sequential disk I/O. On startup, it is replayed to recover updates that were acknowledged but not yet flushed to disk tables, ensuring crash recovery.

### MemTable

The memtable is the active in-memory data structure that stores the most recent updates. It supports efficient point lookups and maintains last-write-wins semantics. 

Deletes are represented as tombstone entries rather than removing data in-place. Reads consult the memtable first, as it contains the most recent state.

### SSTables

SSTables are immutable on-disk tables created from memtable data. They allow the engine to persist data beyond memory limits and support datasets larger than RAM.

In the current implementation, SSTables are written using sequential I/O. Reads search SSTables from newest to oldest to locate the most recent version of a key.

## High-Level Architecture

The architecture follows a standard write-optimized LSM design that is built around the aforementioned core components. The engine is the top-level coordinator and API entry point. The WAL provides durability for all acknowledged writes. The memtable stores recent updates in memory for fast access, and SSTables provide immutable on-disk storage so the engine can support datasets larger than available memory. Together, these components seperate the system into a fast write path, a slower, layered read path, and a background persistence path.

### Write Path

All writes enter the system through the Engine. For both `Put` and `Delete`, the Engine first appends the update to the WAL before acknowledging success to the caller. This ordering is critical for durability: once the WAL append and sync succeed, the write can survive a process crash. After the WAL write succeeds, the Engine applies the update to the active memtable. A `Put` inserts or overwrites the key with a new value, while a `Delete` inserts a tombstone entry that logically marks the key as absent without physically removing older versions from storage. This design preserves last-write-wins semantics while keeping the write path append-only and thus sequential on disk.

As the memtable grows large, it eventually reaches a configured size threshold. At that point, the active memtable is frozen and becomes immutable - it is now waiting to be flushed to disk. A fresh memtable is created in the meantime to continue accepting new writes. This handoff prevents foreground writes from being blocked by disk persistence work. In the background, the immutable memtable is flushed to a new SSTable on disk. Because SSTables are written sequentially and never modified in place, the system avoids random disk writes and maintains its write-optimized design.

### Read Path

Reads also enter through the Engine and follow a layered lookup path from newest state to oldest state. The engine first checks the active mutable memtable, since it contains the most recent in-memory updates. If the key is not found there, the Engine checks the immutable/flushing memtable, if one exists. If the key is still not found, the Engine searches SSTables from newest to oldest. This lookup order ensures that the first visible version of a key is the correct one under last-write-wins semantics.

Tombstones are respected throughout the entire read path. If the newest visible record for a key is a tombstone, the read returns not found, even if older SSTables still contain previous values for that key. This allows deletes to remain logical and append-only while still producing correct results. The cost of this design is some read amplification, since a lookup may need to check multiple layers before concluding whether a key exists, but this is an intentional tradeoff in favour of higher write throughput and simpler persistence.

### Recovery Path

When the database is opened, the Engine intitializes in-memory state, opens or creates the WAL, loads any existing SSTables from disk, and replays WAL records needed for crash recovery. WAL replay restores updates that were acknowledged but not yet flushed to disk tables before the previous process terminated. This ensures that acknowledged writes are not lost even if a crash occurs after the WAL write but before memtable contents have been persisted as an SSTable. Once recovery completes, the Engine returns a ready-to-use database handle.

### Architectural Summary

At a high level, the system seperates responsibilites cleanly across storage layers. The WAL guarantees durability, the memtable absorbs recent writes and serves the freshest in-memory state, immutable memtables enable non-blocking background flushes, and SSTables provide scalable persistent storage on disk. The result is a single-node storage engine that favors sequential I/O, high write throughput, and crash recovery correctness, while accepting read amplification as a tradeoff.

## Deep Dive

## Tradeoffs

### Write Optimization vs Read Amplification

- Optimized for sequential writes (WAL + MemTable) -> low write latency and high write throughput
- Reads may check multiple structures (MemTable + SSTables) -> higher read latency & amplification

### Append-Only Design vs Storage Overhead

- No in-place updates -> avoids random I/O
- Old versions + tombstones accumulate -> higher disk utilization

### Durability vs Latency

- WAL fsync before acknowledgement -> strong durability guarantees
- Adds additional overhead latency to each write

## Future Improvements

### Compaction Engine

- Merge SSTables, remove stale entries and tombstones
- Reduces both read and space amplification

### Bloom Filters

- Quickly determine whether or not a key is absent in an SSTable
- Avoid unnecessary disk reads

### Sparse Indexes

- Enable faster lookups within SSTables
- Reduce the need for full scans

### Atomic SSTable 

- Use a temporary file + rename
- Prevent partial/corrupt table states

### Async/Batched WAL Sync

- Group multiple writes into one fsync call
- Improve write throughput through amortization

### Range Scans

- Merge results across MemTable and SSTables
- Enable order traversal of keys