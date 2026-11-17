# Container hydration and per-item lookups for scrapers

Ticket: CORE-2335 (`queue:core:main`).
Status: proposed.

## Why

A scraper today hydrates Entities only from a non-empty `EntityMap.Path` and only by creating an Entity
and linking it to the identified Media through a Media-targeted relation. Ordered containers that the
host already built from the folder layout (a series and its numbered sub-container) therefore never
receive the poster, overview or dates the lookup document carries: the document root is not a valid
path, and the container is not related to the Media under any relation a scraper may name.

The same scrapers also map every item of a container from the container's document, so an item's
summary, year and still come from the container instead of the item. Sources such as TMDB serve a
per-item document addressed by the container id plus the item's sequence and index, but templates
cannot name those values.

## What Changes

- Documented `{sequence}` and `{index}` placeholders: the parsed sequence (the outer ordinal) and index
  (the item's position within it) of the looked-up Media. They are substituted in `LookupTemplate`,
  `ItemLookupTemplate`, `EntityMap.LookupTemplate` and `EntityMap.Path`. A value the host does not know
  substitutes as the empty string in templates; a map or item lookup whose path or template names an
  unknown one is skipped.
- `ScraperSpec.ItemLookupTemplate`, `ItemFieldMap` and `ItemArtworkMap` (http-json only): after the main
  lookup, the host fetches the per-item document with the same `{id}` plus `{sequence}`/`{index}` and
  maps it with the item maps, whose values override the main document's. A failed or skipped item fetch
  leaves the main mapping unchanged.
- `EntityMap.Container`: the map hydrates the existing container Entity of `Kind` that holds the
  looked-up Media (directly or up its container chain) instead of creating and linking one. `Path` may
  be empty (the document root) and `Relation` must be empty. Only `container`-archetype kinds qualify,
  and the map must declare `ArtworkPath` or `FieldMap`; secondary lookups, parent links and ordinals are
  rejected.
- Validation for all of the above. Additive: existing bundles validate and behave unchanged.

## Impact

- Code: `scraper.go`, `bundle_validate.go`, their tests, regenerated reference docs.
- Consumers: `beamfall-plugin-filmtv` declares the TMDB per-episode lookup and the series and season
  container maps; core implements the placeholders, the item lookup and container hydration.
- Release: an additive minor release (proposed v0.5.0), then repin filmtv and core.
