package sdk

// ImportCategories returns the fixed Migration Wizard data-category vocabulary:
// the behavioral-state categories an Importer may declare it can
// carry. Importers never touch metadata, scraper output, file organization, or
// server settings — Beamfall re-scrapes content from its own scrapers.
// Categories are named by shape; the source system a plugin imports from is
// plugin data, never core vocabulary.
//
// Note: the per-Media played-state category is shape-named "item-state" is used here instead — the per-Media played/new flag,
// domain-agnostic, because the core vocabulary firewall (TestVocab) forbids
// serial-content domain words.
//
//	"watch-state"    resume positions + completion → engagement events
//	"ratings"        numeric/star ratings → per-Profile ratings
//	"collections"    playlists/collections → ordered Collections
//	"subscriptions"  feed subscriptions → Subscriptions
//	"item-state"     per-Media new/played flag (serial-content importers)
func ImportCategories() []string { return copySlice(importCategories) }

var importCategories = []string{"watch-state", "ratings", "collections", "subscriptions", "item-state"}

// ImporterSpec is one declarative Migration Wizard source:
// the Importer Extension Point, parallel to ScraperSpec. A plugin declares one
// per source system it can import behavioral state from (the Film & TV plugin
// declares Plex/Emby/Jellyfin/NFO; the Podcast plugin declares OPML/Overcast/
// AntennaPod). The wizard reads these from the activated bundles; Core never
// hardcodes a source name. The spec is data only — no code, no secrets.
type ImporterSpec struct {
	// ID is a lowercase slug, unique within the bundle ("plex", "jellyfin").
	ID string
	// DisplayName is the human source name shown in the wizard menu
	// (a locale map). Required.
	DisplayName LocaleString
	// Categories are the data categories this importer can carry, from the
	// fixed ImportCategories vocabulary. At least one is required — an
	// importer that imports nothing is meaningless.
	Categories []string
	// Input declares how the user supplies the source: "credentials" (a live
	// API: Emby/Jellyfin) or "export-file" (an uploaded export: Plex XML,
	// OPML, AntennaPod db). One of InputForms; required.
	Input string
	// CredentialRef names the config/secret-store entry holding the live
	// source's credentials, parallel to ScraperSpec.AuthRef. The wizard
	// collects the secret and the operator stores it under this ref before the
	// import can run; the run references the ref, never the secret (invariant
	// #5: secrets live in the platform secure store / 0600 files, never in the
	// job payload). Required for an "input: credentials" importer; must be empty
	// for an "export-file" importer (a file path is the source, not a secret).
	CredentialRef string
	// Conflict declares the importer's conflict-resolution strategy against
	// Beamfall's own curated state, from ConflictStrategies. Re-runs are
	// strictly additive; "additive" is the only strategy
	// today, declared explicitly so a future overwrite mode is a vocabulary
	// addition, not a silent default change.
	Conflict string
}

var inputForms = []string{"credentials", "export-file"}

// InputForms returns the fixed Importer input vocabulary: how a
// user supplies the source the importer reads from.
func InputForms() []string { return copySlice(inputForms) }

var conflictStrategies = []string{"additive"}

// ConflictStrategies returns the fixed Importer conflict-resolution vocabulary.
// Only the additive strategy ships: source data fills gaps or
// advances strictly-ahead positions, never overwrites or downgrades Beamfall's
// own engagement, ratings, or collection membership.
func ConflictStrategies() []string { return copySlice(conflictStrategies) }
