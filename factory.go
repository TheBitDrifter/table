package table

import (
	"reflect"
)

var (
	buildTags                      = []string{}
	Factory           TableFactory = initTableFactory()
	entryIndexTracker              = map[Table]EntryIndex{}
)

func initTableFactory() TableFactory {
	var tableB tableBuilder = safeTableBuilder{}
	if Config.Unsafe() {
		tableB = unsafeTableBuilder{}
	}
	var schemaB schemaBuilder = nilSchemaBuilder{}
	if !Config.SchemaLess() {
		schemaB = quickSchemaBuilder{}
	}
	return tableFactory{
		tableBuilder:  tableB,
		schemaBuilder: schemaB,
	}
}

type TableFactory interface {
	tableBuilder
	schemaBuilder
	NewEntryIndex() EntryIndex
}

type tableBuilder interface {
	NewTable(Schema, EntryIndex, ...ElementType) (Table, error)
}

type schemaBuilder interface {
	NewSchema() Schema
}

type (
	safeTableBuilder   struct{}
	unsafeTableBuilder struct{}
	nilSchemaBuilder   struct{}
	quickSchemaBuilder struct{}

	tableFactory struct {
		tableBuilder
		schemaBuilder
	}
)

func (tableFactory) NewEntryIndex() EntryIndex {
	return &entryIndex{}
}

func (factory safeTableBuilder) NewTable(schema Schema, entryIndex EntryIndex, elementTypes ...ElementType) (Table, error) {
	tbl, err := newTable(schema, true, entryIndex, elementTypes...)
	if err != nil {
		return nil, err
	}
	entryIndexTracker[tbl] = entryIndex
	return tbl, nil
}

func (factory unsafeTableBuilder) NewTable(schema Schema, entryIndex EntryIndex, elementTypes ...ElementType) (Table, error) {
	tbl, err := newTable(schema, false, entryIndex, elementTypes...)
	if err != nil {
		return nil, err
	}
	entryIndexTracker[tbl] = entryIndex
	return tbl, err
}

func (factory quickSchemaBuilder) NewSchema() Schema {
	return newQuickSchema()
}

func (factory nilSchemaBuilder) NewSchema() Schema {
	return newNilSchema()
}

func FactoryNewElementType[T any]() ElementType {
	return newElementType[T]()
}

// Warning: Internal dependency abound!
func FactoryNewAccessor[T any](elementType ElementType) Accessor[T] {
	var zero T
	tType := reflect.TypeOf(zero)

	if tType.Kind() == reflect.Ptr {
		tType = tType.Elem()
	}
	if elementType.Type() != tType {
		panic("mismatch")
	}
	return Accessor[T]{elementType.ID()}
}
