## ADDED Requirements

### Requirement: ParseContext carries optional Library FamilyScope

`ParseContext` SHALL expose `FamilyScope string` carrying the Library's plugin-declared family
scope (a `Value` from `Bundle.FamilyScopes`, or `""` when absent or unknown). Parsers SHALL treat
`""` or unrecognized values as unknown and preserve their legacy `Parse` fallback.

#### Scenario: Absent or unrecognized FamilyScope

- **WHEN** `ParseContext.FamilyScope` is `""` or an unrecognized value and `DirectoryMediaFileCount` is unknown
- **THEN** `ParseWithContext` preserves the legacy `Parse` result

#### Scenario: Recognized FamilyScope overrides legacy classification

- **WHEN** `ParseContext.FamilyScope` is a declared scope such as `"tvshows"`
- **THEN** `RunContextConformance` allows `ParseWithContext` to differ from `Parse` even when `DirectoryMediaFileCount` is unknown
