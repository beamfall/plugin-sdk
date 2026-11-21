# kind-plural-display-name Specification

## Purpose
How a plugin declares the optional plural of an entity kind's display name, which clients use to head a
group of several entities of that kind.

## Requirements

### Requirement: Entity kinds may declare a plural display name

`EntityKind.PluralDisplayName` SHALL be an optional locale map holding the plural of the kind's
`DisplayName`. It SHALL be validated like `DisplayName`, and a kind that declares a plural SHALL also
declare a `DisplayName`. A kind without a plural SHALL validate unchanged.

#### Scenario: Valid plural

- **WHEN** a kind declares `DisplayName` "Album" and `PluralDisplayName` "Albums"
- **THEN** `ValidateBundle` accepts it

#### Scenario: Plural without a singular

- **WHEN** a kind declares a `PluralDisplayName` and no `DisplayName`
- **THEN** `ValidateBundle` returns an error

#### Scenario: Malformed plural

- **WHEN** a kind's `PluralDisplayName` has no English entry or a blank value
- **THEN** `ValidateBundle` returns an error

#### Scenario: Absent plural

- **WHEN** a kind declares no `PluralDisplayName`
- **THEN** `ValidateBundle` behaves as before
