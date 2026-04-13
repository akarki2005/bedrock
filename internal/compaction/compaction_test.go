package compaction

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/sstable"
)

func mustEntry(t *testing.T, key, value string, ts int64) *entry.Entry {
	t.Helper()

	e := entry.NewWithTimestamp([]byte(key), []byte(value), ts)
	if e == nil {
		t.Fatalf("NewWithTimestamp returned nil")
	}

	return e
}

func mustTombstone(t *testing.T, key string, ts int64) *entry.Entry {
	t.Helper()

	e := entry.NewTombstoneWithTimestamp([]byte(key), ts)
	if e == nil {
		t.Fatalf("NewTombstone returned nil")
	}

	e.Timestamp = ts
	e.Checksum = checksumForTest(t, e)

	return e
}

func checksumForTest(t *testing.T, e *entry.Entry) uint32 {
	t.Helper()

	decoded, err := entry.Decode(e.Encode())
	if err != nil {
		t.Fatalf("decode encoded entry: %v", err)
	}

	return decoded.Checksum
}

func mustCreateTable(t *testing.T, dir, name string, entries []*entry.Entry) *sstable.SSTable {
	t.Helper()

	path := filepath.Join(dir, name)

	if err := sstable.CreateFromEntries(path, entries); err != nil {
		t.Fatalf("CreateFromEntries(%q): %v", path, err)
	}

	table, err := sstable.Open(path)
	if err != nil {
		t.Fatalf("Open(%q): %v", path, err)
	}

	return table
}

func TestRunNilPlan(t *testing.T) {
	_, err := Run(nil)
	if err == nil {
		t.Fatalf("expected error for nil plan")
	}
}

func TestMergeEntriesNoTables(t *testing.T) {
	got, err := mergeEntries(nil, nil)
	if err != nil {
		t.Fatalf("mergeEntries: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil result, got %v", got)
	}
}

func TestMergeEntriesMergesSortedKeysAcrossTables(t *testing.T) {
	dir := t.TempDir()

	t1 := mustCreateTable(t, dir, "a.db", []*entry.Entry{
		mustEntry(t, "apple", "1", 1),
		mustEntry(t, "carrot", "3", 3),
	})
	t2 := mustCreateTable(t, dir, "b.db", []*entry.Entry{
		mustEntry(t, "banana", "2", 2),
		mustEntry(t, "date", "4", 4),
	})

	got, err := mergeEntries([]*sstable.SSTable{t1}, []*sstable.SSTable{t2})
	if err != nil {
		t.Fatalf("mergeEntries: %v", err)
	}

	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(got))
	}

	wantKeys := []string{"apple", "banana", "carrot", "date"}
	for i, want := range wantKeys {
		if string(got[i].Key) != want {
			t.Fatalf("entry %d: expected key %q, got %q", i, want, got[i].Key)
		}
	}
}

func TestMergeEntriesKeepsLatestTimestampPerKey(t *testing.T) {
	dir := t.TempDir()

	older := mustCreateTable(t, dir, "older.db", []*entry.Entry{
		mustEntry(t, "apple", "old", 10),
		mustEntry(t, "banana", "banana", 20),
	})

	newer := mustCreateTable(t, dir, "newer.db", []*entry.Entry{
		mustEntry(t, "apple", "new", 99),
		mustEntry(t, "carrot", "carrot", 30),
	})

	got, err := mergeEntries([]*sstable.SSTable{older}, []*sstable.SSTable{newer})
	if err != nil {
		t.Fatalf("mergeEntries: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	if string(got[0].Key) != "apple" {
		t.Fatalf("expected first key apple, got %q", got[0].Key)
	}
	if string(got[0].Value) != "new" {
		t.Fatalf("expected latest value %q, got %q", "new", got[0].Value)
	}
	if got[0].Timestamp != 99 {
		t.Fatalf("expected latest timestamp 99, got %d", got[0].Timestamp)
	}
}

func TestMergeEntriesPreservesTombstoneWinner(t *testing.T) {
	dir := t.TempDir()

	live := mustCreateTable(t, dir, "live.db", []*entry.Entry{
		mustEntry(t, "apple", "value", 10),
	})

	deleted := mustCreateTable(t, dir, "deleted.db", []*entry.Entry{
		mustTombstone(t, "apple", 20),
	})

	got, err := mergeEntries([]*sstable.SSTable{live}, []*sstable.SSTable{deleted})
	if err != nil {
		t.Fatalf("mergeEntries: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if string(got[0].Key) != "apple" {
		t.Fatalf("expected key apple, got %q", got[0].Key)
	}
	if !got[0].Tombstone {
		t.Fatalf("expected tombstone winner")
	}
	if got[0].Timestamp != 20 {
		t.Fatalf("expected latest timestamp 20, got %d", got[0].Timestamp)
	}
}

func TestSplitOutputsEmpty(t *testing.T) {
	got := splitOutputs(0, nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSplitOutputsSingleChunkWhenUnderTarget(t *testing.T) {
	entries := []*entry.Entry{
		mustEntry(t, "a", "1", 1),
		mustEntry(t, "b", "2", 2),
		mustEntry(t, "c", "3", 3),
	}

	got := splitOutputs(0, entries)

	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
	if len(got[0]) != len(entries) {
		t.Fatalf("expected %d entries in chunk, got %d", len(entries), len(got[0]))
	}
}

func TestSplitOutputsSplitsWhenTargetExceeded(t *testing.T) {
	var entries []*entry.Entry

	for i := 0; i < 20000; i++ {
		keySuffix := string(rune('a' + (i % 26)))
		entries = append(entries, mustEntry(
			t,
			"key-"+keySuffix+"-"+strings.Repeat("k", 20),
			strings.Repeat("v", 100),
			int64(i+1),
		))
	}

	got := splitOutputs(0, entries)

	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(got))
	}

	total := 0
	for _, chunk := range got {
		total += len(chunk)
	}
	if total != len(entries) {
		t.Fatalf("expected %d total entries across chunks, got %d", len(entries), total)
	}
}

func TestTargetOutputFileSizeGrowthAndCap(t *testing.T) {
	tests := []struct {
		level int
		want  int
	}{
		{level: 0, want: 1 << 20},
		{level: 1, want: 2 << 20},
		{level: 2, want: 4 << 20},
		{level: 3, want: 8 << 20},
		{level: 4, want: 16 << 20},
		{level: 5, want: 16 << 20},
		{level: 6, want: 16 << 20},
	}

	for _, tt := range tests {
		got := targetOutputFileSize(tt.level)
		if got != tt.want {
			t.Fatalf("level %d: expected %d, got %d", tt.level, tt.want, got)
		}
	}
}

func TestRunUsesNextLevelForSplitTarget(t *testing.T) {
	dir := t.TempDir()

	var entries []*entry.Entry
	for i := 0; i < 15000; i++ {
		keySuffix := string(rune('a' + (i % 26)))
		entries = append(entries, mustEntry(
			t,
			"key-"+strings.Repeat("k", 24)+keySuffix,
			strings.Repeat("v", 100),
			int64(i+1),
		))
	}

	table := mustCreateTable(t, dir, "input.db", entries)

	plan := NewPlan(0, []*sstable.SSTable{table}, nil)

	got, err := Run(plan)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := splitOutputs(1, entries)

	if len(got) != len(want) {
		t.Fatalf("expected %d output chunks, got %d", len(want), len(got))
	}

	total := 0
	for _, chunk := range got {
		total += len(chunk)
	}
	if total != len(entries) {
		t.Fatalf("expected %d total entries, got %d", len(entries), total)
	}
}
