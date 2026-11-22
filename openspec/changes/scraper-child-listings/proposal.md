# Declare scraper child listings

Ticket CORE-3127 (`queue:core:main`). SDK scope only. See ADR-0068 amendment H.

## Why

Plugins need an SDK declaration for the lists below a selected search candidate so a manager can
pick a container and an item using the same contract across plugins.

## Delivery

Add an optional `ScraperSpec.Children` declaration and validate it at bundle load. Invalid
listings fail with the plugin, scraper and one-based level. Tests exercise two- and three-level
bundles, invalid declarations, segment safety, and unchanged JSON when children are omitted.

Core traversal, plugin adoption, and publication are follow-up work. A plugin can remove the
declaration to restore its previous search behaviour.
