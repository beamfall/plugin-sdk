## ADDED Requirements

### Requirement: Scrapers may describe their credential

`ScraperSpec` SHALL accept `CredentialHelp`, which names what the owner enters for the scraper's
AuthRef and optionally links to where to obtain it, so clients present a credential that is not an
API key (a subscriber PIN) under its own name.

#### Scenario: Credential is a subscriber PIN

- **WHEN** a scraper's AuthRef is a subscriber PIN and it declares `CredentialHelp` with the label
  "Subscriber PIN" and a subscribe link
- **THEN** the bundle validates and clients can show that label and link for the credential

### Requirement: Credential help is validated

Validation SHALL reject credential help on a scraper without an AuthRef, a label without an `"en"`
anchor, a help URL that is not an absolute https URL, and a help URL or help label declared without
the other.

#### Scenario: Plain-http help link

- **WHEN** credential help declares an `http://` help URL
- **THEN** `ValidateBundle` returns a credential help error
