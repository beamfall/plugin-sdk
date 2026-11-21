# Use named parse hints in scraper searches

Ticket CORE-2333 (`queue:core:main`). SDK scope only.

## Why

A title can match several records. Plugins can use file-local metadata to make a scraper search
more specific when the SDK lets them name and reference that metadata.

## What changes

- Add bounded plugin-chosen hints to `ParsedMedia` and a bundle declaration that maps embedded tags
  to those keys.
- Let scraper search templates reference hints and mark referenced hints as required.
- Validate parse hints and declarations in the SDK and cover rendering in the offline conformance
  harness.

## Delivery

Existing parsers and bundles can leave all three fields unset. Invalid keys, oversized maps or
values, and malformed placeholders fail SDK validation. A missing required hint skips the scraper
for that search. Unit and conformance vectors cover the bounds, escaping, and skip behavior.

The host implementation, plugin adoption, and release are follow-up work. A plugin can revert to
title-only search by removing its hint references and declarations.
