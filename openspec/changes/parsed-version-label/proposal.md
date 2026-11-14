# Carry a version label on a filename parse result

Ticket: CORE-1676 (`queue:core:main`).
Status: proposed.

## Why

One title often ships as several files: different resolutions, or different cuts. Core cannot group
them under one Media as Versions because the filename parser has no way to say "this file is the
`1080p` version of this title". Core must stay domain-agnostic, so the label has to come from the
plugin that understands the naming convention.

## What Changes

- `ParsedMedia.VersionLabel`: a shape-named, optional label (empty is the default version and the
  legacy shape).
- `ValidVersionLabel` / `ValidateVersionLabel` and `MaxVersionLabelLen`: the label is empty, or
  trimmed printable text of at most 64 bytes.
- `sdktest.RunConformance` and `RunContextConformance` check the label contract on every vector.

## Impact

Additive and source-compatible: a parser that never sets the field passes unchanged. Core reads the
label to group the files of one title in one directory as Versions (CORE-1676).
