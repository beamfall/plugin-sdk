# Let a next-up Source name a media class

Ticket: CORE-3341 (`queue:core:main`). Follows CORE-3331.

## Why

Core's Home splits "Up Next" (video) from "Continue Listening" (audio) by giving the `next-up`
Source an optional `video` or `audio` Value. Bundle validation rejects any Value on `next-up`, so a
plugin cannot declare either Section itself.

## What Changes

- `OptionalValueSourceKinds()` publishes Source kinds whose Value is optional, mapped to their closed
  vocabulary. `next-up` takes `video` or `audio`.
- Validation accepts an empty Value or one from that vocabulary on those kinds and rejects any other
  Value. `ValuedSourceKinds()` keeps its meaning: kinds whose Value is required.

## Non-goals

- No `image` class: core's next-up feed covers playable Media only.

## Failure modes

- A host older than CORE-3331 ignores the Value and shows every class, which is the old behaviour.

## Acceptance evidence

Unit tests that `next-up` accepts no Value, `video` and `audio`, and rejects any other Value;
`script/gate.sh` on the branch head.

## Rollback

Additive: revert the commit. Bundles that never set a Value on `next-up` are unaffected.
