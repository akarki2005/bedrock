package compaction

import (
	"github.com/akarki2005/lsm-engine/internal/entry"
	"github.com/akarki2005/lsm-engine/internal/sstable"
)

type Plan struct {
	level    int
	inputs   []*sstable.SSTable
	overlaps []*sstable.SSTable
}

func New(level int, inputs, overlaps []*sstable.SSTable) *Plan {
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
	return nil, nil
}

func overlaps(a, b *sstable.SSTable) bool {
	return false
}

func mergeEntries(inputs []*sstable.SSTable, overlaps []*sstable.SSTable) ([]*entry.Entry, error) {
	return nil, nil
}

func writeOutputs(dir string, entries []*entry.Entry) ([]*sstable.SSTable, error) {
	return nil, nil
}
