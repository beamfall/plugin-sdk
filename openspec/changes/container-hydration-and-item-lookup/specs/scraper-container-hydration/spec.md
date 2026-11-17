## ADDED Requirements

### Requirement: Entity maps may hydrate an existing container

`EntityMap` SHALL accept `Container`. A container map SHALL hydrate the existing container Entity of its
`Kind` that holds the looked-up Media, directly or up the container chain, with the artwork and typed
fields it resolves, and SHALL NOT create an Entity or a relation. Its `Path` MAY be empty, meaning the
lookup document root.

#### Scenario: Series poster from the document root

- **WHEN** a container map for a container kind has an empty path and `ArtworkPath` `poster_path`
- **THEN** the container Entity of that kind holding the Media receives the resolved poster

#### Scenario: No container holds the Media

- **WHEN** no Entity of the map's kind holds the Media
- **THEN** the map hydrates nothing and the identification still applies

### Requirement: Container maps are validated

Validation SHALL reject a container map whose kind is not a `container` archetype, that names a
`Relation`, that declares a secondary lookup, a parent name path, a parent relation or an ordinal
field, or that declares neither `ArtworkPath` nor `FieldMap`.

#### Scenario: Container map naming a relation

- **WHEN** a container map declares a `Relation`
- **THEN** `ValidateBundle` returns a container map error
