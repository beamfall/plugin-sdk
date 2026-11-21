## ADDED Requirements

### Requirement: Bundles may declare interop tokens

`Bundle.Interop` SHALL map plugin-owned values to opaque interop tokens:
- values of one declared text Media field to item types;
- container Entity kinds to item types;
- declared family scopes to collection tokens.

A bundle without `Interop` SHALL validate unchanged. A non-nil `Interop` SHALL declare at least one
mapping.

#### Scenario: Valid declaration

- **WHEN** a bundle maps two values of its text field `class`, two container kinds and one declared scope
- **THEN** `ValidateBundle` accepts it

#### Scenario: Empty declaration

- **WHEN** a bundle declares an `Interop` with no mappings
- **THEN** `ValidateBundle` returns an error

#### Scenario: Absent declaration

- **WHEN** a bundle declares no `Interop`
- **THEN** `ValidateBundle` behaves as before

### Requirement: Interop tokens are opaque and well formed

Every interop token SHALL be 1-32 ASCII letters. The SDK SHALL NOT restrict tokens to a fixed vocabulary.

#### Scenario: Malformed token

- **WHEN** a mapping's token contains a space, is empty, or is longer than 32 letters
- **THEN** `ValidateBundle` returns an error

### Requirement: Interop mappings reference declared plugin data

`Media` SHALL be declared together with `MediaField`, and that field SHALL be a declared text Media
field. `Kinds` SHALL name declared container kinds, and `Scopes` SHALL name declared family scopes.
Values SHALL be non-blank and unique within each list.

#### Scenario: Non-container kind

- **WHEN** `Kinds` names a category kind
- **THEN** `ValidateBundle` returns an error

#### Scenario: Undeclared scope

- **WHEN** `Scopes` names a value the bundle's `FamilyScopes` do not declare
- **THEN** `ValidateBundle` returns an error

### Requirement: Item types identify one shape

Two kinds SHALL NOT share an item type, and no item type SHALL appear in both `Media` and `Kinds`.
Several Media values MAY share an item type.

#### Scenario: Kinds share a type

- **WHEN** two kinds map to the same item type
- **THEN** `ValidateBundle` returns an error

#### Scenario: Media and kind share a type

- **WHEN** a Media value and a kind map to the same item type
- **THEN** `ValidateBundle` returns an error
