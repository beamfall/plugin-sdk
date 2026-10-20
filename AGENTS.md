# AGENTS.md — Beamfall Plugin SDK

The Go contract that teaches [Beamfall Core](https://github.com/beamfall/core) a new kind of
library. Every first-party Beamfall plugin builds on this module (`github.com/beamfall/plugin-sdk`),
so exported API changes reach every plugin.

> **This file is canonical.** `CLAUDE.md` is a symlink to it — edit `AGENTS.md`, never maintain a
> second copy.

> **Documentation:** keep public docs in `README.md`, `COMPATIBILITY.md`, `CONTRIBUTING.md`,
> `SECURITY.md`, `docs/` and this file. When this repo is checked out inside the Beamfall multi-repo
> workspace, also follow that workspace's `DOCUMENTATION-GOVERNANCE.md` before adding, moving, or
> archiving any doc.

Use normal repo-local CLI and shell tools.

## Layout

- Module root — the `Bundle` manifest and its fail-closed validation, fixed vocabularies, entity,
  field, layout, scraper and importer specs, the `FilenameParser`/`MatchScorer` seams and locale
  strings.
- `sdktest/` — conformance harness plugins run from their own tests; `scrobble/`, `catalogindex/`,
  `compatibility/` — supporting packages (`compatibility/matrix.json` is the recorded matrix).
- `contracts/` — downstream conformance receipt schema and validator.
- `docs/` — hand-written guides plus generated `docs/reference/`; `llms.txt`, `llms-full.txt`.
- `script/` — gate, docs generator (`gen-docs.sh`, `godoc2md/`) and contract tests; `tools/` —
  pinned tool module; `testing/coverage-floor.json` — per-package coverage floors.

## Build and test

- `go test ./...` — unit tests, including the `sdktest` harness's own tests.
- `script/gate.sh` — full local gate: gate concurrency and SDK consumer fan-out contracts, release
  identity, gofmt, vet, test, per-package coverage, golangci-lint. Requires `golangci-lint` on
  `PATH`.
- `.github/workflows/` — `conformance.yml` (lint plus downstream plugin conformance) and
  `test-depth.yml` (required parser fuzz targets). Keep the local gate and the workflows in step.

## Conventions

- `.golangci.yml` is the canonical lint rule set for Beamfall plugins; plugin repos copy it verbatim
  and pin the same `golangci-lint` version as `conformance.yml`. Change it here first.
- Docs: run `script/gen-docs.sh` after any exported-API or doc-comment change and commit the
  outputs. Hand-authored prose lives only in `README.md`, `COMPATIBILITY.md`,
  `docs/plugin-authoring.md` and `docs/registry-admission-runbook.md`; never hand-edit
  `docs/reference/*.md`, `llms.txt` or `llms-full.txt`.
- Exported API follows the versioning policy in `COMPATIBILITY.md`: patch releases stay
  source-compatible; a pre-1.0 breaking change ships in a minor release together with coordinated
  first-party plugin updates.

## Corvint context

Before editing, read `corvint context "<task>"` or `corvint impact <path>...`; before choosing tests,
run `corvint affected --base <origin/main sha>`. **Verify:** `script/gate.sh`.

## Contributing and license

- Licensed under the GNU AGPL v3 (`LICENSE`). Preserve license headers and third-party notices.
- External contributions need an accepted Beamfall Contributor License Agreement first: see
  `CONTRIBUTING.md` and `CLA.md`. Report vulnerabilities as described in `SECURITY.md`, not in
  public issues.
