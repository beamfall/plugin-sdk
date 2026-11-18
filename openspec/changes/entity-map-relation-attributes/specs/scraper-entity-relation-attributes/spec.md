## ADDED Requirements

### Requirement: Entity maps write declared relation attributes

`EntityMap` SHALL accept `RelationAttributeMap`, mapping attribute names declared on the map's
`Relation` to paths resolved against the `Path` element itself, never a `LookupTemplate` document. The
host SHALL write each resolved, non-blank value as an attribute of the relation row it writes for that
element, and SHALL omit unresolved or blank values.

#### Scenario: Cast character on the credit row

- **WHEN** a map over `credits.cast` declares `RelationAttributeMap` `{"character": "character"}` and
  a secondary `LookupTemplate`
- **THEN** each credit row carries the cast element's `character`, not a value from the looked-up
  person document

#### Scenario: Undeclared attribute

- **WHEN** a `RelationAttributeMap` key is not an attribute its relation declares
- **THEN** `ValidateBundle` returns an error naming the attribute

### Requirement: Repeated links merge into one row

When one identification links the same Entity to the same Media under one relation more than once,
the host SHALL write one relation row whose attributes merge: distinct values of one attribute SHALL
join in map declaration order, then element order, separated by ", ". The row's sort position SHALL be
the smallest position among the links.

#### Scenario: Writer credited twice

- **WHEN** a crew list names one person with jobs `Screenplay` and `Writer`
- **THEN** the person's credit row carries `job` `Screenplay, Writer`

### Requirement: Entity maps may filter their elements

`EntityMap` SHALL accept `ElementFilter`, mapping element-relative paths to lists of accepted values.
An element SHALL hydrate only when every filter path resolves to one of that path's values; a
filtered-out element SHALL hydrate nothing and consume no secondary lookup. Validation SHALL reject a
blank filter path or an empty value list.

#### Scenario: Key crew jobs only

- **WHEN** a map over `credits.crew` declares `ElementFilter` `{"job": ["Director", "Writer"]}`
- **THEN** only the director and writer elements hydrate

### Requirement: Maps may offset their sort positions

`EntityMap` SHALL accept a non-negative `OrdinalOffset` that the host adds to every sort position the
map writes. `OrdinalField` SHALL name a key of either `FieldMap` or `RelationAttributeMap`. Validation
SHALL reject a negative offset.

#### Scenario: Crew after cast

- **WHEN** a cast map and a crew map write the same relation and the crew map declares
  `OrdinalOffset` 1000
- **THEN** every crew row sorts after every cast row with a position below 1000

### Requirement: Elements past the lookup cap still hydrate

When a map's secondary lookups reach `MaxLookups`, the host SHALL hydrate the remaining elements from
the element itself, as if the map declared no `LookupTemplate`. An Entity hydrated with no resolved
`FieldMap` value SHALL keep the typed fields an earlier identification wrote.

#### Scenario: Long cast list

- **WHEN** a cast map with `MaxLookups` 10 resolves 40 elements
- **THEN** all 40 people are credited, the first 10 with their looked-up fields

### Requirement: Container maps reject the new fields

Validation SHALL reject a container map that declares `RelationAttributeMap`, `ElementFilter` or a
non-zero `OrdinalOffset`.

#### Scenario: Container map with relation attributes

- **WHEN** a container map declares `RelationAttributeMap`
- **THEN** `ValidateBundle` returns a container map error
