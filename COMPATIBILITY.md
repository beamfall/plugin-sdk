# Compatibility

`github.com/beamfall/plugin-sdk` is pre-1.0. The current release is `v0.4.2`,
cut from `main`. It carries the `v0.4.1` per-item lookups and container entity
maps and the `v0.4.0` catalog index schema v2 break, both described below. The
previous line, `v0.3.x`, lives on the `release/v0.3` branch (last release
`v0.3.7`). The compatibility matrix records the first-party plugins re-released
against `v0.4.2` (CORE-2335).

`const Version` in `bundle.go` is the source of truth for the SDK artifact's
exact identity. Released source MUST match the release tag it ships under, and
a release tag is never moved or recreated. Each new release sets `Version` to
the next unused semver before it is tagged.

## Versioning policy

- Patch releases (`v0.1.x`) must be source-compatible for existing exported
  APIs and conformance helpers.
- Minor releases (`v0.x.0`) may break exported APIs while the SDK is pre-1.0,
  but the breaking change must land with coordinated first-party plugin updates.
- Every first-party plugin must require a tagged SDK version in `go.mod`.
  Committed plugin modules must not use `replace github.com/beamfall/plugin-sdk`
  to stand in for versioning.
- Local development may use an uncommitted `go.work` or temporary `replace` to
  test SDK and plugin changes together.
- The SDK keeps the Core/plugin boundary generic: no plugin domain vocabulary,
  Core store types, rendering code, entitlement/account code, or plugin-specific
  schema enters the SDK.
- Context-aware filename parsing is additive: `FilenameParser` and `ParseProbe`
  remain unchanged, while parsers that need scan-owned directory evidence may
  additionally implement `ContextFilenameParser`. `ParseContext` exposes only
  a bounded directory media-file count class (`single` or saturated `multiple`),
  never sibling names or exact unbounded counts. Its zero value is absent/
  unknown; unknown or future values must preserve the legacy `Parse` result.
- Any change that adds exported contract surface, such as that optional seam,
  ships in a coordinated release. Every release keeps a distinct exact
  identity: `bundle.go` `Version` must match the next unused release tag
  exactly before publication. Core
  and first-party plugins may adopt a release only by pinning that module
  version and passing their conformance gates; floating HEAD and committed
  local `replace` pins do not count as release adoption.
- `CompatibleWithSDK` compares versions per SemVer §11 precedence: a pre-release
  (e.g. `0.1.6-rc1`) sorts **below** its release triple (`0.1.6`), so an rc does
  not satisfy a floor set to that release; build metadata (`+…`) is ignored.

## v0.4.2: exact SDK range for every first-party plugin

No API change. `v0.4.1` recorded seven plugins at range `[0.4.0, 0.4.1]`, but a
plugin's conformance at its declared floor resolves that floor's matrix, and
the `v0.4.0` matrix has no evidence for a `[0.4.0, 0.4.1]` range, so the
workspace contract check (which runs conformance at the floor and at the pin)
could not pass. `v0.4.2` re-records all eight first-party plugins at
`[0.4.2, 0.4.2]`. Keep a plugin's floor equal to its ceiling unless the floor
release's own matrix records that exact range.

## v0.4.1: per-item lookups and container entity maps

An additive, source-compatible patch (CORE-2335). `ScraperSpec` gains
`ItemLookupTemplate`, `ItemFieldMap` and `ItemArtworkMap` for a second lookup
per matched item, with the `{sequence}` and `{index}` placeholders, and
`EntityMap` gains `Container` so a scraper can hydrate the container Entities
that hold an item (a show and its season). `CompatibleWithSDK` compares the
exact SDK version against each bundle's ceiling, so the first-party plugins are
re-released with ceiling `0.4.1`; filmtv uses the new fields and also raises its
floor to `0.4.1`.

## v0.4.0: catalog index schema v2

`catalogindex` moves from schema version 1 to version 2 (CORE-0523). This is an
API and wire break, so it ships as a minor release.

- Every `Entry` carries a signed `Legal` record: a canonical SPDX licence
  expression, notice files with SHA-256 digests, the source repository, source
  commit and source-archive digest, an optional source offer, a
  build-provenance digest, and a rights attestation signed by a reviewer named
  in the policy keyring.
- `SignedIndex` embeds the operator's signed licence `Policy` (allowed SPDX
  expressions and their source-offer rules, the reviewer keyring, the maximum
  attestation age), and `Index.PolicySHA256` binds the index to it.
  `ParseAndVerify` checks the registry signature, the policy signature and
  binding, and every entry's legal metadata, failing closed on any gap.
- `ParseAndVerify` rejects a version 1 document with `ErrLegacySchema`.
  `MigrateV1` verifies a v1 document's signature and turns its entries into v2
  entries given one legal record per plugin; the operator then attests each
  entry (`SignRightsAttestation`) and signs a new v2 index at a higher sequence
  (`SignIndex`).
- New exported surface: `Legal`, `Notice`, `RightsAttestation`, `Policy`,
  `PolicyLicense`, `PolicyReviewer`, `SignedPolicy`, `EntryBytes`,
  `PolicyDigest`, `AttestationStatement`, `VerifyEntry`, `VerifyEntryBytes`,
  `SignPolicy`, `SignRightsAttestation`, `SignIndex`, `MigrateV1` and the
  legal error sentinels.

Consumers of `catalogindex` (core and beamfall-cloud) must publish and read v2
catalogs when they adopt `v0.4.0`; a v1 catalog no longer verifies. The
first-party plugins are re-released against `v0.4.0` in the same coordinated
step, and the compatibility matrix is re-recorded from those releases.

## Compatibility check

Before checking an SDK release candidate, set `bundle.go` to the next unused
release semver, then run the gate in candidate mode with the matching tag (for
example, for a `0.3.3` release):

```sh
git fetch --tags origin
BEAMFALL_SDK_CANDIDATE_TAG=v0.3.3 script/gate.sh
test -z "$(git ls-remote --tags origin refs/tags/v0.3.3)"
```

Candidate mode checks only the fetched local tag refs and rejects a proposed
tag that is already present. The gate also inspects every local `v*` tag
pointing at `HEAD` and requires each to equal `v${Version}`. The publisher
performs the final `ls-remote` check immediately before publication; remote
inspection is deliberately not part of normal development or CI gates. On a
GitHub tag event, the same Go test reads `GITHUB_REF_TYPE=tag` and
`GITHUB_REF_NAME` and enforces the same identity. With no candidate, tag event,
or `v*` tag at `HEAD`, an ordinary untagged build remains valid.

For each first-party plugin:

```sh
go list -m github.com/beamfall/plugin-sdk
go test ./...
```
