# source-optional-value Specification

## Purpose
TBD - created by archiving change next-up-media-class-value. Update Purpose after archive.

## Requirements

### Requirement: A next-up Source may name a media class

A `next-up` Source's Value SHALL be empty or a name from `OptionalValueSourceKinds()["next-up"]`
(`video`, `audio`). Empty SHALL mean every class. Bundle validation SHALL reject any other Value.

#### Scenario: No class

- **WHEN** a bundle declares a `next-up` Source with no Value
- **THEN** bundle validation accepts it

#### Scenario: A class

- **WHEN** a bundle declares a `next-up` Source with Value `audio`
- **THEN** bundle validation accepts it

#### Scenario: Unknown class

- **WHEN** a bundle declares a `next-up` Source with Value `image`
- **THEN** bundle validation rejects the bundle
