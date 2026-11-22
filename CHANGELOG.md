# Changelog

## Unreleased

- Search candidates and child listings support bare JSON arrays with `$` and terminal
  `[?key=value,...]` filters, including on the root array. Both predicate forms can
  read nested condition fields. This is additive and the SDK version stays at 0.4.7.
  Older cores can ignore the new path shapes only because no released plugin uses them yet.
- Scrapers can declare up to three child listing levels below a search candidate
  (ADR-0068 amendment H). This is additive, and older cores ignore the field.
- Plugins can supply named identification hints in parse results or map embedded tags to hint keys.
  Scraper search templates can reference those hints in URL queries and require them before searching.
  The SDK rejects malformed hint placeholders and hints in lookup templates or GraphQL search
  bodies. Literal braces are allowed in URL templates.
  The SDK validates the declarations and the conformance harness can check the rendered requests.
