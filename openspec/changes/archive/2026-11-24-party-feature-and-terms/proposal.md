# Let a plugin offer core's party session and name its parts

Ticket: CORE-3486 (`queue:core:main`). Implements ADR-0237 §1 and §2.1 on the SDK side; CORE-3487
(karaoke plugin) and CORE-3488 (core and web) consume it.

## Why

ADR-0237 makes the party session a generic core feature: "any plugin may offer it", and "karaoke",
"singer" and "song" are plugin labels. The SDK has no way to say either, so core offers a party on
every Library and its clients hard-code karaoke wording.

## What Changes

- `FeatureParty` (`"party"`) joins the closed `FeatureEligibilityCapabilities()` vocabulary. A
  plugin that declares it lets core run a party session over its Libraries.
- `Bundle.PartyTerms` (optional) names the party's parts as locale maps: the party itself, a
  participant and its plural, a request and its plural, and the stage. Unset terms fall back to
  core's neutral words.
- Validation: each declared term is a valid locale map with an English anchor; a plural needs its
  singular; declaring `PartyTerms` without `FeatureParty` is rejected.

## Non-goals

- No party behaviour or settings in the SDK: rotation, join codes and screens stay core's (ADR-0237).
- No pluralization rules: hosts phrase counts in their own copy.

## Failure modes

- A core on an older SDK rejects a bundle that declares `party` or `PartyTerms`, so plugins set them
  only once core pins this release.

## Acceptance evidence

Unit tests for the vocabulary, terms validation and JSON shape; `openspec validate --all --strict`;
`script/gate.sh` on the branch head.

## Rollback

Additive: revert the commit. Bundles that declare neither are unaffected.
