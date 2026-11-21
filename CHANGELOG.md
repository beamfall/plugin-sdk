# Changelog

## Unreleased

- Plugins can supply named identification hints in parse results or map embedded tags to hint keys.
  Scraper search templates can reference those hints in URL queries and require them before searching.
  The SDK rejects hint placeholders in GraphQL search bodies until it defines GraphQL value binding.
  The SDK validates the declarations and the conformance harness can check the rendered requests.
