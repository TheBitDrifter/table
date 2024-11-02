package table

import (
	"fmt"
	"iter"
	"math"
	"reflect"
	"unsafe"

	"github.com/TheBitDrifter/mask"
	iter_util "github.com/TheBitDrifter/util/iter"
	numbers_util "github.com/TheBitDrifter/util/numbers"
)

var _ Table = &quickTable{}

type quickTable struct {
	schema       Schema
	entryIndex   EntryIndex
	rowCache     rowCache
	safeCache    []any
	unsafeCache  []unsafe.Pointer
	elementTypes []ElementType
	mask         mask.Mask
	entryIDs     []EntryID
	rows         []Row
	len          int
	cap          int
}

func newTable(
	schema Schema, safeTable bool, entryIndex EntryIndex, elementTypes ...ElementType,
) (*quickTable, error) {
	if schema == nil {
		return nil, TableInstantiationNilSchemaError{}
	}

	if Config.AutoComponentRegistrationTableCreation && !Config.SchemaLess() {
		schema.Register(elementTypes...)
	}
	tbl := &quickTable{
		schema:     schema,
		entryIndex: entryIndex,
	}
	tbl.elementTypes = elementTypes
	rowCount := schema.Registered()

	tbl.rows = make([]Row, rowCount)
	if safeTable {
		tbl.rowCache = safeCache(make([]any, rowCount))
		tbl.safeCache = tbl.rowCache.(safeCache)
	} else {
		tbl.rowCache = unsafeCache(make([]unsafe.Pointer, rowCount))
		tbl.unsafeCache = tbl.rowCache.(unsafeCache)
	}
	for _, elementType := range tbl.elementTypes {
		rowIndex := tbl.schema.RowIndexFor(elementType)
		tbl.rows[rowIndex] = newRow(elementType)

		bit := rowIndex
		tbl.mask.Mark(bit)
	}
	return tbl, nil
}

func (tbl *quickTable) Entry(n int) (Entry, error) {
	if n < 0 || n >= len(tbl.entryIDs) {
		return nil, AccessError{Index: n, UpperBound: len(tbl.entryIDs)}
	}
	entries := tbl.entryIndex.Entries()
	index := int(tbl.entryIDs[n]) - 1

	if index < 0 || index >= len(entries) {
		return nil, AccessError{Index: index, UpperBound: len(entries)}
	}
	entry := entries[index]
	if entry.ID() == 0 {
		return nil, InvalidEntryAccessError{}
	}
	return entries[index], nil
}

func (tbl *quickTable) NewEntries(n int) ([]Entry, error) {
	if n <= 0 {
		return nil, BatchOperationError{Count: n}
	}
	defer tbl.rowCache.cacheRows(tbl)

	err := tbl.ensureCapacity(n)
	if err != nil {
		return nil, err
	}
	err = tbl.setLen(n)
	if err != nil {
		return nil, err
	}
	entries, entryIndexError := tbl.entryIndex.NewEntries(n, tbl)
	if entryIndexError != nil {
		return nil, entryIndexError
	}
	for _, entry := range entries {
		tbl.entryIDs = append(tbl.entryIDs, entry.ID())
	}
	return entries, nil
}

func (tbl *quickTable) DeleteEntries(indices ...int) ([]EntryID, error) {
	n, err := tbl.prepForPopDeletion(indices...)
	if err != nil {
		return nil, err
	}
	return tbl.popEntries(n, true), nil
}

