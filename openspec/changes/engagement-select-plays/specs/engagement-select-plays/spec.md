## ADDED Requirements

### Requirement: A bundle may declare that its Media play on Select

`Engagement.SelectPlays` SHALL be a boolean on the bundle's engagement posture. True SHALL mean
that Media in the plugin's Libraries are picked by title to play: a host offers the flag to clients
so that Select starts such a Media rather than opening its detail. The zero value SHALL keep the
client's default Select behaviour, and bundle validation SHALL accept either value.

#### Scenario: Declared

- **WHEN** a bundle sets `Engagement.SelectPlays` to true
- **THEN** bundle validation accepts it and the activated bundle reports the flag as true

#### Scenario: Not declared

- **WHEN** a bundle leaves `Engagement.SelectPlays` unset
- **THEN** the activated bundle reports the flag as false
