# Registry admission runbook

Registry admission draws a hard line between **mechanical admission** (this SDK's job) and **manual
review** (a human's job). This runbook is the human half.

## Temporary first-party-only admission rule

As of 2026-08-18, third-party submissions are **CLOSED**. Only plugin IDs for Beamfall-authored,
first-party plugins may be admitted to the catalog until the catalog index can carry
license, notice, and source-offer metadata. The v1 catalog index has no license, notice, provenance, or source-offer
capability, so it cannot convey a third-party plugin while preserving the notices and source offer
that its rights may require. `catalogindex` therefore rejects every ID outside the explicit
first-party allow-set; manual review cannot override this fail-closed rule.

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
4. **Conformance** — the artifact's own `sdktest`-based suite (`RunConformance` /
   `RunScoreConformance` / `RunBundleConformance`) must pass under `go test`.

`Admit` stops at the first failing gate and returns the failing `GateClass` plus a reason string.
There is no discretion here — a listing either clears all four gates or it is rejected with a
named, reproducible reason. Curation beyond these four gates is **not editorial**: taste, genre,
or subject matter are never admission criteria.

## What is manual (a human call, every time)

Two things fall outside what any of the four gates can check, because they require judgment a
mechanical rule cannot encode:

- **Legality** — does the artifact (code, bundled assets, declared scraper targets, imagery)
  violate law in a jurisdiction the registry operates in, or infringe a third party's rights.
- **Malware / abuse** — does the artifact attempt anything the SDK's static/mechanical gates don't
  catch: obfuscated intent, supply-chain tampering, an attempt to exfiltrate data outside the
  declared boundary, or social-engineering content in listing copy.

Every submitted listing that clears the four mechanical gates queues for this review before the
registry marks it publicly discoverable. Response-time targets and reviewer staffing are an
operational (registry-side) commitment, not an SDK contract, but this function must be
**staffed before third-party submissions open**.

### Reviewer steps

1. Confirm the listing passed `sdktest.Admit` (mechanical gates are a precondition, not part of
   this review — do not re-litigate manifest/lifecycle/compat-range/conformance here).
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
