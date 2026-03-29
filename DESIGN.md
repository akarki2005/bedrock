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

## API Design

## Core Components

## High-Level Architecture

## Deep Dive

## Tradeoffs

## Future Improvements