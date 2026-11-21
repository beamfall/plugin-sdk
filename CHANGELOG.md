# Changelog

## Unreleased

- Plugins can supply named identification hints in parse results or map embedded tags to hint keys.
  Scraper search templates can reference those hints in URL queries and require them before searching.
  The SDK rejects malformed hint placeholders and hints in lookup templates or GraphQL search
  bodies. Literal braces are allowed in URL templates.
  The SDK validates the declarations and the conformance harness can check the rendered requests.
