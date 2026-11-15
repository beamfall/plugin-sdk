## ADDED Requirements

### Requirement: Scrapers may map a Media sort title as a built-in key

`BuiltinMediaFields` SHALL include `sortTitle`, so a bundle's `FieldMap` or `SearchFieldMap` may map a
bare `sortTitle` key without declaring a plugin Media field. The SDK SHALL NOT apply any language
rule to the value.

#### Scenario: Bundle maps sortTitle

- **WHEN** a scraper's `FieldMap` maps `sortTitle` to a response path
- **THEN** bundle validation accepts it as a built-in route

#### Scenario: Mis-cased key

- **WHEN** a scraper's `FieldMap` maps `sorttitle`
- **THEN** bundle validation rejects it as an unknown bare key
