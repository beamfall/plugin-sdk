## ADDED Requirements

### Requirement: Scrapers may declare cached reference documents

`ScraperSpec` SHALL accept `ReferenceDocuments`, named static documents with a `/`-rooted path and a
cache lifetime in hours (zero meaning one week, at most one year). Map paths SHALL read them through
`{ref.<name>:<path>}` placeholders, replaced by the scalar at `<path>` in the named document before the
map path resolves. Only http-json scrapers SHALL declare them.

#### Scenario: Artwork selected by the source's type id

- **WHEN** an artwork map path reads `{ref.artwork-types:data[slug=background,recordType=series].id}`
- **THEN** the placeholder becomes the id the reference document lists for that slug and record type,
  and the map path selects artwork with that type

#### Scenario: Undeclared reference document

- **WHEN** a map path names a reference document the scraper does not declare
- **THEN** `ValidateBundle` returns a reference document error

### Requirement: Map paths may list alternatives

A map path SHALL accept alternatives separated by `||`, resolved in order, with the first that
resolves winning. A predicate SHALL accept several comma-separated `key=value` conditions, all of which
must match.

#### Scenario: Translation falls back to the original

- **WHEN** a title path is `data.translations.nameTranslations[language=eng].name||data.name` and the
  document has no English translation
- **THEN** the title is `data.name`

### Requirement: Search templates may use the item year

`SearchTemplate` SHALL accept `{year}`, substituted with the year identification knows for the item.
When the year is unknown, a query parameter whose whole value is `{year}` SHALL be dropped from the
request.

#### Scenario: Year unknown

- **WHEN** the template is `/search?query={query}&year={year}` and the year is unknown
- **THEN** the request is `/search?query=<query>` with no `year` parameter
