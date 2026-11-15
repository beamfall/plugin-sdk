# Add sortTitle to the built-in Media FieldMap keys

Ticket: CORE-1768 (`queue:core:main`).
Status: proposed.

## Why

Core Media carries an optional, domain-agnostic sort title (CORE-1715) that the library title sort
and A-Z letter rail key on, falling back to the title. CORE-1768 lets identification write it from a
scraper's lookup result, so a plugin that knows its domain's sorting rules (for example dropping a
leading article) can supply it. The SDK's closed set of bare FieldMap keys only allows `title`,
`summary` and `year`, so a bundle mapping `sortTitle` fails validation at load time.

## What Changes

- `BuiltinMediaFields` gains `sortTitle`. A scraper may map it in `FieldMap` / `SearchFieldMap` like
  the other built-ins; core writes it at identification rank and keeps an operator-locked value.
- No language rules in the SDK: the plugin decides what its sort title is.

## Impact

Additive: bundles that never map `sortTitle` behave exactly as before. Core reads the key once it
pins this SDK release (CORE-1768 core branch `claude/w3-core-1768`).
