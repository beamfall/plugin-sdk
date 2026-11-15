# Optional login secrets, reference documents and map-path alternatives for scrapers

Tickets: CORE-1127 and CORE-1128 (`queue:core:main`).
Status: proposed.

## Why

TheTVDB v4 `POST /login` takes `{apikey, pin}`. The official swagger (v4.8.0, `docs/swagger.yml` in
`thetvdb/v4-api`, read 2026-11-15) says to send `pin` only with a user-supported key and to remove it
from the call otherwise. `LoginSpec.SecretFields` entries are required, so a scraper cannot declare a
PIN that is sent only when the owner saved one.

The same swagger and README also ask clients to look artwork types up in `/artwork/types` (cached for
a week or longer) instead of hard-coding type ids, to use `year` on `/search`, and to read translations
from `/series/{id}/extended?meta=translations`, falling back to the original language. Scrapers can
already express most of this as data, except a cached reference document, a multi-condition
predicate and a fallback between two paths.

## What Changes

- `LoginSpec.OptionalSecretFields map[string]OptionalSecret`: a login body field sent only when its
  secret is configured and left out of the body otherwise. `OptionalSecret` carries the AuthRef, a
  label and an optional https help link, so clients can show it beside the scraper's key.
- `ScraperSpec.ReferenceDocuments map[string]ReferenceDocument`: static documents (path plus cache
  hours, default one week) that map paths read through `{ref.<name>:<path>}` placeholders.
- Documented map-path syntax: multi-condition predicates (`[slug=background,recordType=series]`) and
  `||` alternatives where the first path that resolves wins.
- Documented `{year}` on `SearchTemplate`: substituted when known, and a query parameter whose whole
  value is `{year}` is dropped when it is not.
- Validation for both new declarations.

## Impact

Additive and source-compatible: a scraper that sets neither field validates and runs as before. The
engine behaviour (sending optional fields, fetching and caching reference documents, path
alternatives, `{year}`) is core's; core implements it behind its own types until it pins the release
that carries these fields. Releasing this needs a new SDK tag (v0.3.6), then filmtv and core repins.
