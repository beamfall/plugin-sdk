# Kind plural display name

Ticket: CORE-2820 (`queue:core:main`). Covers item 1 of CORE-2493.
Status: proposed.

## Why

Clients head a group of related Entities with the kind's `DisplayName`, which is singular. An artist
page therefore heads four albums "Album" where Plex says "Albums". Plurals are plugin locale data,
and Core never spells plugin vocabulary (ADR-0171 §2.3). The plural has to come from the bundle.

## What Changes

- Add `EntityKind.PluralDisplayName LocaleString`, an optional plural of `DisplayName`.
- Validate it like `DisplayName`. A non-empty plural without a `DisplayName` is rejected.
- Absent means clients fall back to `DisplayName`, then `Name`, as before.
- Additive: a bundle without plurals validates and behaves unchanged.
- Release `v0.4.7`, which records the first-party plugins at `[0.4.7, 0.4.7]`.

## Non-goals

- No count-aware plural rules (CLDR one/few/many). A heading over a group needs one plural form.
- Core and client rendering belong to beamfall/core.

## Failure modes

- A malformed plural fails `ValidateBundle`, so the plugin cannot load.
- A kind with no plural still renders its singular label.

## Acceptance evidence

- `bundle_validate_test.go` covers a valid plural, a malformed plural and a plural without a singular.
- `script/gate.sh` passes.
- music and filmtv declare plurals against the release.

## Rollback

Revert the commit before the tag. After the tag, keep the exported field for all of `v0.4.x`. To
withdraw the feature, Core stops reading the field, which needs no SDK release.
