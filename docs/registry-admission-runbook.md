# Registry admission runbook

Registry admission draws a hard line between **mechanical admission** (this SDK's job) and **manual
review** (a human's job). This runbook is the human half.

## Temporary first-party-only admission rule

As of 2026-08-18, third-party submissions are **CLOSED**. `catalogindex` rejects every plugin ID
outside the explicit first-party allow-set, and manual review cannot override that fail-closed
rule. The schema version 1 index could not carry licence, notice, provenance or source-offer
metadata. Schema version 2 (below) can, but opening third-party admission is a separate owner
decision. Until then the first-party allow-set stays.

## Signed legal metadata (catalog index schema version 2)

Every v2 catalog entry carries signed legal metadata (`catalogindex.Legal`):

- a canonical SPDX licence expression. `NONE` and `NOASSERTION` are rejected.
- copyright/NOTICE file references (relative path plus SHA-256).
- the exact source repository, commit and source-archive digest, and a source offer.
- the build-provenance digest.
- a rights attestation. A reviewer key signs the entry's release, legal and policy fields. The
  attestation records the reviewer key id, reviewer identity, review date and disposition.

The index embeds the operator's **signed licence policy** (`catalogindex.SignedPolicy`). A registry
root key signs it. The policy lists:

- the accepted SPDX expressions, matched exactly, and which of them need a source offer;
- the reviewer keyring (key id, identity, public key);
- the maximum attestation age.

The index payload carries the policy's SHA-256. The registry signature therefore binds the policy
digest, every reviewer identity and every review date. `catalogindex.ParseAndVerify` rejects all of
the following, fail-closed:

- an omitted or malformed field;
- an expression the policy does not list;
- a reviewer outside the keyring, or an identity that differs from the keyring's;
- a disposition other than `approved`;
- a stale, early or expired attestation;
- a policy that is unsigned, altered or issued after the index;
- a digest mismatch.

**Who the policy reviewer is.** The policy reviewer is whoever the catalog operator's signed policy
names. For Beamfall that is the owner. The SDK hard-codes no reviewer. An attestation records the
named reviewer's decision under that policy. It is not a legal opinion, and nothing in this
repository is a counsel review.

**No production policy ships here.** The policy in `catalogindex` tests is a labelled test fixture,
not a licence decision. The catalog operator (the owner, for Beamfall) must supply and sign the
production policy, reviewer keyring and keys. This repository does not decide which licences are
acceptable.

**Existing v1 indexes.** `ParseAndVerify` rejects a v1 document with `ErrLegacySchema`; nothing is
grandfathered. To migrate, the operator:

1. runs `catalogindex.MigrateV1`. It verifies the v1 signature and attaches one legal record per
   plugin. A missing or extra record fails.
2. has each entry attested (`SignRightsAttestation`).
3. signs a new v2 index at a higher sequence (`SignIndex`).
4. checks it with `ParseAndVerify`.

## What is mechanical (never a human call)

`sdktest.Admit` (see [`sdktest` reference](reference/sdktest.md)) is the entire mechanical
admission pipeline the registry runs against every submitted listing manifest, in order:

1. **Manifest** — `sdk.ValidateBundle` against the declared `Bundle`: valid plugin ID, semver,
   locale maps, vocabularies, Sources, Layouts.
2. **Lifecycle** — `sdk.ValidateBundles` against the candidate plus the registry's already-admitted
   set: no plugin-ID or Entity-Kind-name collision (fail-closed, all-or-nothing load). The admitted
   set is validated alone first, so a registry broken independent of
   the candidate is rejected as "registry admitted set is invalid independent of candidate" —
   attributed to registry state, never blamed on the candidate.
3. **Compat range** — `sdk.CompatibleWithSDK`: the listing's declared `[SDKFloor, SDKCeiling]`
   compat range must cover the registry's reference SDK version.
4. **Legal** — the listing's `LegalEvidence` must name the same plugin ID, version and SDK range as
   the bundle. It must verify against the operator's signed policy (`catalogindex.VerifyEntry`). The
   artifact, source, provenance and notice bytes must match their digests
   (`catalogindex.VerifyEntryBytes`). Missing evidence rejects.
5. **Conformance** — the artifact's own `sdktest`-based suite (`RunConformance` /
   `RunScoreConformance` / `RunBundleConformance`) must pass under `go test`.

`Admit` stops at the first failing gate and returns the failing `GateClass` plus a reason string.
There is no discretion here — a listing either clears all five gates or it is rejected with a
named, reproducible reason. Curation beyond these five gates is **not editorial**: taste, genre,
or subject matter are never admission criteria.

## What is manual (a human call, every time)

Two things fall outside what any of the five gates can check, because they require judgment a
mechanical rule cannot encode. The legal gate checks that legal metadata is complete, consistent
and attested by a reviewer the signed policy names. It cannot judge the following:

- **Legality** — does the artifact (code, bundled assets, declared scraper targets, imagery)
  violate law in a jurisdiction the registry operates in, or infringe a third party's rights.
- **Malware / abuse** — does the artifact attempt anything the SDK's static/mechanical gates don't
  catch: obfuscated intent, supply-chain tampering, an attempt to exfiltrate data outside the
  declared boundary, or social-engineering content in listing copy.

Every submitted listing that clears the five mechanical gates queues for this review before the
registry marks it publicly discoverable. Response-time targets and reviewer staffing are an
operational (registry-side) commitment, not an SDK contract, but this function must be
**staffed before third-party submissions open**.

### Reviewer steps

1. Confirm the listing passed `sdktest.Admit` (mechanical gates are a precondition, not part of
   this review — do not re-litigate manifest/lifecycle/compat-range/legal/conformance here).
2. Read the declared `Bundle.DisplayName`, `MediaLabel`, and any bundled scraper/source
   declarations for legality red flags (copyright-infringing intent, illegal-content facilitation).
3. Run the artifact's declared import/network surface against the SDK's network
   boundary.
   `sdktest.RequireAllowedImports` is a test-time check that already ran under `go test` via the
   sdktest runners in the author's own CI — a targeted denylist plus its evasion closures (denied
   imports, dot-import ban, `http.Client`/`http.Transport` construction ban, function-value
   capture), NOT a full dependency allowlist. Third-party dependencies are therefore reviewed via
   `go.mod` here, in this manual review; the reviewer also owns the judgment calls no static check
   can make — e.g. a legitimate-looking HTTP call to an abuse-pattern host.
4. Record the decision (approve / reject with reason / escalate) in the registry's review log.
5. On reject, the reason is author-facing; on escalate, route per the registry's escalation
   rotation.

Steps 4–5 are registry-side process: the canonical review-log and escalation procedure lives in
the registry's own (owning) repo, and this runbook only cites it — it never restates that process.

### What this runbook does not own

Signing, revocation, delivery-gating, and payment are registry-side and out of
scope for the SDK. This runbook covers only the human judgment step between "cleared the
mechanical gates" and "publicly discoverable."
