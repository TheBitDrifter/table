package table

import (
	"iter"
	"reflect"
)

type (
	EntryID       uint32
	ElementTypeID uint32
	RowIndex      uint32
)

type Entry interface {
	ID() EntryID
	Recycled() int
	Index() int
}

type EntryIndex interface {
	Entries() []Entry
	NewEntries(int, Table) ([]Entry, error)
	UpdateIndex(EntryID, int) error
	RecycleEntries(...EntryID) error
	Reset() error
	Recyclable() []Entry
}

type Schema interface {
	Register(...ElementType)
	Registered() int
	Contains(ElementType) bool
	ContainsAll(...ElementType) bool
	RowIndexFor(ElementType) uint32
	RowIndexForID(ElementTypeID) uint32
}

type ElementType interface {
	ID() ElementTypeID
	Type() reflect.Type
	Size() uint32
}

type (
	Row reflect.Value
)

type Table interface {
	Entry(int) (Entry, error)
	NewEntries(int) ([]Entry, error)
	DeleteEntries(...int) ([]EntryID, error)
	TransferEntries(Table, ...int) error
	Contains(ElementType) bool
	ContainsAll(...ElementType) bool
	ContainsNone(...ElementType) bool
	ContainsAny(...ElementType) bool
	Clear() error
	Length() int
	ElementTypes() iter.Seq[ElementType]
	Rows() iter.Seq2[int, Row]
	RowCount() int
	Row(ElementType) (Row, error)
	Get(ElementType, int) (reflect.Value, error)
	Set(ElementType, reflect.Value, int) error
}

// Warning: internal Dependencies abound!
type Accessor[T any] struct {
	elementTypeID ElementTypeID
}

type LockedAccessor[T any] struct {
	schema   *quickSchema
	rowIndex uint32
}
