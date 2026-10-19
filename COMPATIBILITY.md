# Compatibility

`github.com/beamfall/plugin-sdk` is pre-1.0. The current release is `v0.3.2`.

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
