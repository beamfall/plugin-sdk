# Add a square artwork slot

Ticket CORE-0468 (`queue:core:main`). SDK scope only.

## Why

The artwork vocabulary has no 1:1 slot, so album and podcast cover art is declared as `poster`
and rendered in a 2:3 frame. ADR-0003 treats the vocabulary as fixed and clients skip unknown
slots, so adding `square` is additive.

## Delivery

Add `square` to `ArtworkSlots()`, test that a bundle mapping it validates and unknown slots stay
rejected, regenerate the reference docs and note the change in `COMPATIBILITY.md`. Tagging,
core rendering and plugin adoption are follow-up work.
