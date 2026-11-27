# Carry Library FamilyScope in ParseContext

Ticket CORE-2433 (`queue:core:main`). SDK scope only.

## Why

Film & TV libraries have a plugin-declared `FamilyScope` (`"films"`, `"tvshows"`, `"mixed"`), but
`ParseContext` only carries `DirectoryMediaFileCount`, forcing `plugin-filmtv` to guess TV vs film
from folder names (`nearestLibraryFolder`).

## What changes

- Add `FamilyScope string` to `sdk.ParseContext` (`""` when absent or unknown, preserving legacy
  `Parse` fallback).
- Update `sdktest.RunContextConformance` so non-empty `FamilyScope` vectors with
  `DirectoryMediaFileCountUnknown` are not required to equal legacy `Parse`.

## Delivery

Additive and source-compatible. Host forwarding (`beamfall`) and `beamfall-plugin-filmtv` adoption
follow once this SDK change is tagged and pinned.
