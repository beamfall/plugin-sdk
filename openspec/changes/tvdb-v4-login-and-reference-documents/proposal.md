# Credential help, reference documents and map-path alternatives for scrapers

Tickets: CORE-1127 and CORE-1128 (`queue:core:main`).
Status: proposed.

## Why

TheTVDB v4 `POST /login` takes `{apikey, pin}` (official swagger v4.8.0, `docs/swagger.yml` in
`thetvdb/v4-api`, read 2026-11-15; the token is valid for one month). CORE-1129 (owner decision
2026-11-11, completed) already made the subscriber PIN the scraper's AuthRef, with the project key in
`LoginSpec.SecretFields`, so TheTVDB stays off until a PIN is saved. That decision supersedes the
optional-PIN shape CORE-1127 first proposed, so this change adds no optional login field. What
CORE-1127 still needs is presentation: clients label the AuthRef as an API key and cannot say it is a
PIN or link to where to get one.

The same swagger and README also ask clients to look artwork types up in `/artwork/types` (cached for
a week or longer) instead of hard-coding type ids, to use `year` on `/search`, and to read translations
from `/series/{id}/extended?meta=translations`, falling back to the original language. Scrapers can
already express most of this as data, except a cached reference document, a multi-condition
predicate and a fallback between two paths.

## What Changes

- `ScraperSpec.CredentialHelp *CredentialHelp`: what the owner enters for the AuthRef ("Subscriber
  PIN") and an optional https link to where to get it ("Subscribe at thetvdb.com").
- `ScraperSpec.ReferenceDocuments map[string]ReferenceDocument`: static documents (path plus cache
  hours, default one week) that map paths read through `{ref.<name>:<path>}` placeholders.
- Documented map-path syntax: multi-condition predicates (`[slug=background,recordType=series]`) and
  `||` alternatives where the first path that resolves wins.
- Documented `{year}` on `SearchTemplate`: substituted when known, and a query parameter whose whole
  value is `{year}` is dropped when it is not.
- `ScraperSpec.LookupByIDRecordIDPath`: when set, a `LookupByIDTemplate` document (TheTVDB's
  `/search/remoteid/{id}`, which answers with a base record that has no overview or translations)
  is read only for the scraper's own record id, and the result is the ordinary `LookupTemplate`
  lookup of that id, so an IMDb or TMDB id yields the same full record as a search match.
- Validation for the new declarations; the local-file (exif) kind rejects all three outright.

## Impact

Additive and source-compatible: a scraper that sets none of the new fields validates and runs as before. The
engine and client behaviour (presenting credential help, fetching and caching reference documents, path
alternatives, `{year}`) is core's; core implements it behind its own types until it pins the release
that carries these fields. Releasing this needs a new SDK tag (v0.3.6), then filmtv and core repins.
