# party-feature Specification

## Purpose
Lets a plugin offer core's generic party session (ADR-0237) through `FeatureEligibility` and name
the session's parts in its own words through `Bundle.PartyTerms`, so core stays free of
karaoke-specific wording.

## Requirements

### Requirement: A plugin may offer the party session

`FeatureEligibilityCapabilities()` SHALL include `party` (`FeatureParty`). A bundle that lists it in
`FeatureEligibility` SHALL be valid; core uses it to decide which Libraries may host a party.

#### Scenario: Party declared

- **WHEN** a bundle lists `party` in `FeatureEligibility`
- **THEN** bundle validation accepts it

### Requirement: A plugin may name the party's parts

`Bundle.PartyTerms` SHALL be optional. Each of its terms (`Party`, `Participant`, `Participants`,
`Request`, `Requests`, `Stage`) SHALL be absent or a valid locale map with an English value. A plural
term SHALL NOT be declared without its singular. `PartyTerms` SHALL NOT be declared unless
`FeatureEligibility` lists `party`. Absent terms mean the host's neutral words.

#### Scenario: Terms with the feature

- **WHEN** a bundle lists `party` and declares `PartyTerms` with `Participant` "Singer"
- **THEN** bundle validation accepts it

#### Scenario: Terms without the feature

- **WHEN** a bundle declares `PartyTerms` but does not list `party`
- **THEN** bundle validation rejects the bundle

#### Scenario: Plural without singular

- **WHEN** `PartyTerms` declares `Participants` but not `Participant`
- **THEN** bundle validation rejects the bundle

#### Scenario: No English value

- **WHEN** a declared term has only a French value
- **THEN** bundle validation rejects the bundle