func (tbl *quickTable) TransferEntries(
	other Table,
	indexes ...int,
) error {
	indexes = numbers_util.UniqueInts(indexes)
	n := len(indexes)

	if n <= 0 {
		return BatchOperationError{Count: n}
	}
	if n > tbl.len {
		return BatchDeletionError{Capacity: tbl.len, BatchOperationError: BatchOperationError{Count: n}}
	}
	if entryIndexTracker[tbl] != entryIndexTracker[other] {
		return TransferEntryIndexMismatchError{}
	}
	defer tbl.rowCache.cacheRows(tbl)

	sharedElementTypes := tbl.sharedElementTypesWith(other)
	if len(sharedElementTypes) == 0 {
		_, err := tbl.DeleteEntries(indexes...)
		if err != nil {
			return err
		}
		_, err = other.NewEntries(len(indexes))
		if err != nil {
			return err
		}
		return nil
	}
	otherTableLength := other.Length()
	entryIDs := make([]EntryID, len(indexes))
	for i, idx := range indexes {
		if idx < 0 || idx >= len(tbl.entryIDs) {
			return AccessError{Index: idx, UpperBound: len(tbl.entryIDs)}
		}
		entryIDs[i] = tbl.entryIDs[idx]
	}
	err := tbl.entryIndex.RecycleEntries(entryIDs...)
	_, err = other.NewEntries(n)
	if err != nil {
		return err
	}
	for i, idx := range indexes {
		for _, sharedElementType := range sharedElementTypes {
			row, err := tbl.Row(sharedElementType)
			if err != nil {
				return err
			}
			otherRow, err := other.Row(sharedElementType)
			if err != nil {
				return err
			}
			otherRow.set(i+otherTableLength, row.get(idx))
		}
	}
	n, err = tbl.prepForPopDeletion(indexes...)
	if err != nil {
		return err
	}
	tbl.popEntries(n, false)
	return nil
}

func (tbl *quickTable) Clear() error {
	defer tbl.rowCache.cacheRows(tbl)

	tbl.len = 0
	tbl.cap = 0

	for elementType := range tbl.ElementTypes() {
		row, err := tbl.Row(elementType)
		if err != nil {
			return err
		}
		row.setLen(tbl.len)
		row.setCap(tbl.cap)
	}
	return nil
}

func (tbl *quickTable) Length() int {
	return tbl.len
}

func (tbl *quickTable) ElementTypes() iter.Seq[ElementType] {
	return func(yield func(ElementType) bool) {
		for _, elementType := range tbl.elementTypes {
			if !yield(elementType) {
				return
			}
		}
	}
}

func (tbl *quickTable) Rows() iter.Seq2[int, Row] {
	return func(yield func(int, Row) bool) {
		for i, row := range tbl.rows {
			if row.CanAddr() && !yield(i, row) {
				return
			}
		}
	}
}

func (tbl *quickTable) RowCount() int {
	return len(tbl.elementTypes)
}

func (tbl *quickTable) Row(elementType ElementType) (Row, error) {
	if !tbl.Contains(elementType) {
		return Row{}, InvalidElementAccessError{elementType, iter_util.Collect(tbl.ElementTypes())}
	}
	rowIndex := tbl.schema.RowIndexFor(elementType)
	return tbl.rows[rowIndex], nil
}

func (tbl *quickTable) Contains(elementType ElementType) bool {
	if !tbl.schema.Contains(elementType) {
		return false
	}
	bit := tbl.schema.RowIndexFor(elementType)
	return tbl.mask.Contains(bit)
}

func (tbl *quickTable) ContainsAll(elementTypes ...ElementType) bool {
	if !tbl.schema.ContainsAll(elementTypes...) {
		return false
	}
	mask := mask.Mask{}
	for _, elementType := range elementTypes {
		bit := tbl.schema.RowIndexFor(elementType)
		mask.Mark(bit)
	}
	return tbl.mask.ContainsAll(mask)
}

func (tbl *quickTable) ContainsAny(elementTypes ...ElementType) bool {
	mask := mask.Mask{}
	for _, elementType := range elementTypes {
		if !tbl.schema.Contains(elementType) {
			continue
		}
		bit := tbl.schema.RowIndexFor(elementType)
		mask.Mark(bit)
	}
	return tbl.mask.ContainsAny(mask)
}

func (tbl *quickTable) ContainsNone(elementTypes ...ElementType) bool {
	msk := mask.Mask{}
	for _, elementType := range elementTypes {
		if !tbl.schema.Contains(elementType) {
			continue
		}
		bit := tbl.schema.RowIndexFor(elementType)
		msk.Mark(bit)
	}
	if msk.IsEmpty() {
		return true
	}
	return tbl.mask.ContainsNone(msk)
}

func (tbl *quickTable) Get(elementType ElementType, idx int) (reflect.Value, error) {
	row, err := tbl.Row(elementType)
	if err != nil {
		return reflect.Value{}, err
	}
	return row.get(idx), nil
}

