## ADDED Requirements

### Requirement: A bundle may declare that its Media play alone

`Engagement.PlaysAlone` SHALL be a boolean on the bundle's engagement posture. True SHALL mean
that Media in the plugin's Libraries play alone: a host offers the flag to clients so they do not
queue other items after such a Media or auto-advance into or out of it. The zero value SHALL keep
the host's default queue behaviour, and bundle validation SHALL accept either value.

#### Scenario: Declared

- **WHEN** a bundle sets `Engagement.PlaysAlone` to true
- **THEN** bundle validation accepts it and the activated bundle reports the flag as true

#### Scenario: Not declared

- **WHEN** a bundle leaves `Engagement.PlaysAlone` unset
- **THEN** the activated bundle reports the flag as false
