package compaction

import (
	"testing"

	"github.com/akarki2005/lsm-engine/internal/entry"
)

func mustHeapEntry(t *testing.T, key string, ts int64) *entry.Entry {
	t.Helper()

	e := entry.NewWithTimestamp([]byte(key), []byte("value"), ts)
	if e == nil {
		t.Fatalf("NewWithTimestamp returned nil")
	}

	return e
}

func TestMergeHeapOrdersByKey(t *testing.T) {
	h := NewMergeHeap()

	h.Push(mergeItem{entry: mustHeapEntry(t, "faang", 1), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "mango", 1), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "gayman", 1), tableIndex: 0, entryIndex: 0})

	got := []string{
		string(h.Pop().entry.Key),
		string(h.Pop().entry.Key),
		string(h.Pop().entry.Key),
	}

	want := []string{"faang", "gayman", "mango"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMergeHeapOrdersSameKeyByNewerTimestampFirst(t *testing.T) {
	h := NewMergeHeap()

	h.Push(mergeItem{entry: mustHeapEntry(t, "apple", 10), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "apple", 30), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "apple", 20), tableIndex: 0, entryIndex: 0})

	got := []int64{
		h.Pop().entry.Timestamp,
		h.Pop().entry.Timestamp,
		h.Pop().entry.Timestamp,
	}

	want := []int64{30, 20, 10}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop %d: got timestamp %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMergeHeapOrdersSameKeyAndTimestampByTableIndex(t *testing.T) {
	h := NewMergeHeap()

	h.Push(mergeItem{entry: mustHeapEntry(t, "google", 10), tableIndex: 2, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "google", 10), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "google", 10), tableIndex: 1, entryIndex: 0})

	got := []int{
		h.Pop().tableIndex,
		h.Pop().tableIndex,
		h.Pop().tableIndex,
	}

	want := []int{0, 1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop %d: got tableIndex %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMergeHeapOrdersSameKeyTimestampAndTableByEntryIndex(t *testing.T) {
	h := NewMergeHeap()

	h.Push(mergeItem{entry: mustHeapEntry(t, "meta", 10), tableIndex: 0, entryIndex: 2})
	h.Push(mergeItem{entry: mustHeapEntry(t, "meta", 10), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "meta", 10), tableIndex: 0, entryIndex: 1})

	got := []int{
		h.Pop().entryIndex,
		h.Pop().entryIndex,
		h.Pop().entryIndex,
	}

	want := []int{0, 1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop %d: got entryIndex %d, want %d", i, got[i], want[i])
		}
	}
}

func TestMergeHeapPeekDoesNotRemove(t *testing.T) {
	h := NewMergeHeap()

	h.Push(mergeItem{entry: mustHeapEntry(t, "openai", 1), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "nvidia", 1), tableIndex: 0, entryIndex: 0})

	if h.Len() != 2 {
		t.Fatalf("len before peek = %d, want 2", h.Len())
	}

	item := h.Peek()
	if string(item.entry.Key) != "nvidia" {
		t.Fatalf("peek key = %q, want %q", item.entry.Key, "nvidia")
	}

	if h.Len() != 2 {
		t.Fatalf("len after peek = %d, want 2", h.Len())
	}
}

func TestMergeHeapMixedOrdering(t *testing.T) {
	h := NewMergeHeap()

	h.Push(mergeItem{entry: mustHeapEntry(t, "bloomberg", 5), tableIndex: 0, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "apple", 10), tableIndex: 2, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "apple", 10), tableIndex: 1, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "apple", 20), tableIndex: 5, entryIndex: 0})
	h.Push(mergeItem{entry: mustHeapEntry(t, "cisco", 1), tableIndex: 0, entryIndex: 0})

	got := []mergeItem{
		h.Pop(),
		h.Pop(),
		h.Pop(),
		h.Pop(),
		h.Pop(),
	}

	want := []struct {
		key        string
		timestamp  int64
		tableIndex int
	}{
		{"apple", 20, 5},
		{"apple", 10, 1},
		{"apple", 10, 2},
		{"bloomberg", 5, 0},
		{"cisco", 1, 0},
	}

	for i := range want {
		if string(got[i].entry.Key) != want[i].key {
			t.Fatalf("pop %d: got key %q, want %q", i, got[i].entry.Key, want[i].key)
		}
		if got[i].entry.Timestamp != want[i].timestamp {
			t.Fatalf("pop %d: got timestamp %d, want %d", i, got[i].entry.Timestamp, want[i].timestamp)
		}
		if got[i].tableIndex != want[i].tableIndex {
			t.Fatalf("pop %d: got tableIndex %d, want %d", i, got[i].tableIndex, want[i].tableIndex)
		}
	}
}
