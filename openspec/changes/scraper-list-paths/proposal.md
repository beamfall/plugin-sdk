# Extend scraper list paths

Tickets CORE-1775 and CORE-3154 (`queue:core:main`). SDK scope only.

## Why

Some scraper endpoints return the listing as a bare array. Others return rows
from multiple orderings in a single array, so the scraper needs to keep every
row from the selected ordering.

## Delivery

Allow an explicit root array and a terminal filter-all predicate in
`CandidatesPath` and child listing `ItemsPath`. Use the shared grammar for
bundle validation and the offline reference trace. Both predicate forms can
read nested condition fields, with literal keys taking precedence. Existing
first-match predicates and omitted path values retain their behavior.

Core execution, plugin adoption, and publication are follow-up work.
