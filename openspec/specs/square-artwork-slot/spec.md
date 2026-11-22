# square-artwork-slot Specification

## Purpose
Lets a plugin declare 1:1 square cover art as an artwork slot, so hosts can draw square
artwork (album and feed covers) without naming a domain.

## Requirements

### Requirement: Square artwork slot

The SDK SHALL include `square` (1:1 cover art) in the artwork slot vocabulary. Unknown slots
SHALL remain rejected.

#### Scenario: A bundle maps the square slot

- **WHEN** a scraper `ArtworkMap` names `square`
- **THEN** `ValidateBundle` accepts the bundle

#### Scenario: An unknown slot is mapped

- **WHEN** a scraper `ArtworkMap` names a slot outside the vocabulary
- **THEN** `ValidateBundle` rejects the bundle
