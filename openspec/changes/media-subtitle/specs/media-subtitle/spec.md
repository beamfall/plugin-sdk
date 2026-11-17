## ADDED Requirements

### Requirement: A bundle may declare a Media subtitle template

`Bundle.MediaSubtitle` SHALL be an ordered list of locale-mapped template segments. Each `{name}`
placeholder SHALL name a structural ordinal (`sequence`, `index`, `index-end`) or a declared Media
field of type text or number. Validation SHALL reject an unknown placeholder, a segment without a
placeholder and a segment without an English fallback.

#### Scenario: Declared field and ordinals

- **WHEN** a bundle declares the segments `S{sequence}`, `E{index}` and `{episode-title}` and declares
  the text Media field `episode-title`
- **THEN** bundle validation accepts it

#### Scenario: Unknown placeholder

- **WHEN** a segment names `{missing}` and no Media field `missing` is declared
- **THEN** bundle validation rejects the bundle

### Requirement: Hosts render the subtitle per item and locale

`RenderMediaSubtitle` SHALL render each segment in the requested locale, drop a segment whose
placeholders do not all have a value, and join the remaining segments with " · ".

#### Scenario: Missing value drops a segment

- **WHEN** an item has the ordinals 5 and 3 but no `episode-title`
- **THEN** the rendered subtitle is `S5 · E3`

#### Scenario: No values

- **WHEN** an item has none of the named values
- **THEN** the rendered subtitle is empty