func (tbl *quickTable) Set(elementType ElementType, re reflect.Value, idx int) error {
	row, err := tbl.Row(elementType)
	if err != nil {
		return err
	}
	row.set(idx, re)
	rowIdx := tbl.schema.RowIndexFor(elementType)
	tbl.rowCache.cacheRow(int(rowIdx), row)
	return nil
}

// --------- helpers --------------
func (tbl *quickTable) setLen(n int) error {
	tbl.len += n
	for elementType := range tbl.ElementTypes() {
		row, err := tbl.Row(elementType)
		if err != nil {
			return InvalidElementAccessError{elementType, nil}
		}
		row.setLen(tbl.len)
	}
	return nil
}

func (tbl *quickTable) shrink() {
	if float64(tbl.cap) > float64(tbl.len)*1.2 {
		tbl.cap = tbl.len
	}
}

func (tbl *quickTable) ensureCapacity(n int) error {
	minCapacityRequired := tbl.len + n

	if tbl.cap >= minCapacityRequired {
		return nil
	}
	twentyPercentIncrease := float64(tbl.cap) * 1.2
	newCap := math.Max(float64(minCapacityRequired), twentyPercentIncrease)
	tbl.cap = int(newCap)

	for elementType := range tbl.ElementTypes() {
		tableRow, err := tbl.Row(elementType)
		if err != nil {
			return err
		}
		rowIndex := tbl.schema.RowIndexFor(elementType)
		rowType := tableRow.Type()

		newRowVal := reflect.New(rowType).Elem()
		newRowVal.Set(
			reflect.MakeSlice(rowType, tbl.len, tbl.cap),
		)

		reflect.Copy(newRowVal, reflect.Value(tableRow))
		tbl.rows[rowIndex] = Row(newRowVal)
	}
	return nil
}

func (tbl *quickTable) sharedElementTypesWith(other Table) []ElementType {
	sharedElements := make([]ElementType, 0)
	for otherElementType := range other.ElementTypes() {
		if tbl.Contains(otherElementType) {
			sharedElements = append(sharedElements, otherElementType)
		}
	}
	return sharedElements
}

func (tbl *quickTable) swapEntries(i, j int) {
	if i < 0 || i >= tbl.len || j < 0 || j >= tbl.len {
		panic(fmt.Sprintf("swap columns bound error i: %d, j: %d, len: %d", i, j, tbl.len))
	}
	for _, row := range tbl.rows {
		if !row.CanAddr() {
			continue
		}
		copyI := row.get(i)
		row.set(i, row.get(j))

		copyAsElement := copyI
		row.set(j, copyAsElement)
	}
	tbl.entryIDs[i], tbl.entryIDs[j] = tbl.entryIDs[j], tbl.entryIDs[i]
	tbl.entryIndex.UpdateIndex(tbl.entryIDs[i], i)
	tbl.entryIndex.UpdateIndex(tbl.entryIDs[j], j)
}

func (tbl *quickTable) popEntries(n int, recycle bool) []EntryID {
	if n > tbl.len || n <= 0 {
		panic(fmt.Sprintf("cannot pop %d, table len: %d", n, tbl.len))
	}
	defer tbl.rowCache.cacheRows(tbl)
	entryIDsToDelete := tbl.entryIDs[tbl.len-n : tbl.len]
	tbl.setLen(-n)
	tbl.shrink()
	tbl.entryIDs = tbl.entryIDs[:tbl.len]
	if recycle {
		tbl.entryIndex.RecycleEntries(entryIDsToDelete...)
	}
	return entryIDsToDelete
}

func (tbl *quickTable) prepForPopDeletion(indices ...int) (int, error) {
	sortedUnique := numbers_util.UniqueInts(indices)
	n := len(sortedUnique)

	if n <= 0 {
		return 0, BatchOperationError{Count: n}
	}
	if n > tbl.len {
		return 0, BatchDeletionError{Capacity: tbl.len, BatchOperationError: BatchOperationError{Count: n}}
	}
	defer tbl.rowCache.cacheRows(tbl)

	sortedDescending := numbers_util.DescendingInts(sortedUnique)
	for _, idx := range sortedDescending {
		if idx < 0 || idx >= tbl.len {
			return 0, AccessError{Index: idx, UpperBound: tbl.len}
		}
	}
	for i, idx := range sortedDescending {
		tbl.swapEntries(idx, tbl.len-1-i)
	}
	return n, nil
}
