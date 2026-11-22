## ADDED Requirements

### Requirement: Explicit scraper list root

The SDK SHALL accept `$` as the entire response path in `CandidatesPath` and
child listing `ItemsPath`. Surrounding whitespace is ignored for the root
marker. An empty path retains its existing meaning. The marker SHALL NOT be
combined with a dotted segment or a first-match predicate.

#### Scenario: The response is a bare array

- **WHEN** a search response or child listing is an array of objects and its list path is `$`
- **THEN** the list resolves to those rows in response order
- **AND** scalar row mappings resolve relative to each row

### Requirement: Scraper list filtering

A terminal `[?key=value,...]` predicate SHALL return every object row matching
all conditions, in response order. The predicate can follow a named array or
`$`. Both predicate forms SHALL try the literal condition name on the row
before a dotted walk and compare values using the existing scalar string
conversion. The existing predicate without `?` SHALL still select the first match.

#### Scenario: An array mixes rows from different orderings

- **WHEN** a list path ends in a filter-all predicate over a nested ordering field
- **THEN** every matching row is retained and other rows are discarded
- **AND** zero matches produces an empty array

#### Scenario: A list-only form is misplaced

- **WHEN** a path uses the root marker or filter-all predicate outside the two list positions
- **THEN** bundle validation rejects the declaration
- **AND** validation also rejects any path segment after a filter-all predicate
