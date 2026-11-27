# Escape free-text queries for Lucene in SearchTemplate

Ticket CORE-3252 (`queue:core:main`). SDK scope only.

## Why

MusicBrainz's `/recording?query=({query}){hint.recording_clause}{hint.artist_clause}{hint.album_clause}`
template wraps `{query}` in Lucene grouping parentheses alongside hint clauses. Core substitutes
`{query}` URL-escaped without Lucene escaping, so a title containing unbalanced parentheses, `:`,
quotes, or other Lucene syntax characters makes the Lucene query invalid.

## What changes

- Define `{query.lucene}` on `ScraperSpec.SearchTemplate`: the host escapes the free-text query
  for Apache Lucene query parser syntax (`\ + - ! ( ) : ^ [ ] " { } ~ * ? | & /`) before
  applying the same transport escaping as `{query}`.
- Reject `{query.lucene}` outside `SearchTemplate` in `ValidateBundle`.
- Support `{query.lucene}` in `sdktest.CompileScraperTrace` and cover Lucene specials in `sdktest`.

## Delivery

Additive and source-compatible. Host substitution (`beamfall`) and MusicBrainz template adoption
(`beamfall-plugin-music`) follow once this SDK change is tagged and pinned.
