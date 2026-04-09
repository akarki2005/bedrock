package compaction

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/sstable"
)

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

func Run(plan *Plan, dir string) ([]*sstable.SSTable, error) {
	if plan == nil {
		return nil, errors.New("nil compaction plan")
	}

	entries, err := mergeEntries(plan.inputs, plan.overlaps)
	if err != nil {
		return nil, fmt.Errorf("merge entries: %w", err)
	}

	outputs, err := writeOutputs(dir, entries)
	if err != nil {
		return nil, fmt.Errorf("write outputs: %w", err)
	}

	return outputs, nil
}

func overlaps(a, b *sstable.SSTable) bool {
	return !(bytes.Compare(a.MaxKey(), b.MinKey()) < 0 || bytes.Compare(b.MaxKey(), a.MinKey()) < 0)
}

func mergeEntries(inputs []*sstable.SSTable, overlaps []*sstable.SSTable) ([]*entry.Entry, error) {
	allTables := make([]*sstable.SSTable, 0, len(inputs)+len(overlaps))
	allTables = append(allTables, inputs...)
	allTables = append(allTables, overlaps...)

	if len(allTables) == 0 {
		return nil, nil
	}

	tableEntries := make([][]*entry.Entry, len(allTables))
	for i, table := range allTables {
		entries, err := readAllEntries(table)
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

func writeOutputs(dir string, entries []*entry.Entry) ([]*sstable.SSTable, error) {
	return nil, nil
}
