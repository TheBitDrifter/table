
# Table Package Overview

This package defines a core `Table` data structure and its associated interfaces, designed for [ECS](https://github.com/SanderMertens/ecs-faq?tab=readme-ov-file#what-is-ecs) (Entity-Component-System) architecture. A `Table` stores a fixed set of typed rows, where each column represents a
identity, referred to as `Entry` or `EntryID`.

## Key Concepts and API

- **Rows and Entries**: Each row in the `Table` corresponds to a specific type(`ElementType`), and each column represents an
`Entry`.
- **Entry-Based Operations**: The API supports operations such as `New`, `Delete`, and `Transfer` for efficient `Entry` management.
- **Set Operations**: Methods like `All`, `Any`, and `None` allow checks across `ElementTypes` to confirm which are present or
absent from a `Table`.

## Optional and Flexible Schema-Based Access

Unlike most other Go `ECS` implementations, this package introduces a `Schema`, which maps global `ElementTypes` to
their local positions within a given table. This mapping allows the number of global `ElementTypes` to exceed the maximum
that each individual table (or parent storage) can store. `Tables` with mismatched `Schemas` are permitted to transfer
`Entries`, allowing data exchange between tables, even when they store different subsets of `ElementTypes`.

## Build Tags

- `unsafe` This build tag sets the package to "unsafe mode". In this mode, the package will leverage unsafe pointers and pointer arithmetic for maximum performance, rather than using interface-based access. The package factory will satisfy the `Table` interface using an `UnsafeRowCache` (an unsafe pointer array) and expose a concrete `Accessor` that is aware of this implementation.

- `schema_enabled` This build tag sets the package to "quick schema mode". In this mode, the package introduces a `Schema` that maps global `ElementTypes` to their local positions within a given `Table`. This mapping allows the number of global `ElementTypes` to exceed the maximum that each individual Table (or parent storage) can store. The package factory will expose a concrete `Accessor` that is aware of the schema implementation.

## ECS Terminology

In traditional `ECS` terms, the concepts in this package align as follows:

- `Table` ↔ `Archetype`
- `Entry`/`EntryID` ↔ `Entity`
- `ElementType` ↔ `Component`

## Extensibility

This package aims to enable extensibility through polymorphism/flexible interfaces:

Factories accept interfaces, making it possible to extend the table's functionality without
altering core implementations.

## Performance vs Dependency/Structure

While it’s possible to use this library without concrete objects, peak performance is achieved by avoiding interface overhead, allowing the compiler to optimize further.
To reduce the impact of this dependency, the package encapsulates these details within the access behavior using `Accessor` objects that understand and assert underlying types based on build tags.
Since these `Accessors` are closely tied to the internal `Table` structure, they may not work correctly with `Tables` that have been replaced or extended by custom implementations. However, they can serve as a template for creating custom accessors if needed.

## In Depth Documentation/Usage

For in depth details please see the [docs]("placeholder.net").
