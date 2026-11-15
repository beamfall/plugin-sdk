## ADDED Requirements

### Requirement: Parse results may carry a version label

`ParsedMedia` SHALL carry an optional `VersionLabel` naming which version of a title a file is. An
empty label SHALL mean the default version, and a parser that never sets it SHALL behave exactly as
before.

#### Scenario: Labelled file

- **WHEN** a parser recognizes a version label in a filename
- **THEN** it returns that label in `VersionLabel` alongside the title, year and ordinals it would
  return for the unlabelled file

#### Scenario: Unlabelled file

- **WHEN** a filename carries no version label
- **THEN** `VersionLabel` is empty

### Requirement: Version labels are bounded printable text

The SDK SHALL accept as a version label only the empty string or trimmed valid UTF-8 text of at most 64 bytes
in which every rune is printable (`unicode.IsPrint`; the ASCII space is the only space allowed, and a
literal U+FFFD is accepted), so control, format and separator runes such as U+200B, U+2028, U+2029
and U+FEFF are rejected, and the conformance runners SHALL fail a vector whose
parse result breaks that rule.

#### Scenario: Untrimmed label

- **WHEN** a parser returns a label with leading or trailing whitespace
- **THEN** `ValidateVersionLabel` returns an error and `sdktest.RunConformance` fails the vector
