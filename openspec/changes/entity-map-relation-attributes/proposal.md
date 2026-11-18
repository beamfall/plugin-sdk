# Relation attributes, element filters and sort offsets for entity maps

Ticket: CORE-2503 (`queue:core:main`), with the data half of CORE-2432.
Status: proposed.

## Why

A media detail page cannot say who plays whom, who directed or who wrote a film. The credit relation
a plugin declares (`credited-in` in plugin-filmtv) already names `character`, `job`, `department` and
`billing` attributes, but `EntityMap` has no way to write a relation attribute from a document
element, so every credit row is written without them. A plugin also cannot hydrate a subset of an
array (only the key jobs of a crew list), and a second map that writes the same relation cannot place
its rows after the first map's, so a host that orders credits by position interleaves crew with the
lead cast. Finally, `MaxLookups` bounds secondary fetches but says nothing about the elements past the
cap, and hosts drop them, so a long cast list is cut to its first few names.

## What Changes

- `EntityMap.RelationAttributeMap`: relation attribute name → path resolved against the `Path`
  element itself (never a `LookupTemplate` document: attributes describe the link, not the referenced
  Entity). Keys must be attributes the map's `Relation` declares. Unresolved or blank values are
  omitted. When one identification links an Entity to the Media under one relation more than once,
  the host writes one row: its attributes merge, distinct values of one attribute joining in
  declaration and element order with ", ", and its sort position is the smallest.
- `EntityMap.ElementFilter`: element-relative path → accepted values. Only elements whose every
  filter path resolves to one of its values hydrate; others hydrate nothing and consume no lookup.
- `EntityMap.OrdinalOffset`: a non-negative number added to every sort position the map writes, so
  maps sharing a relation can lay their rows out one after another.
- `EntityMap.OrdinalField` may also name a `RelationAttributeMap` key (a billing order the element
  carries).
- `MaxLookups` semantics: elements past the cap hydrate from the element itself, as if the map
  declared no `LookupTemplate`, rather than being dropped. An Entity hydrated with no resolved
  `FieldMap` value keeps the typed fields an earlier identification wrote.
- Validation for all of the above; container maps reject the new fields. Additive: existing bundles
  validate unchanged.

## Impact

- Code: `scraper.go`, `bundle_validate.go`, their tests, regenerated reference docs.
- Consumers: plugin-filmtv maps cast `character` and billing `order`, and a second `credits.crew` map
  filtered to key jobs with `job` and `department`. Core resolves the new fields, writes relation
  attributes, merges duplicate credits, hydrates past the lookup cap and returns credits in sort order.
- Release: an additive, source-compatible patch release (proposed v0.4.4). As for v0.4.1, every
  first-party plugin is re-released with its ceiling raised (filmtv also raises its floor), then core
  repins.
