## ADDED Requirements

### Requirement: Container maps may hydrate several artwork slots

`EntityMap.ArtworkMap` SHALL map core artwork slots to container-node-relative paths on a Container map.
The host SHALL compose each resolved value like the scraper's Media artwork and attach it to the
container Entity under that slot.

#### Scenario: Series backdrop

- **WHEN** a container map declares `ArtworkMap` `{"backdrop": "backdrop_path"}` and the document has a backdrop path
- **THEN** the container Entity receives a backdrop image

#### Scenario: Unknown slot

- **WHEN** a container map's `ArtworkMap` names a slot that is not a core artwork slot
- **THEN** `ValidateBundle` returns an error

### Requirement: Container maps may set the display title

`EntityMap.TitlePath` SHALL resolve, against the container node, a display title for the container
Entity. A non-blank value SHALL replace the title without changing the Entity's identity; an unresolved
or blank value SHALL keep the existing title.

#### Scenario: Matched title replaces the folder spelling

- **WHEN** the container Entity is named "A Mothers Son" and `TitlePath` resolves "A Mother's Son"
- **THEN** the Entity's title is "A Mother's Son" and its ID is unchanged

#### Scenario: Unresolved title keeps the folder spelling

- **WHEN** `TitlePath` does not resolve
- **THEN** the container Entity keeps its existing title

### Requirement: Container maps may link sibling-hydrated Entities

`EntityMap.ContainerRelations` SHALL map a relation declared on the container Kind, whose `ToKind` is an
Entity kind, to the `Relation` of a non-container entity map of that kind in the same scraper, and no
non-container map of another kind SHALL declare that `Relation`. The host
SHALL link the container Entity to each Entity that map hydrated in the same identification, keeping its
sort order.

#### Scenario: Series genres

- **WHEN** a series container map declares `ContainerRelations` `{"in-genre": "genre-of"}` and a genre map with relation `genre-of` hydrates Drama
- **THEN** the series Entity is linked to the Drama Entity under `in-genre`

#### Scenario: No sibling map

- **WHEN** `ContainerRelations` names a relation that no non-container map of the target kind declares
- **THEN** `ValidateBundle` returns an error

#### Scenario: Sibling relation shared by another kind

- **WHEN** `ContainerRelations` names a relation that a non-container map of another kind also declares
- **THEN** `ValidateBundle` returns an error

### Requirement: Container-only map fields are validated

A Container map SHALL be valid when it declares at least one of `ArtworkPath`, `ArtworkMap`, `FieldMap`
or `TitlePath`. Validation SHALL reject `ArtworkMap`, `TitlePath` and `ContainerRelations` on a map that
is not a Container map, blank paths, a `poster` slot declared by both `ArtworkPath` and `ArtworkMap`, and
a container relation that the Kind does not declare toward an Entity kind.

#### Scenario: Title-only container map

- **WHEN** a container map declares only `TitlePath`
- **THEN** `ValidateBundle` accepts it

#### Scenario: Title path on a non-container map

- **WHEN** a non-container map declares `TitlePath`
- **THEN** `ValidateBundle` returns an error

### Requirement: Scraper reference traces cover Container maps

`sdktest.CompileScraperTrace` SHALL emit each Container map's fields, artwork slots, display title and
container relation links against the lookup document, resolving the documented path grammar (zero-based
array indices and `key[k=v,…]` predicates). `RunScraperTraceConformance` SHALL grade containers only
when a vector's `Want.Containers` is set, so vectors frozen before this change keep passing.

#### Scenario: Show container vector

- **WHEN** a vector's lookup document carries a series name with an accent, a backdrop, `networks[]`,
  US content ratings, genres and cast, and `Want.Containers` lists the expected show container
- **THEN** the trace grades the container's fields, artwork, title and links, in sorted relation order

#### Scenario: Vector without containers

- **WHEN** a vector sets no `Want.Containers`
- **THEN** container output is not graded

### Requirement: Scraper reference traces read referenced documents

For a sibling map with a `LookupTemplate`, `sdktest.CompileScraperTrace` SHALL resolve each element's
name against the referenced document the vector supplies in `TraceInput.EntityDocs`, keyed by the template
with `{id}` replaced, honouring `MaxLookups`. An element without a supplied document SHALL hydrate nothing.

#### Scenario: Cast names from person documents

- **WHEN** a cast element `{"id":1,"name":"Lead"}` has the referenced document `/person/1` named "Lead Actor"
- **THEN** the container's credit link names "Lead Actor"
