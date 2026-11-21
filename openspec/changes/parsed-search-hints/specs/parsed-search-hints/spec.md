## ADDED Requirements

### Requirement: Parse results may carry named search hints

`ParsedMedia.Hints` SHALL accept up to 16 plugin-chosen entries. Keys SHALL match
`[a-z0-9_]{1,32}` and values SHALL be at most 256 bytes. The SDK SHALL validate these limits in
`ValidateParseHints` and the parser conformance harness.

#### Scenario: Parse result with an out-of-range hint

- **WHEN** a result has an invalid key, an oversized value, or more than 16 hints
- **THEN** parse hint validation returns an error

### Requirement: Bundles may source hints from embedded tags

`Bundle.EmbeddedTagHints` SHALL map exact, non-blank `ParseProbe.Tags` names to distinct valid hint
keys, with at most 16 mappings. The host SHALL fill absent or empty hints from non-empty mapped
tags and validate the resulting parse hints before using them.

#### Scenario: Parser and embedded tag supply the same hint

- **WHEN** a parsed hint and its mapped tag both have a non-empty value
- **THEN** the parser's value takes precedence

### Requirement: Scraper search templates may interpolate named hints

`SearchTemplate` SHALL accept `{hint.<key>}` for valid hint keys unless the placeholder is immediately
preceded by `{` or followed by `}`. After removing valid placeholders, the bundle validator SHALL
reject any remaining `hint.` as malformed. The host SHALL URL-query-escape each substituted value.
A missing or empty value SHALL render as an empty string. Each `RequiredSearchHints` key SHALL be
referenced by the search template, appear once, and count toward the 16-hint limit.

#### Scenario: A required hint is unavailable

- **WHEN** a required search hint is missing or empty
- **THEN** identification skips this scraper for that search

#### Scenario: An optional hint is unavailable

- **WHEN** an optional search hint is missing
- **THEN** its placeholder renders as an empty string and the search may proceed

### Requirement: Hint placeholders are limited to search URL queries

Hint placeholders SHALL appear only in a search template's URL query. The bundle validator
SHALL reject them in lookup templates and GraphQL request bodies. URL request templates SHALL
allow literal braces outside hint placeholders.

#### Scenario: A scraper declares a hint placeholder in a GraphQL request

- **WHEN** a GraphQL search body contains a hint placeholder
- **THEN** bundle validation returns an error
