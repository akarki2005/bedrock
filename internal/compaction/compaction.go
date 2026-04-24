package compaction

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/akarki2005/bedrock/internal/entry"
	"github.com/akarki2005/bedrock/internal/sstable"
)

const baseSSTableSizeBytes = 1 << 20
const maxSSTableSizeBytes = 16 << 20

type Plan struct {
	level    int
	inputs   []*sstable.SSTable
	overlaps []*sstable.SSTable
}

func NewPlan(level int, inputs, overlaps []*sstable.SSTable) *Plan {
	return &Plan{
		level:    level,
		inputs:   inputs,
		overlaps: overlaps,
	}
}

func (p *Plan) Level() int                   { return p.level }
func (p *Plan) Inputs() []*sstable.SSTable   { return p.inputs }
func (p *Plan) Overlaps() []*sstable.SSTable { return p.overlaps }

func Run(plan *Plan) ([][]*entry.Entry, error) {
	if plan == nil {
		return nil, errors.New("nil compaction plan")
	}

	entries, err := mergeEntries(plan.inputs, plan.overlaps)
	if err != nil {
		return nil, fmt.Errorf("merge entries: %w", err)
	}

	return splitOutputs(plan.level+1, entries), nil
}

// https://leetcode.com/problems/merge-k-sorted-lists/
func mergeEntries(inputs []*sstable.SSTable, overlaps []*sstable.SSTable) ([]*entry.Entry, error) {
	allTables := make([]*sstable.SSTable, 0, len(inputs)+len(overlaps))
	allTables = append(allTables, inputs...)
	allTables = append(allTables, overlaps...)

	if len(allTables) == 0 {
		return nil, nil
	}

	tableEntries := make([][]*entry.Entry, len(allTables))
	for i, table := range allTables {
		entries := make([]*entry.Entry, 0)

		err := table.Scan(func(e *entry.Entry) error {
			entries = append(entries, entry.CloneEntry(e))
			return nil
		})
		if err != nil {
			return nil, err
		}
		tableEntries[i] = entries
	}

	h := NewMergeHeap()

	for tableIdx, entries := range tableEntries {
		if len(entries) == 0 {
			continue
		}

		h.Push(mergeItem{
			entry:      entries[0],
			tableIndex: tableIdx,
			entryIndex: 0,
		})
	}

	merged := make([]*entry.Entry, 0)

	for h.Len() > 0 {
		first := h.Pop()
		key := first.entry.Key
		winner := first.entry

		sameKeyItems := []mergeItem{first}

		for h.Len() > 0 && bytes.Equal(h.Peek().entry.Key, key) {
			item := h.Pop()
			sameKeyItems = append(sameKeyItems, item)

			if item.entry.Timestamp > winner.Timestamp {
				winner = item.entry
			}
		}

		merged = append(merged, entry.CloneEntry(winner))

		for _, item := range sameKeyItems {
			nextIndex := item.entryIndex + 1
			if nextIndex >= len(tableEntries[item.tableIndex]) {
				continue
			}

			h.Push(mergeItem{
				entry:      tableEntries[item.tableIndex][nextIndex],
				tableIndex: item.tableIndex,
				entryIndex: nextIndex,
			})
		}
	}

	return merged, nil
}

func splitOutputs(level int, entries []*entry.Entry) [][]*entry.Entry {
	if len(entries) == 0 {
		return nil
	}

	targetSize := targetOutputFileSize(level)

	var chunks [][]*entry.Entry
	var chunk []*entry.Entry
	chunkSize := 0

	for _, e := range entries {
		entrySize := e.Size()

		if len(chunk) > 0 && chunkSize+entrySize > targetSize {
			chunks = append(chunks, chunk)
			chunk = nil
			chunkSize = 0
		}

		chunk = append(chunk, e)
		chunkSize += entrySize
	}

	if len(chunk) > 0 {
		chunks = append(chunks, chunk)
	}

	return chunks
}

func targetOutputFileSize(level int) int {

	size := baseSSTableSizeBytes << level

	if size > maxSSTableSizeBytes {
		return maxSSTableSizeBytes
	}

	return size
}
