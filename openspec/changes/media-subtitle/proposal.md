# Declare a per-Media subtitle template

Ticket: CORE-1226 (`queue:core:main`), consumed by CORE-2386.
Status: proposed.

## Why

Clients render sibling Media in an open folder as identical cards: an episode carries the show's
poster, title and year, so a season shows twelve copies of the same card. Core is domain-agnostic
and cannot know that a filmtv item is "S5 · E3 · <title>". The plugin that owns the vocabulary has
to say how its items are told apart, and the SDK has no field for it.

## What Changes

- `Bundle.MediaSubtitle []LocaleString`: ordered segments, each a template whose `{name}`
  placeholders name a declared Media field (text or number) or a structural ordinal
  (`sequence`, `index`, `index-end`).
- `RenderMediaSubtitle(segments, locale, lookup)` renders the segments for one item in one locale:
  a segment whose placeholders are not all present is dropped, the rest join with " · ". Numbers
  are the host's job to format; the lookup returns display text.
- `MediaSubtitleLocales(segments)` lists the locale tags a host renders.
- Validation: every segment needs an English fallback, every placeholder must name a structural
  ordinal or a declared text/number Media field, and each segment has at least one placeholder.

## Non-goals

- No storage: the template renders over fields core already stores, so existing libraries need no
  rescan. No new field types, no formatting directives inside placeholders.

## Failure modes

- A bundle naming an undeclared field fails validation at load, as other bundle errors do.
- An item missing a value drops that segment; an item missing every value has no subtitle and the
  client keeps its current card.

## Acceptance evidence

Unit tests for validation and rendering; `script/gate.sh` on the branch head.

## Rollback

Additive field: revert the commit. Bundles that never declare `MediaSubtitle` are unaffected.
