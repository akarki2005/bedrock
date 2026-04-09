package compaction

import (
	"bytes"
	"container/heap"

	"github.com/akarki2005/lsm-engine/internal/entry"
)

type mergeItem struct {
	entry      *entry.Entry
	tableIndex int
	entryIndex int
}

type mergeItemHeap []mergeItem

func (h mergeItemHeap) Len() int { return len(h) }

func (h mergeItemHeap) Less(i, j int) bool {
	cmp := bytes.Compare(h[i].entry.Key, h[j].entry.Key)
	if cmp != 0 {
		return cmp < 0
	}

	if h[i].entry.Timestamp != h[j].entry.Timestamp {
		return h[i].entry.Timestamp > h[j].entry.Timestamp
	}

	if h[i].tableIndex != h[j].tableIndex {
		return h[i].tableIndex < h[j].tableIndex
	}

	return h[i].entryIndex < h[j].entryIndex
}

func (h mergeItemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *mergeItemHeap) Push(x any) {
	*h = append(*h, x.(mergeItem))
}

func (h *mergeItemHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

type MergeHeap struct {
	items mergeItemHeap
}

func NewMergeHeap() *MergeHeap {
	h := &MergeHeap{
		items: make(mergeItemHeap, 0),
	}
	heap.Init(&h.items)
	return h
}

func (h *MergeHeap) Len() int {
	return h.items.Len()
}

func (h *MergeHeap) Push(item mergeItem) {
	heap.Push(&h.items, item)
}

func (h *MergeHeap) Pop() mergeItem {
	return heap.Pop(&h.items).(mergeItem)
}

func (h *MergeHeap) Peek() mergeItem {
	return h.items[0]
}
