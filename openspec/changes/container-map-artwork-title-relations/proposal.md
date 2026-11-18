# Container maps: artwork slots, display title and relations

Tickets: CORE-2514, CORE-2513 (`queue:core:main`).
Status: proposed.

## Why

A Container entity map can hydrate only one image (`ArtworkPath`, the poster) and typed fields onto the
container Entity the host built from the folder layout. A series page therefore never gets a backdrop,
and the genres and cast the same lookup document carries are linked only to the episode Media, never to
the series Entity the show page renders (CORE-2514). The container also keeps the folder spelling as
its display name after a match ("A Mothers Son", "90 Day Fiance"), because a Container map may not name
the Entity (CORE-2513).

## What Changes

- `EntityMap.ArtworkMap` (Container maps only): artwork slot → container-node-relative path, composed
  like the scraper's Media `ArtworkMap` (the slot's `ArtworkURLTemplates` entry, else
  `ArtworkBaseURL`). Slots must be core artwork slots; `poster` must not be declared both here and by
  `ArtworkPath`.
- `EntityMap.TitlePath` (Container maps only): a container-node-relative path whose non-blank value
  becomes the container Entity's display title. The Entity's identity is unchanged; the host-built
  name stays the fallback when the path does not resolve.
- `EntityMap.ContainerRelations` (Container maps only): relation declared on the container Kind whose
  `ToKind` is an Entity kind → the `Relation` of a non-container entity map of that kind in the same
  scraper. The host links the container Entity to every Entity that sibling map hydrated in the same
  identification, under the container relation, keeping the sibling's sort order.
- A Container map may hydrate through any of `ArtworkPath`, `ArtworkMap`, `FieldMap` or `TitlePath`.
- Validation for all of the above. Additive: existing bundles validate and behave unchanged.

## Impact

- Code: `scraper.go`, `bundle_validate.go`, tests, regenerated reference docs.
- Consumers: `beamfall-plugin-filmtv` declares the series backdrop, title and genre/cast links; core
  implements the three fields in its scrape and identify paths.
- Release: an additive, source-compatible patch release. `CompatibleWithSDK` compares the exact SDK
  version against each bundle's ceiling, so the first-party plugins are re-released at that ceiling
  (filmtv also at that floor), then core repins.
