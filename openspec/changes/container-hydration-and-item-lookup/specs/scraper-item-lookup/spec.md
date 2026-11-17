## ADDED Requirements

### Requirement: Templates may name the item's sequence and index

Scraper templates SHALL accept `{sequence}` and `{index}` placeholders carrying the looked-up Media's
parsed sequence and index. They SHALL be accepted in `LookupTemplate`, `ItemLookupTemplate`,
`EntityMap.LookupTemplate` and `EntityMap.Path`. A value the host does not know SHALL substitute as the
empty string in a request template, and an entity map whose path names an unknown value SHALL be
skipped.

#### Scenario: Sub-container selected by sequence

- **WHEN** an entity map path is `seasons[season_number={sequence}]` and the Media's parsed sequence
  is `1`
- **THEN** the path selects the element whose `season_number` is 1

#### Scenario: Unknown sequence

- **WHEN** the Media has no parsed sequence
- **THEN** an entity map whose path names `{sequence}` is skipped, never resolved against a blank value

### Requirement: Scrapers may declare a per-item lookup

`ScraperSpec` SHALL accept `ItemLookupTemplate` with `ItemFieldMap` and `ItemArtworkMap`. The host SHALL
fetch the item document after the main lookup, with the same `{id}` plus `{sequence}` and `{index}`, map
it with the item maps and let item values override the main document's for the same field or slot. A
template naming a value the host does not know SHALL skip the item fetch, and a failed item fetch SHALL
leave the main mapping unchanged.

#### Scenario: Item document refines the container document

- **WHEN** a scraper maps `summary` from the container document and `ItemFieldMap` maps `summary` from
  `/tv/{id}/season/{sequence}/episode/{index}`
- **THEN** the identified Media's summary is the item document's value

### Requirement: Item lookups are validated

Validation SHALL require an http-json scraper with a `LookupTemplate`, an item template containing
`{id}`, item maps only with an item template, and known fields and artwork slots in the item maps.

#### Scenario: Item map without a template

- **WHEN** a scraper declares `ItemFieldMap` without `ItemLookupTemplate`
- **THEN** `ValidateBundle` returns an item lookup error
