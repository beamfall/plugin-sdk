## ADDED Requirements

### Requirement: Scraper search templates may interpolate Lucene-escaped queries

`SearchTemplate` SHALL accept `{query.lucene}` for non-`rss` scrapers. The host SHALL escape the
free-text query for Apache Lucene query parser syntax (`\ + - ! ( ) : ^ [ ] " { } ~ * ? | & /`)
before applying the same transport escaping as `{query}`.

#### Scenario: Title with Lucene special characters in a hinted MusicBrainz query

- **WHEN** `SearchTemplate` is `/recording?query=({query.lucene}){hint.artist_clause}&fmt=json` and the free-text query is `Smile :)`
- **THEN** `{query.lucene}` renders as `Smile+%5C%3A%5C%29` in the compiled search URL

### Requirement: Lucene query placeholder is limited to SearchTemplate

Bundle validation SHALL reject `{query.lucene}` outside `SearchTemplate`.

#### Scenario: Lucene query placeholder outside SearchTemplate

- **WHEN** `LookupTemplate`, `ItemLookupTemplate`, `LookupByIDTemplate`, `EntityMap.LookupTemplate`, or `ChildListing.ListTemplate` contains `{query.lucene}`
- **THEN** bundle validation returns an error
