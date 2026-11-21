# Changelog

## Unreleased

- Plugins can supply named identification hints in parse results or map embedded tags to hint keys.
  Scraper search templates can reference those hints and require them before searching. The SDK
  validates the declarations and the conformance harness can check the rendered requests.
