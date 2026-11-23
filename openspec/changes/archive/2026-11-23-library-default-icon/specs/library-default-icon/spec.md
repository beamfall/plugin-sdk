## ADDED Requirements

### Requirement: A bundle may declare its default Library icon

`LibraryDefaults.Icon` SHALL be empty or a name from `LibraryIcons()`. A non-empty value SHALL be
the default icon for Libraries bound to the plugin and the placeholder glyph clients draw for the
plugin's Media without artwork. Bundle validation SHALL reject a name outside the vocabulary.

#### Scenario: Declared

- **WHEN** a bundle sets `LibraryDefaults.Icon` to `mic`
- **THEN** bundle validation accepts it

#### Scenario: Unknown name

- **WHEN** a bundle sets `LibraryDefaults.Icon` to a name outside `LibraryIcons()`
- **THEN** bundle validation rejects the bundle

#### Scenario: Not declared

- **WHEN** a bundle leaves `LibraryDefaults.Icon` empty
- **THEN** bundle validation accepts it and hosts keep their class-derived default
