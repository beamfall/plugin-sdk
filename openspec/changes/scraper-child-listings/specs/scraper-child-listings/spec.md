## ADDED Requirements

### Requirement: Scraper child listing declarations

The SDK SHALL accept up to three ordered child listings on an `http-json` scraper with a search
template. Each level SHALL have a unique lower-kebab label, an endpoint-relative list template,
and valid JSON paths for the rows, row key, and display label. The row cap SHALL be between 0
and 500 inclusive, with zero selecting the default of 200.

#### Scenario: An ordered declaration is valid

- **WHEN** a scraper declares two or three valid child listings under a search candidate
- **THEN** bundle validation accepts the declaration without assigning domain-specific meanings to the levels

#### Scenario: A template can reference the preceding selection

- **WHEN** a list template uses the candidate ID or declared configuration placeholders
- **THEN** those placeholders are permitted at every level
- **AND** the first level forbids `{parent}`, `{sequence}`, and `{index}`, and every later level uses at least one of them
- **AND** hint placeholders are forbidden at every level

#### Scenario: Bundle validation refuses an invalid listing

- **WHEN** a declaration violates the transport, depth, label, template, path, or row-cap requirements
- **THEN** the SDK refuses the bundle and identifies the plugin, scraper, and one-based level

#### Scenario: The declaration is omitted

- **WHEN** a scraper declares no child listings
- **THEN** its validation and JSON representation are unchanged

### Requirement: Child key segment validation

The SDK SHALL expose `ValidChildKeySegment` for checking a previous row key before URL
substitution. The host SHALL also verify the key was returned by the previous level.

#### Scenario: A supplied key cannot alter path structure

- **WHEN** a key is empty, equals `.` or `..`, or contains `/`, `\`, `?`, `#`, `%2f`, or `%2F`
- **THEN** `ValidChildKeySegment` rejects it
