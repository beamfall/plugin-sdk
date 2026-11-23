# Declare a plugin's default Library icon

Ticket: CORE-3336 (`queue:core:main`). Governing ADR: ADR-0055 §2.4.
Status: proposed.

## Why

A card for Media with no artwork draws a placeholder glyph chosen from the media class, so a
karaoke song draws the same music note as an album track. Core is domain-agnostic and cannot tell
them apart. The Library icon vocabulary (`film`, `note`, `mic`, …) already names the symbols
clients draw for a Library, but a plugin cannot pick one: the default comes from media classes.

## What Changes

- `LibraryDefaults.Icon string`: a name from the closed `LibraryIcons()` vocabulary. A host uses it
  as the default icon for Libraries bound to the plugin and passes it to clients as the placeholder
  glyph for the plugin's Media that have no artwork.
- `LibraryIcons()` publishes the vocabulary; validation rejects a name outside it. Empty keeps the
  host's class-derived default.

## Non-goals

- No per-Media or per-class icon, and no new symbols: the vocabulary is the one hosts already use.

## Failure modes

- A host or client that predates the field ignores it and keeps the class-derived glyph.

## Acceptance evidence

Unit tests that a known icon validates and an unknown one is rejected; `script/gate.sh` on the
branch head.

## Rollback

Additive field: revert the commit. Bundles that never set `Icon` are unaffected.
