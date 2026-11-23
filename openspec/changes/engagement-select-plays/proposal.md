# Declare that a plugin's Media play on Select

Ticket: CORE-3307 (`queue:core:main`). Governing ADR: ADR-0055 §2.4.
Status: proposed.

## Why

Some Libraries hold Media that people pick by title to play at once, such as songs to sing. For
those, Select should start the item, not open a detail page, and a client may list them as titled
rows rather than posters. Core is domain-agnostic, so today clients guess from the media class and
missing artwork, which misses video items and items with thumbnails.

## What Changes

- `Engagement.SelectPlays bool`: true means Media in this plugin's Libraries play on Select. A host
  passes it to clients, which start such an item when it is selected, keep Details in a secondary
  action, and may draw it as a titled row or tile.
- No validation: the zero value keeps today's behaviour.

## Non-goals

- No per-Media or per-class override, and no change to queueing (`PlaysAlone`) or resume.

## Failure modes

- A host or client that predates the field ignores it, and Select opens detail as it does today.

## Acceptance evidence

A unit test that the field survives bundle validation; `script/gate.sh` on the branch head.

## Rollback

Additive field: revert the commit. Bundles that never set `SelectPlays` are unaffected.
