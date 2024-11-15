package table

import (
	numbers_util "github.com/TheBitDrifter/util/numbers"
)

var _ EntryIndex = &entryIndex{}

type entryIndex struct {
	currEntryID EntryID
	entries     []Entry
	recyclable  []Entry
}

func (ei *entryIndex) NewEntries(n int, tbl Table) ([]Entry, error) {
	if n <= 0 {
		return nil, BatchOperationError{Count: n}
	}
	amountRecyclable := min(len(ei.recyclable), n)
	newEntries := []Entry{}

	for i := 0; i < amountRecyclable; i++ {
		entry := entry{
			id:       ei.recyclable[i].ID(),
			recycled: ei.recyclable[i].Recycled() + 1,
			table:    tbl,
		}
		index := entry.ID() - 1
		ei.entries[index] = entry
		newEntries = append(newEntries, entry)
	}
	ei.recyclable = ei.recyclable[amountRecyclable:]
	leftover := n - amountRecyclable

	for i := 0; i < leftover; i++ {
		ei.currEntryID++
		entry := entry{
			id:       ei.currEntryID,
			recycled: 0,
			table:    tbl,
		}
		ei.entries = append(ei.entries, entry)
		newEntries = append(newEntries, entry)
	}
	return newEntries, nil
}

func (ei *entryIndex) Entries() []Entry {
	return ei.entries
}

func (ei *entryIndex) UpdateIndex(id EntryID, rowIndex int) error {
	entryIndex := int(id - 1)
	if entryIndex < 0 || entryIndex >= len(ei.entries) {
		return AccessError{Index: entryIndex, UpperBound: len(ei.entries) - 1}
	}
	e := ei.entries[entryIndex]
	newEntry := entry{
		id:       e.ID(),
		recycled: e.Recycled(),
		rowIndex: rowIndex,
	}
	ei.entries[entryIndex] = newEntry
	return nil
}

func (ei *entryIndex) RecycleEntries(ids ...EntryID) error {
	uniqueIDs := numbers_util.UniqueInts(entryIDs(ids).toInts())

	uniqCount := len(uniqueIDs)
	entriesCount := len(ei.entries)
	if uniqCount <= 0 || uniqCount >= entriesCount {
		return BatchDeletionError{Capacity: uniqCount, BatchOperationError: BatchOperationError{Count: uniqCount}}
	}

	for _, id := range ids {
		index := id - 1
		if ei.entries[index].ID() == 0 {
			continue
		}
		zeroEntry := entry{
			id:       0,
			recycled: ei.entries[index].Recycled(),
			rowIndex: 0,
		}
		recycledEntry := entry{
			id:       id,
			recycled: ei.entries[index].Recycled(),
			rowIndex: 0,
		}
		ei.recyclable = append(ei.recyclable, recycledEntry)
		ei.entries[index] = zeroEntry
	}
	return nil
}

func (ei *entryIndex) Reset() error {
	ei.entries = ei.entries[:0]
	ei.recyclable = ei.recyclable[:0]
	ei.currEntryID = 0
	return nil
}

func (ei *entryIndex) Recyclable() []Entry {
	return ei.recyclable
}
