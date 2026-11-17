## 1. Build

- [x] 1.1 Document the `{sequence}` and `{index}` placeholders
- [x] 1.2 Add `ScraperSpec.ItemLookupTemplate`, `ItemFieldMap` and `ItemArtworkMap` with validation
- [x] 1.3 Add `EntityMap.Container` with validation
- [x] 1.4 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [x] 2.1 Unit tests for valid and invalid item lookups and container maps
- [x] 2.2 Every first-party plugin and core compile against the change
- [x] 2.3 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 Tag v0.4.1, re-release the first-party plugins at that ceiling (filmtv also at that floor), then repin core
