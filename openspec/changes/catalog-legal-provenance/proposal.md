# Carry signed licence, notice, provenance and rights metadata in the catalog index

Ticket: CORE-0523 (`queue:core:main`, PUBLEGAL-SDK-CATALOG-1).
Status: proposed.

## Why

The v1 signed catalog index names a plugin, a version and an artifact digest, and nothing else. It
cannot say under which licence the artifact is distributed, where its notices and corresponding
source are, which source produced it, or who reviewed its distribution rights. Catalog admission
therefore stays first-party-only, and nothing mechanical stops an entry without that information
from being published.

The ticket held this work for an outside counsel decision. On 2026-11-10 the owner waived that hold
and named himself the deciding authority. This change does not encode any legal conclusion: the
licence policy is data that the catalog operator signs, and the SDK only verifies it.

## What Changes

- `catalogindex` schema version 2. Every entry carries `legal`: an SPDX licence expression, notice
  files (path and SHA-256), source repository, commit and source-archive SHA-256, an optional source
  offer, a build-provenance SHA-256 and a rights attestation (reviewer key id, reviewer identity,
  review date, disposition, statement SHA-256 and reviewer signature).
- The signed index document embeds an operator-signed licence policy: the allowed SPDX expressions,
  each with its source-offer rule, the policy-reviewer keyring (key id, identity, Ed25519 key) and the
  maximum attestation age. The index payload carries the policy's SHA-256, so the index signature
  covers the policy binding and every reviewer field.
- `ParseAndVerify` rejects, fail-closed and with a named error: a v1 document (`ErrLegacySchema`), a
  missing or unsigned policy, a policy digest mismatch, invalid or non-canonical SPDX, `NONE` or
  `NOASSERTION`, an expression the policy does not list, missing notices, a missing source offer the
  policy requires, malformed source or provenance digests, a reviewer outside the keyring or with a
  different identity, a disposition other than `approved`, an attestation whose statement digest or
  signature does not match the entry, and an attestation older than the policy or the maximum age.
- `MigrateV1` verifies a v1 document and turns its entries into v2 entries only when the caller
  supplies a legal record for every entry and for no other plugin. Nothing is grandfathered: the
  operator still attests and signs a new v2 index.
- Producer helpers `SignPolicy`, `SignRightsAttestation`, `SignIndex` and `VerifyEntryBytes` let the
  registry build and check entries with its own keys.
- `sdktest.Admit` gains a `legal` gate (before conformance): the listing must carry its catalog
  entry, the signed policy and the artifact, source and notice bytes, and every digest must match.
- `script/gen-docs_test.sh` regenerates the docs twice in a scratch copy and fails on drift; the
  gate runs it.

## Non-goals

- Deciding which licences are compatible or redistributable. No production policy, keyring or
  reviewer ships with the SDK; the operator must supply a signed policy. Test fixtures carry a
  policy labelled as a test fixture.
- Opening third-party admission. The first-party allow-set stays in force; lifting it is a separate
  owner decision.
- Fetching or verifying artifacts at install time in Core, and updating the Cloud generator and the
  workspace contract registry. Those consumers move to v2 in their own repositories.

## Failure modes

- A consumer still pinned to v1 rejects a v2 document as an unknown schema; a v2 consumer rejects a
  v1 document as `ErrLegacySchema`. Both fail closed; neither installs from an unverified catalog.
- An operator who rotates the policy invalidates every attestation made under the old digest; the
  index will not verify until each entry is re-attested.

## Acceptance evidence

- Mutation tests for every rejection above, round-trip and signature/keyring tests, and a fuzz
  target over `ParseAndVerify`.
- `script/gen-docs_test.sh` passes; `script/gate.sh` passes on the lane head.
- Manual rows (legal approval, human release approval) are NOT_RUN: waived by the owner on
  2026-11-10.

## Rollback

Revert the change. v1 documents verify again; v2 documents are rejected as an unknown schema.
