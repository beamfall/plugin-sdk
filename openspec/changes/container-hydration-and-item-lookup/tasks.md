## 1. Build

- [ ] 1.1 Document the `{sequence}` and `{index}` placeholders
- [ ] 1.2 Add `ScraperSpec.ItemLookupTemplate`, `ItemFieldMap` and `ItemArtworkMap` with validation
- [ ] 1.3 Add `EntityMap.Container` with validation
- [ ] 1.4 Regenerate docs with `script/gen-docs.sh`

## 2. Prove

- [ ] 2.1 Unit tests for valid and invalid item lookups and container maps
- [ ] 2.2 Every first-party plugin and core compile against the change
- [ ] 2.3 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 Tag v0.5.0, then repin filmtv and core
