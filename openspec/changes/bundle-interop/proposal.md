# Bundle interop declaration

Ticket: CORE-2810 (`queue:core:main`), part 1 of CORE-2681.
Status: proposed.

## Why

Core's read-only compatibility surfaces for external automation tools (CORE-2681) must tell each tool
what type every listed item has and what collection each library holds. Those words are plugin domain
vocabulary, which Core never spells (ADR-0171 §2.3), so the mapping has to come from plugin data, as
`ScrobbleTargetSpec` already does for scrobble adapters.

## What Changes

- `Bundle.Interop *InteropSpec` with `MediaField`, `Media`, `Kinds` and `Scopes`, each list made of
  `InteropMapping{Value, Type}`.
  - `Media` maps exact values of one declared text Media field to item types.
  - `Kinds` maps this bundle's container Entity kinds to item types.
  - `Scopes` maps this bundle's declared family scope values to collection tokens.
- Tokens are opaque plugin data of 1-32 ASCII letters. The SDK carries no fixed list of them and names
  no product.
- Validation rejects:
  - an empty spec;
  - a field without mappings, or mappings without a field;
  - an undeclared or non-text field;
  - blank or duplicate values;
  - malformed tokens;
  - undeclared or non-container kinds;
  - two kinds sharing an item type;
  - an item type used by both Media and a kind;
  - undeclared scopes.
- Additive: a bundle without `Interop` validates and behaves unchanged.
- Release `v0.4.6`, which records the first-party plugins at `[0.4.6, 0.4.6]`.

## Non-goals

- No fixed vocabulary of item types or collection tokens in the SDK.
- No streaming, playback or watch-state declaration.
- Core behaviour belongs to CORE-2681 in beamfall/core.

## Failure modes

- A value with no mapping has no fallback. Core lists no item for it and emits no collection token.
- A malformed declaration fails `ValidateBundle`, so the plugin cannot load.

## Acceptance evidence

- `bundle_interop_test.go` covers the valid shapes and every rejection listed above.
- `script/gate.sh` passes.
- filmtv declares `Interop` against the release (CORE-2812).

## Rollback

Revert the commit before the tag. After the tag, release a patch that drops the field. No consumer
reads it until core's CORE-2681 lands.
