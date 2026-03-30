# LSM Storage Engine System Design

## Functional Requirements

- Support `Put(key, value)` to insert a new key-value pair or overwrite an existing value for a key
- Support `Get(key)` to retrieve the most recent value associated with a key
- Support `Delete(key)` to logically remove a key so that subsequent reads return not found
- `Delete(key)` is idempotent; after it succeeds, 
the key is considered absent regardless of whether it previously existed
- Use last-write-wins semantics, so the most recent `Put` or `Delete`
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
- The scope is limited to a single-node engine; replication, sharding and fault tolerance at a distributed level are out of scope

## API Design

## Core Components

## High-Level Architecture

## Deep Dive

## Tradeoffs

## Future Improvements