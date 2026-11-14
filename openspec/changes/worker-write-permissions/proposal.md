# Integration Worker permissions for metadata, collection, artwork and watch-state writes

Ticket: CORE-1695 (`queue:core:main`).
Status: proposed.

## Why

The Integration Plugin permission vocabulary lets a Worker read the catalog, request ingest and
request file changes, but not edit what the server already knows. Collection managers, artwork
curators and watch-history sync tools therefore cannot run as Integration Workers, and operators
fall back to scripts that hold a full user token. Core owns enforcement; the SDK owns the closed
vocabulary and its host-owned presentation and risk policy, so the new grants start here.

## What Changes

- Five permission kinds join the closed vocabulary returned by `PermissionKinds`:
  `write-metadata`, `write-collections`, `write-artwork`, `write-watch-state` and
  `read-playback-state`.
- Each carries a host-owned `PermissionPolicy` with an English label and description, and every
  one is `HighRisk`: an operator must approve it after install and Core audits each use. A plugin
  cannot lower the risk or replace the warning text (existing `Bundle` validation).
- The descriptions state the scope each grant is limited to: media and libraries the grant's
  binding reveals, and for watch state and playback state one approved profile.

## Non-goals

- Enforcement. Core implements the Host API methods, scope binding, consent and audit
  (beamfall/core, `openspec/changes/core-1695-worker-write-scopes/`).
- A release. Bumping `Version`, tagging and repinning first-party plugins and Core are done by the
  SDK release owner.

## Failure modes

- A Core still pinned to an older SDK keeps the old vocabulary; a bundle declaring a new kind is
  rejected there as an unknown permission, which fails closed.
- A Core that pins this SDK before it implements the methods fails its vocabulary sync check at
  startup and in tests, so the two must land together.

## Acceptance evidence

- `TestPermissionPolicyCoversClosedVocabulary` lists the new kinds as high risk.
- `script/gate.sh` passes on the lane head.

## Rollback

Revert the change. Bundles declaring the new kinds fail validation as unknown permissions again.
