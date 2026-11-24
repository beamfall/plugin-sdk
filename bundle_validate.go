package sdk

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"github.com/beamfall/plugin-sdk/internal/jsonpath"
)

var (
	bundleSlugPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	semverPattern            = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)
	accentPattern            = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	localeTagPattern         = regexp.MustCompile(`^[a-z]{2,3}(-[a-z0-9]{2,8})*$`)
	configPlaceholderPattern = regexp.MustCompile(`\{config\.([^{}]*)\}`)
	childLevelPattern        = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	childPlaceholderPattern  = regexp.MustCompile(`\{([^{}]*)\}`)
	hintPlaceholderPattern   = regexp.MustCompile(`\{hint\.([^{}]*)\}`)
	// A malformed hint can have just one brace. With only a closing brace,
	// look for a hint token at the start of a URL component or query term.
	hintPlaceholderMarker = regexp.MustCompile(`(?i:\{hint\.|(?:^|[?&=/+#])hint\.[[:alnum:]_.-]*\})`)
	// referencePlaceholderPattern matches a {ref.<name>:<path>} map-path
	// placeholder reading a declared ReferenceDocument.
	referencePlaceholderPattern = regexp.MustCompile(`\{ref\.([^{}:]*):([^{}]*)\}`)
	// sidecarExtPattern is the InteractiveSidecars contract: a lowercase
	// extension with no dot (bundle.go doc).
	sidecarExtPattern = regexp.MustCompile(`^[a-z0-9]+$`)
)

// ValidateBundle checks a Bundle against the fixed SDK vocabularies and the
// declarative authoring invariants Core enforces before activating a plugin.
func ValidateBundle(bundle Bundle) error {
	if !bundleSlugPattern.MatchString(string(bundle.ID)) {
		return fmt.Errorf("plugin ID %q must be a lowercase slug", bundle.ID)
	}
	if !semverPattern.MatchString(bundle.Version) {
		return fmt.Errorf("plugin %s: version %q must be semver", bundle.ID, bundle.Version)
	}
	if err := validateSDKCompatibility(bundle); err != nil {
		return err
	}
	if strings.TrimSpace(bundle.DisplayName.Resolve("")) == "" {
		return fmt.Errorf("plugin %s: display name is required", bundle.ID)
	}
	if err := validateLocaleString(bundle.DisplayName); err != nil {
		return fmt.Errorf("plugin %s: display name: %w", bundle.ID, err)
	}
	if bundle.Accent != "" && !accentPattern.MatchString(bundle.Accent) {
		return fmt.Errorf("plugin %s: accent %q must be a #RRGGBB hex color", bundle.ID, bundle.Accent)
	}
	if strings.TrimSpace(bundle.MediaLabel.Resolve("")) == "" {
		return fmt.Errorf("plugin %s: media label is required", bundle.ID)
	}
	if err := validateLocaleString(bundle.MediaLabel); err != nil {
		return fmt.Errorf("plugin %s: media label: %w", bundle.ID, err)
	}
	if bundle.StoreSensitivity != "" && !slices.Contains(StoreSensitivities(), bundle.StoreSensitivity) {
		return fmt.Errorf("plugin %s: store sensitivity %q is not in {%s}", bundle.ID, bundle.StoreSensitivity, strings.Join(StoreSensitivities(), ", "))
	}
	if bundle.LibraryDefaults.Icon != "" && !slices.Contains(LibraryIcons(), bundle.LibraryDefaults.Icon) {
		return fmt.Errorf("plugin %s: library icon %q is not in {%s}", bundle.ID, bundle.LibraryDefaults.Icon, strings.Join(LibraryIcons(), ", "))
	}
	if bundle.IdentificationPolicy != "" && !slices.Contains(IdentificationPolicies(), bundle.IdentificationPolicy) {
		return fmt.Errorf("plugin %s: identification policy %q is not in {auto, queue}", bundle.ID, bundle.IdentificationPolicy)
	}
	if err := validateEmbeddedTagHints(bundle.EmbeddedTagHints); err != nil {
		return fmt.Errorf("plugin %s: %w", bundle.ID, err)
	}
	if bundle.ControlPolicy != "" && !slices.Contains(ControlPolicies(), bundle.ControlPolicy) {
		return fmt.Errorf("plugin %s: control policy %q is not in {%s}", bundle.ID, bundle.ControlPolicy, strings.Join(ControlPolicies(), ", "))
	}
	features := map[string]bool{}
	for _, feature := range bundle.FeatureEligibility {
		if !slices.Contains(FeatureEligibilityCapabilities(), feature) {
			return fmt.Errorf("plugin %s: feature eligibility %q is not in {%s}", bundle.ID, feature, strings.Join(FeatureEligibilityCapabilities(), ", "))
		}
		if features[feature] {
			return fmt.Errorf("plugin %s: duplicate feature eligibility %q", bundle.ID, feature)
		}
		features[feature] = true
	}
	if err := validatePartyTerms(bundle.PartyTerms, features[FeatureParty]); err != nil {
		return fmt.Errorf("plugin %s: party terms: %w", bundle.ID, err)
	}
	mediaClasses := map[string]bool{}
	for _, class := range bundle.MediaClasses {
		if class != "video" && class != "audio" && class != "image" {
			return fmt.Errorf("plugin %s: unknown media class %q", bundle.ID, class)
		}
		if mediaClasses[class] {
			return fmt.Errorf("plugin %s: duplicate media class %q", bundle.ID, class)
		}
		mediaClasses[class] = true
	}

	mediaFields := map[string]string{}
	for _, field := range bundle.MediaFields {
		if !bundleSlugPattern.MatchString(field.Name) {
			return fmt.Errorf("plugin %s: media field %q must be a lowercase slug", bundle.ID, field.Name)
		}
		if _, dup := mediaFields[field.Name]; dup {
			return fmt.Errorf("plugin %s: duplicate media field %q", bundle.ID, field.Name)
		}
		mediaFields[field.Name] = field.Type
		if !slices.Contains(FieldTypes(), field.Type) {
			return fmt.Errorf("plugin %s: media field %q uses unknown type %q", bundle.ID, field.Name, field.Type)
		}
		if err := validateLocaleString(field.Label); err != nil {
			return fmt.Errorf("plugin %s: media field %q label: %w", bundle.ID, field.Name, err)
		}
		if field.Searchable && field.Type != "text" {
			return fmt.Errorf("plugin %s: media field %q is searchable but type %q, only text fields can be searchable", bundle.ID, field.Name, field.Type)
		}
	}
	if err := validateScrobbleTarget(bundle, mediaFields); err != nil {
		return err
	}
	lockableFields := map[string]bool{}
	for _, key := range bundle.LockableMediaFields {
		if _, err := classifyMediaFieldKey(key, mediaFields); err != nil {
			return fmt.Errorf("plugin %s: lockable media field: %w", bundle.ID, err)
		}
		if lockableFields[key] {
			return fmt.Errorf("plugin %s: duplicate lockable media field %q", bundle.ID, key)
		}
		lockableFields[key] = true
	}
	if len(bundle.CertificationScale) > 0 {
		if mediaFields["certification"] != "text" {
			return fmt.Errorf("plugin %s: certification scale requires the text media field %q", bundle.ID, "certification")
		}
		seen := map[string]bool{}
		for _, value := range bundle.CertificationScale {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("plugin %s: certification scale values must be non-blank", bundle.ID)
			}
			if seen[value] {
				return fmt.Errorf("plugin %s: duplicate certification scale value %q", bundle.ID, value)
			}
			seen[value] = true
		}
	}
	for class, label := range bundle.MediaClassLabels {
		if !bundleSlugPattern.MatchString(class) {
			return fmt.Errorf("plugin %s: media class label key %q must be a lowercase slug", bundle.ID, class)
		}
		if strings.TrimSpace(label["en"]) == "" {
			return fmt.Errorf("plugin %s: media class label %q requires an English fallback", bundle.ID, class)
		}
		if err := validateLocaleString(label); err != nil {
			return fmt.Errorf("plugin %s: media class label %q: %w", bundle.ID, class, err)
		}
	}

	if err := validateMediaSubtitle(bundle.MediaSubtitle, mediaFields); err != nil {
		return fmt.Errorf("plugin %s: %w", bundle.ID, err)
	}

	configFields := map[string]bool{}
	for _, field := range bundle.ConfigFields {
		if err := validateConfigField(bundle.ID, "config field", field); err != nil {
			return err
		}
		if configFields[field.Name] {
			return fmt.Errorf("plugin %s: duplicate config field %q", bundle.ID, field.Name)
		}
		configFields[field.Name] = true
	}

	permissionKinds := map[string]bool{}
	for _, permission := range bundle.Permissions {
		policy, ok := PermissionPolicyFor(permission.Kind)
		if !ok {
			return fmt.Errorf("plugin %s: permission %q is not in {%s}", bundle.ID, permission.Kind, strings.Join(PermissionKinds(), ", "))
		}
		if permissionKinds[permission.Kind] {
			return fmt.Errorf("plugin %s: duplicate permission %q", bundle.ID, permission.Kind)
		}
		permissionKinds[permission.Kind] = true
		if err := validateLocaleString(permission.Label); err != nil {
			return fmt.Errorf("plugin %s: permission %q label: %w", bundle.ID, permission.Kind, err)
		}
		if !maps.Equal(permission.Label, policy.Label) {
			return fmt.Errorf("plugin %s: permission %q label must equal the host-owned policy label", bundle.ID, permission.Kind)
		}
		if err := validateLocaleString(permission.Description); err != nil {
			return fmt.Errorf("plugin %s: permission %q description: %w", bundle.ID, permission.Kind, err)
		}
		if !maps.Equal(permission.Description, policy.Description) {
			return fmt.Errorf("plugin %s: permission %q description must equal the host-owned policy warning", bundle.ID, permission.Kind)
		}
		if err := validateLocaleString(permission.Context); err != nil {
			return fmt.Errorf("plugin %s: permission %q context: %w", bundle.ID, permission.Kind, err)
		}
		if policy.HighRisk && !permission.HighRisk {
			return fmt.Errorf("plugin %s: permission %q high-risk flag cannot be lower than the host-owned policy", bundle.ID, permission.Kind)
		}
	}

	actionIDs := map[string]bool{}
	for _, action := range bundle.Actions {
		if !bundleSlugPattern.MatchString(action.ID) {
			return fmt.Errorf("plugin %s: action ID %q must be a lowercase slug", bundle.ID, action.ID)
		}
		if actionIDs[action.ID] {
			return fmt.Errorf("plugin %s: duplicate action %q", bundle.ID, action.ID)
		}
		actionIDs[action.ID] = true
		if strings.TrimSpace(action.Label.Resolve("")) == "" {
			return fmt.Errorf("plugin %s: action %q label is required", bundle.ID, action.ID)
		}
		if err := validateLocaleString(action.Label); err != nil {
			return fmt.Errorf("plugin %s: action %q label: %w", bundle.ID, action.ID, err)
		}
		if action.Risk != "" && !slices.Contains(ActionRiskLevels(), action.Risk) {
			return fmt.Errorf("plugin %s: action %q risk %q is not in {%s}", bundle.ID, action.ID, action.Risk, strings.Join(ActionRiskLevels(), ", "))
		}
		for _, permission := range action.RequiredPermissions {
			if !slices.Contains(PermissionKinds(), permission) {
				return fmt.Errorf("plugin %s: action %q permission %q is not in {%s}", bundle.ID, action.ID, permission, strings.Join(PermissionKinds(), ", "))
			}
		}
		for _, field := range action.InputFields {
			if err := validateConfigField(bundle.ID, "action "+action.ID+" input field", field); err != nil {
				return err
			}
		}
		if err := validateLocaleString(action.Confirmation); err != nil {
			return fmt.Errorf("plugin %s: action %q confirmation: %w", bundle.ID, action.ID, err)
		}
	}

	kinds := map[string]bool{}
	containerKinds := map[string]bool{}
	for _, kind := range bundle.EntityKinds {
		if !bundleSlugPattern.MatchString(kind.Name) {
			return fmt.Errorf("plugin %s: entity kind %q must be a lowercase slug", bundle.ID, kind.Name)
		}
		if kinds[kind.Name] {
			return fmt.Errorf("plugin %s: duplicate entity kind %q", bundle.ID, kind.Name)
		}
		kinds[kind.Name] = true
		containerKinds[kind.Name] = kind.Archetype == "container"
		if err := validateLocaleString(kind.DisplayName); err != nil {
			return fmt.Errorf("plugin %s: kind %q display name: %w", bundle.ID, kind.Name, err)
		}
		if err := validateLocaleString(kind.PluralDisplayName); err != nil {
			return fmt.Errorf("plugin %s: kind %q plural display name: %w", bundle.ID, kind.Name, err)
		}
		if len(kind.PluralDisplayName) > 0 && len(kind.DisplayName) == 0 {
			return fmt.Errorf("plugin %s: kind %q declares a plural display name without a display name", bundle.ID, kind.Name)
		}
		if !slices.Contains(Archetypes(), kind.Archetype) {
			return fmt.Errorf("plugin %s: kind %q uses unknown archetype %q", bundle.ID, kind.Name, kind.Archetype)
		}
		for _, field := range kind.Fields {
			if !slices.Contains(FieldTypes(), field.Type) {
				return fmt.Errorf("plugin %s: kind %q field %q uses unknown type %q", bundle.ID, kind.Name, field.Name, field.Type)
			}
			if err := validateLocaleString(field.Label); err != nil {
				return fmt.Errorf("plugin %s: kind %q field %q label: %w", bundle.ID, kind.Name, field.Name, err)
			}
			if (field.Filterable || field.Sortable || field.Searchable) && !bundleSlugPattern.MatchString(field.Name) {
				return fmt.Errorf("plugin %s: kind %q indexed field %q must be a lowercase slug", bundle.ID, kind.Name, field.Name)
			}
			if field.Searchable && field.Type != "text" {
				return fmt.Errorf("plugin %s: kind %q field %q is searchable but type %q, only text fields can be searchable", bundle.ID, kind.Name, field.Name, field.Type)
			}
		}
		for _, slot := range kind.ArtworkSlots {
			if !slices.Contains(ArtworkSlots(), slot) {
				return fmt.Errorf("plugin %s: kind %q uses unknown artwork slot %q", bundle.ID, kind.Name, slot)
			}
		}
		flagFields := map[string]bool{}
		numberFields := map[string]bool{}
		for _, field := range kind.Fields {
			switch field.Type {
			case "flag":
				flagFields[field.Name] = true
			case "number":
				numberFields[field.Name] = true
			}
		}
		for label, name := range map[string]string{
			"sort-last field":            kind.SortLastField,
			"exclude-auto-advance field": kind.ExcludeAutoAdvanceField,
		} {
			if name == "" {
				continue
			}
			if !kind.OrderedChildren {
				return fmt.Errorf("plugin %s: kind %q declares a %s but is not an ordered-children container", bundle.ID, kind.Name, label)
			}
			if !flagFields[name] {
				return fmt.Errorf("plugin %s: kind %q %s %q must name a declared flag field", bundle.ID, kind.Name, label, name)
			}
		}
		if kind.TitleJoinRule != "" {
			if kind.Archetype != "container" {
				return fmt.Errorf("plugin %s: kind %q declares a title-join rule but is not a container", bundle.ID, kind.Name)
			}
			if !slices.Contains(TitleJoinRules(), kind.TitleJoinRule) {
				return fmt.Errorf("plugin %s: kind %q uses unknown title-join rule %q", bundle.ID, kind.Name, kind.TitleJoinRule)
			}
		}
		if kind.RatingScaleArchetype != "" {
			if kind.Archetype != "container" {
				return fmt.Errorf("plugin %s: kind %q declares a rating-scale archetype but is not a container", bundle.ID, kind.Name)
			}
			if !slices.Contains(RatingScaleArchetypes(), kind.RatingScaleArchetype) {
				return fmt.Errorf("plugin %s: kind %q uses unknown rating-scale archetype %q", bundle.ID, kind.Name, kind.RatingScaleArchetype)
			}
		}
		if kind.OrderingSourceField != "" {
			if !kind.OrderedChildren {
				return fmt.Errorf("plugin %s: kind %q declares an ordering-source field but is not an ordered-children container", bundle.ID, kind.Name)
			}
			if !numberFields[kind.OrderingSourceField] {
				return fmt.Errorf("plugin %s: kind %q ordering-source field %q must name a declared number field", bundle.ID, kind.Name, kind.OrderingSourceField)
			}
		}
		if kind.OrphanPolicy != "" && !slices.Contains(OrphanPolicies(), kind.OrphanPolicy) {
			return fmt.Errorf("plugin %s: kind %q orphan policy %q is not in {%s}", bundle.ID, kind.Name, kind.OrphanPolicy, strings.Join(OrphanPolicies(), ", "))
		}
	}

	mediaRelations := map[string]bool{}
	// relationAttributes holds each Media-targeted relation's plain declared
	// attributes, the ones an EntityMap's RelationAttributeMap may write.
	relationAttributes := map[string]map[string]bool{}
	selfRelations := map[string]bool{}
	hierarchical := map[string]bool{}
	for _, kind := range bundle.EntityKinds {
		if kind.Hierarchical {
			hierarchical[kind.Name] = true
		}
	}
	for _, kind := range bundle.EntityKinds {
		for _, relation := range kind.Relations {
			if relation.Cardinality != "" && !slices.Contains(RelationCardinalities(), relation.Cardinality) {
				return fmt.Errorf("plugin %s: kind %q relation %q uses unknown cardinality %q", bundle.ID, kind.Name, relation.Name, relation.Cardinality)
			}
			attributes := map[string]bool{}
			for _, attribute := range relation.Attributes {
				if strings.TrimSpace(attribute) == "" {
					return fmt.Errorf("plugin %s: kind %q relation %q declares a blank attribute", bundle.ID, kind.Name, relation.Name)
				}
				if attributes[attribute] {
					return fmt.Errorf("plugin %s: kind %q relation %q declares duplicate attribute %q", bundle.ID, kind.Name, relation.Name, attribute)
				}
				attributes[attribute] = true
			}
			for _, attribute := range relation.TypedAttributes {
				if strings.TrimSpace(attribute.Name) == "" {
					return fmt.Errorf("plugin %s: kind %q relation %q declares a blank typed attribute", bundle.ID, kind.Name, relation.Name)
				}
				if attributes[attribute.Name] {
					return fmt.Errorf("plugin %s: kind %q relation %q declares duplicate attribute %q", bundle.ID, kind.Name, relation.Name, attribute.Name)
				}
				attributes[attribute.Name] = true
				if !slices.Contains(FieldTypes(), attribute.Type) {
					return fmt.Errorf("plugin %s: kind %q relation %q typed attribute %q uses unknown type %q", bundle.ID, kind.Name, relation.Name, attribute.Name, attribute.Type)
				}
			}
			if relation.ToKind == MediaKindName {
				mediaRelations[kind.Name+"\x00"+relation.Name] = true
				plain := map[string]bool{}
				for _, attribute := range relation.Attributes {
					plain[attribute] = true
				}
				relationAttributes[kind.Name+"\x00"+relation.Name] = plain
				continue
			}
			if !kinds[relation.ToKind] {
				return fmt.Errorf("plugin %s: kind %q relation %q targets undeclared kind %q", bundle.ID, kind.Name, relation.Name, relation.ToKind)
			}
			if relation.ToKind == kind.Name && hierarchical[kind.Name] {
				selfRelations[kind.Name+"\x00"+relation.Name] = true
			}
		}
	}

	layoutIDs := map[string]bool{}
	layoutArchetypes := map[string]bool{}
	browseCount := 0
	for _, layout := range bundle.Layouts {
		if !bundleSlugPattern.MatchString(layout.ID) {
			return fmt.Errorf("plugin %s: layout ID %q must be a lowercase slug", bundle.ID, layout.ID)
		}
		if layoutIDs[layout.ID] {
			return fmt.Errorf("plugin %s: duplicate layout ID %q", bundle.ID, layout.ID)
		}
		layoutIDs[layout.ID] = true
		if !slices.Contains(LayoutRoles(), layout.Role) {
			return fmt.Errorf("plugin %s: layout %q uses unknown role %q", bundle.ID, layout.ID, layout.Role)
		}
		if layout.Role == "browse" {
			browseCount++
		}
		if layout.EntityKind != "" {
			if layout.Role != "detail" {
				return fmt.Errorf("plugin %s: layout %q: only detail layouts take an entity kind", bundle.ID, layout.ID)
			}
			if !kinds[layout.EntityKind] {
				return fmt.Errorf("plugin %s: layout %q binds undeclared entity kind %q", bundle.ID, layout.ID, layout.EntityKind)
			}
		}
		if layout.Archetype != "" {
			if layout.Role != "detail" {
				return fmt.Errorf("plugin %s: layout %q: only detail layouts take an archetype", bundle.ID, layout.ID)
			}
			if !slices.Contains(LayoutArchetypes(), layout.Archetype) {
				return fmt.Errorf("plugin %s: layout %q uses unknown archetype %q", bundle.ID, layout.ID, layout.Archetype)
			}
			if layoutArchetypes[layout.Archetype] {
				return fmt.Errorf("plugin %s: duplicate layout archetype %q", bundle.ID, layout.Archetype)
			}
			layoutArchetypes[layout.Archetype] = true
		}
		sectionIDs := map[string]bool{}
		for _, section := range layout.Sections {
			if !bundleSlugPattern.MatchString(section.ID) {
				return fmt.Errorf("plugin %s: layout %q: section ID %q must be a lowercase slug", bundle.ID, layout.ID, section.ID)
			}
			if sectionIDs[section.ID] {
				return fmt.Errorf("plugin %s: layout %q: duplicate section ID %q", bundle.ID, layout.ID, section.ID)
			}
			sectionIDs[section.ID] = true
			if !slices.Contains(SectionKinds(), section.Kind) {
				return fmt.Errorf("plugin %s: section %q uses unknown kind %q", bundle.ID, section.ID, section.Kind)
			}
			if err := validateLocaleString(section.Title); err != nil {
				return fmt.Errorf("plugin %s: section %q title: %w", bundle.ID, section.ID, err)
			}
			if section.ArtworkSlot != "" && !slices.Contains(ArtworkSlots(), section.ArtworkSlot) {
				return fmt.Errorf("plugin %s: section %q uses unknown artwork slot %q", bundle.ID, section.ID, section.ArtworkSlot)
			}
			if err := validateSource(section.Source); err != nil {
				return fmt.Errorf("plugin %s: section %q: %w", bundle.ID, section.ID, err)
			}
			if section.Source.EntityKind != "" && !kinds[section.Source.EntityKind] {
				return fmt.Errorf("plugin %s: section %q lists undeclared entity kind %q", bundle.ID, section.ID, section.Source.EntityKind)
			}
			for field, value := range section.Source.FieldFilters {
				fieldType, declared := mediaFields[field]
				if !declared {
					return fmt.Errorf("plugin %s: section %q filters undeclared media field %q", bundle.ID, section.ID, field)
				}
				if err := validateFieldFilterValue(fieldType, value); err != nil {
					return fmt.Errorf("plugin %s: section %q filters media field %q of type %q: %w", bundle.ID, section.ID, field, fieldType, err)
				}
			}
			if filter := section.Source.EntityFilter; filter != nil && !kinds[filter.Kind] {
				return fmt.Errorf("plugin %s: section %q entity filter names undeclared entity kind %q", bundle.ID, section.ID, filter.Kind)
			}
			if len(section.FilterOptions) > 0 && section.Kind != "filter-bar" {
				return fmt.Errorf("plugin %s: section %q declares filter options on kind %q, only filter-bar sections take them", bundle.ID, section.ID, section.Kind)
			}
			if section.Kind == "playback-affordances" && len(section.PlaybackAffordances) == 0 {
				return fmt.Errorf("plugin %s: section %q playback-affordances sections require playback affordances", bundle.ID, section.ID)
			}
			if len(section.PlaybackAffordances) > 0 && section.Kind != "playback-affordances" {
				return fmt.Errorf("plugin %s: section %q declares playback affordances on kind %q, only playback-affordances sections take them", bundle.ID, section.ID, section.Kind)
			}
			for _, affordance := range section.PlaybackAffordances {
				if !slices.Contains(PlaybackAffordances(), affordance) {
					return fmt.Errorf("plugin %s: section %q playback affordance %q is not in {%s}", bundle.ID, section.ID, affordance, strings.Join(PlaybackAffordances(), ", "))
				}
			}
			for _, option := range section.FilterOptions {
				fieldType, declared := mediaFields[option.Field]
				if !declared {
					return fmt.Errorf("plugin %s: section %q filter option names undeclared media field %q", bundle.ID, section.ID, option.Field)
				}
				if fieldType != "text" {
					return fmt.Errorf("plugin %s: section %q filter option names media field %q of type %q, only text fields can be filtered", bundle.ID, section.ID, option.Field, fieldType)
				}
				if strings.TrimSpace(option.Value) == "" {
					return fmt.Errorf("plugin %s: section %q filter option on field %q requires a value", bundle.ID, section.ID, option.Field)
				}
				if err := validateLocaleString(option.Label); err != nil {
					return fmt.Errorf("plugin %s: section %q filter option label: %w", bundle.ID, section.ID, err)
				}
			}
		}
	}
	if browseCount != 1 {
		return fmt.Errorf("plugin %s: bundles declare exactly one browse layout, got %d", bundle.ID, browseCount)
	}

	scraperIDs := map[string]bool{}
	for _, scraper := range bundle.Scrapers {
		if !bundleSlugPattern.MatchString(scraper.ID) {
			return fmt.Errorf("plugin %s: scraper ID %q must be a lowercase slug", bundle.ID, scraper.ID)
		}
		if scraperIDs[scraper.ID] {
			return fmt.Errorf("plugin %s: duplicate scraper ID %q", bundle.ID, scraper.ID)
		}
		scraperIDs[scraper.ID] = true
		if err := validateChildListings(bundle.ID, scraper); err != nil {
			return err
		}
		if !slices.Contains(ScraperKinds(), scraper.Kind) {
			return fmt.Errorf("plugin %s: scraper %q uses unknown kind %q", bundle.ID, scraper.ID, scraper.Kind)
		}
		if err := validateLocaleString(scraper.DisplayName); err != nil {
			return fmt.Errorf("plugin %s: scraper %q display name: %w", bundle.ID, scraper.ID, err)
		}
		if err := validateAttribution(scraper.Attribution); err != nil {
			return fmt.Errorf("plugin %s: scraper %q attribution: %w", bundle.ID, scraper.ID, err)
		}
		if LocalFileScraperKind(scraper.Kind) {
			if err := validateLocalFileScraper(bundle.ID, scraper); err != nil {
				return err
			}
			// Local-file scrapers skip the transport/template rules below but
			// DO declare ImpliedMediaFields (the photos plugin's
			// {"media.class": ...} classification) — validate those before
			// moving on rather than letting the skip exempt them.
			if err := validateImpliedMediaFields(bundle.ID, scraper, mediaFields); err != nil {
				return err
			}
			continue
		}
		if !strings.HasPrefix(scraper.Endpoint, "https://") {
			return fmt.Errorf("plugin %s: scraper %q endpoint must be https", bundle.ID, scraper.ID)
		}
		if scraper.LookupTemplate == "" && scraper.SearchTemplate == "" {
			return fmt.Errorf("plugin %s: scraper %q declares neither a lookup nor a search template", bundle.ID, scraper.ID)
		}
		if err := ValidateLookupByIDTemplates(scraper); err != nil {
			return fmt.Errorf("plugin %s: %w", bundle.ID, err)
		}
		if err := validateLookupByIDRecordIDPaths(scraper); err != nil {
			return fmt.Errorf("plugin %s: %w", bundle.ID, err)
		}
		if err := validateScraperTemplatePlaceholders(bundle.ID, scraper, configFields); err != nil {
			return err
		}
		if err := validateReferenceDocuments(scraper, configFields); err != nil {
			return fmt.Errorf("plugin %s: scraper %q %w", bundle.ID, scraper.ID, err)
		}
		if err := validateCredentialHelp(scraper); err != nil {
			return fmt.Errorf("plugin %s: scraper %q %w", bundle.ID, scraper.ID, err)
		}
		if !ValidateAuthStyle(scraper.AuthStyle) {
			return fmt.Errorf("plugin %s: scraper %q uses unknown auth style %q (forms: %s)", bundle.ID, scraper.ID, scraper.AuthStyle, strings.Join(AuthStyleForms(), ", "))
		}
		if scraper.AuthStyle != "" && scraper.AuthRef == "" {
			return fmt.Errorf("plugin %s: scraper %q declares auth style %q without an AuthRef", bundle.ID, scraper.ID, scraper.AuthStyle)
		}
		if (scraper.AuthStyle == "login-bearer") != (scraper.Login != nil) {
			return fmt.Errorf("plugin %s: scraper %q: the login-bearer style and the Login declaration require each other", bundle.ID, scraper.ID)
		}
		if login := scraper.Login; login != nil {
			if !strings.HasPrefix(login.Path, "/") || login.SecretField == "" || login.TokenPath == "" {
				return fmt.Errorf("plugin %s: scraper %q login declaration needs a /-rooted path, a secret field, and a token path", bundle.ID, scraper.ID)
			}
			for field, ref := range login.SecretFields {
				if field == "" || field == login.SecretField || !validAuthRef(ref) {
					return fmt.Errorf("plugin %s: scraper %q login secret field %q needs a distinct name and a plain AuthRef (got %q)", bundle.ID, scraper.ID, field, ref)
				}
			}
		}
		if strings.HasPrefix(scraper.AuthStyle, "signature:") != (scraper.Signature != nil) {
			return fmt.Errorf("plugin %s: scraper %q: the signature:<name> style and the Signature declaration require each other", bundle.ID, scraper.ID)
		}
		if sig := scraper.Signature; sig != nil {
			if !slices.Contains(SignatureArchetypes(), sig.Archetype) {
				return fmt.Errorf("plugin %s: scraper %q uses unknown signature archetype %q (forms: %s)", bundle.ID, scraper.ID, sig.Archetype, strings.Join(SignatureArchetypes(), ", "))
			}
			if "signature:"+sig.Archetype != scraper.AuthStyle {
				return fmt.Errorf("plugin %s: scraper %q signature archetype %q does not match auth style %q", bundle.ID, scraper.ID, sig.Archetype, scraper.AuthStyle)
			}
			if sig.HeaderName == "" {
				return fmt.Errorf("plugin %s: scraper %q signature declaration needs a header name", bundle.ID, scraper.ID)
			}
		}
		if scraper.ArchiveBackfill && scraper.Kind != "rss" {
			return fmt.Errorf("plugin %s: scraper %q: archive backfill is only valid on rss scrapers", bundle.ID, scraper.ID)
		}
		if scraper.ArchiveBackfill && scraper.ArchiveBackfillGUIDPath == "" {
			return fmt.Errorf("plugin %s: scraper %q declares archive backfill without a guid source", bundle.ID, scraper.ID)
		}
		if scraper.SearchTemplate != "" {
			if scraper.Kind == "rss" {
				return fmt.Errorf("plugin %s: scraper %q: rss scrapers cannot declare a search template", bundle.ID, scraper.ID)
			}
			if scraper.MaxPages < 0 {
				return fmt.Errorf("plugin %s: scraper %q max pages must not be negative", bundle.ID, scraper.ID)
			}
			if scraper.CandidatesPath == "" {
				return fmt.Errorf("plugin %s: scraper %q search template requires a candidates path", bundle.ID, scraper.ID)
			}
			if scraper.LookupTemplate != "" && scraper.CandidateIDPath == "" {
				return fmt.Errorf("plugin %s: scraper %q declares search and lookup but no candidate ID path to chain them", bundle.ID, scraper.ID)
			}
		} else if scraper.CandidatesPath != "" || scraper.CandidateIDPath != "" {
			return fmt.Errorf("plugin %s: scraper %q declares candidate paths without a search template", bundle.ID, scraper.ID)
		}
		if scraper.SearchTemplate == "" && (scraper.SearchFieldMap != nil || scraper.SearchArtworkMap != nil) {
			return fmt.Errorf("plugin %s: scraper %q declares search maps without a search template", bundle.ID, scraper.ID)
		}
		if err := validateScraperResponsePaths(scraper); err != nil {
			return fmt.Errorf("plugin %s: scraper %q: %w", bundle.ID, scraper.ID, err)
		}
		for _, fieldMap := range []map[string]string{scraper.FieldMap, scraper.SearchFieldMap} {
			for key := range fieldMap {
				if _, err := classifyMediaFieldKey(key, mediaFields); err != nil {
					return fmt.Errorf("plugin %s: scraper %q field map: %w", bundle.ID, scraper.ID, err)
				}
			}
		}
		for field, fieldType := range scraper.FieldTypes {
			if !slices.Contains(FieldTypes(), fieldType) {
				return fmt.Errorf("plugin %s: scraper %q declares unknown field type %q for field %q", bundle.ID, scraper.ID, fieldType, field)
			}
		}
		for field, kind := range scraper.Transforms {
			if !slices.Contains(TransformKinds(), kind) {
				return fmt.Errorf("plugin %s: scraper %q uses unknown transform %q on field %q", bundle.ID, scraper.ID, kind, field)
			}
			_, mapped := scraper.FieldMap[field]
			_, itemMapped := scraper.ItemFieldMap[field]
			if _, searchMapped := scraper.SearchFieldMap[field]; !mapped && !searchMapped && !itemMapped {
				return fmt.Errorf("plugin %s: scraper %q transforms unmapped field %q", bundle.ID, scraper.ID, field)
			}
		}
		if err := validateImpliedMediaFields(bundle.ID, scraper, mediaFields); err != nil {
			return err
		}
		if len(scraper.EntityMaps) > 0 {
			if scraper.Kind == "rss" {
				return fmt.Errorf("plugin %s: scraper %q: rss scrapers cannot declare entity maps", bundle.ID, scraper.ID)
			}
			if scraper.LookupTemplate == "" {
				return fmt.Errorf("plugin %s: scraper %q declares entity maps without a lookup template to resolve them against", bundle.ID, scraper.ID)
			}
		}
		if err := validateItemLookup(scraper, mediaFields); err != nil {
			return fmt.Errorf("plugin %s: scraper %q: %w", bundle.ID, scraper.ID, err)
		}
		for _, entityMap := range scraper.EntityMaps {
			if !kinds[entityMap.Kind] {
				return fmt.Errorf("plugin %s: scraper %q entity map targets undeclared kind %q", bundle.ID, scraper.ID, entityMap.Kind)
			}
			if err := validateEntityMapResponsePaths(entityMap); err != nil {
				return fmt.Errorf("plugin %s: scraper %q entity map for kind %q: %w", bundle.ID, scraper.ID, entityMap.Kind, err)
			}
			if err := validateContainerMapExtras(bundle, scraper, entityMap); err != nil {
				return fmt.Errorf("plugin %s: scraper %q: %w", bundle.ID, scraper.ID, err)
			}
			if entityMap.Container {
				if err := validateContainerEntityMap(entityMap, containerKinds); err != nil {
					return fmt.Errorf("plugin %s: scraper %q: %w", bundle.ID, scraper.ID, err)
				}
			} else {
				if !mediaRelations[entityMap.Kind+"\x00"+entityMap.Relation] {
					return fmt.Errorf("plugin %s: scraper %q entity map relation %q is not a Media-targeted relation declared on kind %q", bundle.ID, scraper.ID, entityMap.Relation, entityMap.Kind)
				}
				if strings.TrimSpace(entityMap.Path) == "" {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q requires a path", bundle.ID, scraper.ID, entityMap.Kind)
				}
			}
			if entityMap.ArtworkPath != "" && strings.TrimSpace(entityMap.ArtworkPath) == "" {
				return fmt.Errorf("plugin %s: scraper %q entity map for kind %q has a blank artwork path", bundle.ID, scraper.ID, entityMap.Kind)
			}
			if (entityMap.LookupTemplate == "") != (entityMap.LookupIDPath == "") {
				return fmt.Errorf("plugin %s: scraper %q entity map for kind %q reference lookup requires both lookup template and ID path", bundle.ID, scraper.ID, entityMap.Kind)
			}
			if entityMap.LookupTemplate != "" {
				if scraper.Kind != "http-json" {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q reference lookup requires http-json", bundle.ID, scraper.ID, entityMap.Kind)
				}
				if entityMap.MaxLookups <= 0 {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q reference lookup requires positive max lookups", bundle.ID, scraper.ID, entityMap.Kind)
				}
				if strings.TrimSpace(entityMap.LookupTemplate) == "" || !strings.Contains(entityMap.LookupTemplate, "{id}") {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q reference lookup template must contain {id}", bundle.ID, scraper.ID, entityMap.Kind)
				}
				if strings.TrimSpace(entityMap.LookupIDPath) == "" {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q reference lookup ID path is blank", bundle.ID, scraper.ID, entityMap.Kind)
				}
				if hintPlaceholderMarker.MatchString(entityMap.LookupTemplate) {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q reference lookup template uses a hint placeholder outside SearchTemplate", bundle.ID, scraper.ID, entityMap.Kind)
				}
			} else if entityMap.MaxLookups < 0 {
				return fmt.Errorf("plugin %s: scraper %q entity map for kind %q max lookups must not be negative", bundle.ID, scraper.ID, entityMap.Kind)
			}
			if entityMap.ArtworkURLTemplate != "" && !validArtworkURLTemplate(entityMap.ArtworkURLTemplate) {
				return fmt.Errorf("plugin %s: scraper %q entity map for kind %q artwork URL template must be https and contain {value}", bundle.ID, scraper.ID, entityMap.Kind)
			}
			kindFields := bundle.EntityFieldTypes(entityMap.Kind)
			for field, path := range entityMap.FieldMap {
				if _, declared := kindFields[field]; !declared {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q maps undeclared field %q", bundle.ID, scraper.ID, entityMap.Kind, field)
				}
				if strings.TrimSpace(path) == "" {
					return fmt.Errorf("plugin %s: scraper %q entity map field %q for kind %q has a blank path", bundle.ID, scraper.ID, field, entityMap.Kind)
				}
			}
			if entityMap.ParentNamePath != "" {
				if strings.TrimSpace(entityMap.ParentNamePath) == "" {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q has a blank parent name path", bundle.ID, scraper.ID, entityMap.Kind)
				}
				if entityMap.ParentRelation == "" {
					return fmt.Errorf("plugin %s: scraper %q entity map for kind %q declares a parent name path without a parent relation", bundle.ID, scraper.ID, entityMap.Kind)
				}
				if !selfRelations[entityMap.Kind+"\x00"+entityMap.ParentRelation] {
					return fmt.Errorf("plugin %s: scraper %q entity map parent relation %q is not a same-kind relation declared on hierarchical kind %q", bundle.ID, scraper.ID, entityMap.ParentRelation, entityMap.Kind)
				}
			} else if entityMap.ParentRelation != "" {
				return fmt.Errorf("plugin %s: scraper %q entity map for kind %q declares a parent relation without a parent name path", bundle.ID, scraper.ID, entityMap.Kind)
			}
			if err := validateEntityMapRelationValues(entityMap, relationAttributes[entityMap.Kind+"\x00"+entityMap.Relation]); err != nil {
				return fmt.Errorf("plugin %s: scraper %q: %w", bundle.ID, scraper.ID, err)
			}
		}
		if scraper.ArtworkBaseURL != "" && !strings.HasPrefix(scraper.ArtworkBaseURL, "https://") {
			return fmt.Errorf("plugin %s: scraper %q artwork base URL must be https", bundle.ID, scraper.ID)
		}
		for slot, tmpl := range scraper.ArtworkURLTemplates {
			if !slices.Contains(ArtworkSlots(), slot) {
				return fmt.Errorf("plugin %s: scraper %q maps unknown artwork slot %q", bundle.ID, scraper.ID, slot)
			}
			if !validArtworkURLTemplate(tmpl) {
				return fmt.Errorf("plugin %s: scraper %q artwork URL template for slot %q must be https and contain {value}", bundle.ID, scraper.ID, slot)
			}
		}
		if scraper.MinRequestIntervalMS < 0 {
			return fmt.Errorf("plugin %s: scraper %q min request interval must not be negative", bundle.ID, scraper.ID)
		}
		for slot := range scraper.ArtworkMap {
			if !slices.Contains(ArtworkSlots(), slot) {
				return fmt.Errorf("plugin %s: scraper %q maps unknown artwork slot %q", bundle.ID, scraper.ID, slot)
			}
		}
		for slot := range scraper.SearchArtworkMap {
			if !slices.Contains(ArtworkSlots(), slot) {
				return fmt.Errorf("plugin %s: scraper %q maps unknown artwork slot %q", bundle.ID, scraper.ID, slot)
			}
		}
		candidateSlots := map[string]bool{}
		for _, slot := range scraper.CandidateArtworkSlots {
			if !slices.Contains(ArtworkSlots(), slot) {
				return fmt.Errorf("plugin %s: scraper %q declares unknown candidate artwork slot %q", bundle.ID, scraper.ID, slot)
			}
			if candidateSlots[slot] {
				return fmt.Errorf("plugin %s: scraper %q duplicate candidate artwork slot %q", bundle.ID, scraper.ID, slot)
			}
			candidateSlots[slot] = true
			_, mapped := scraper.ArtworkMap[slot]
			_, searchMapped := scraper.SearchArtworkMap[slot]
			if !mapped && !searchMapped {
				return fmt.Errorf("plugin %s: scraper %q candidate artwork slot %q is not mapped by ArtworkMap or SearchArtworkMap", bundle.ID, scraper.ID, slot)
			}
		}
	}

	importerIDs := map[string]bool{}
	for _, importer := range bundle.Importers {
		if !bundleSlugPattern.MatchString(importer.ID) {
			return fmt.Errorf("plugin %s: importer ID %q must be a lowercase slug", bundle.ID, importer.ID)
		}
		if importerIDs[importer.ID] {
			return fmt.Errorf("plugin %s: duplicate importer ID %q", bundle.ID, importer.ID)
		}
		importerIDs[importer.ID] = true
		if strings.TrimSpace(importer.DisplayName.Resolve("")) == "" {
			return fmt.Errorf("plugin %s: importer %q display name is required", bundle.ID, importer.ID)
		}
		if err := validateLocaleString(importer.DisplayName); err != nil {
			return fmt.Errorf("plugin %s: importer %q display name: %w", bundle.ID, importer.ID, err)
		}
		if len(importer.Categories) == 0 {
			return fmt.Errorf("plugin %s: importer %q declares no data categories", bundle.ID, importer.ID)
		}
		for _, category := range importer.Categories {
			if !slices.Contains(ImportCategories(), category) {
				return fmt.Errorf("plugin %s: importer %q declares unknown data category %q", bundle.ID, importer.ID, category)
			}
		}
		if !slices.Contains(InputForms(), importer.Input) {
			return fmt.Errorf("plugin %s: importer %q uses unknown input form %q (forms: %s)", bundle.ID, importer.ID, importer.Input, strings.Join(InputForms(), ", "))
		}
		if !slices.Contains(ConflictStrategies(), importer.Conflict) {
			return fmt.Errorf("plugin %s: importer %q uses unknown conflict strategy %q (strategies: %s)", bundle.ID, importer.ID, importer.Conflict, strings.Join(ConflictStrategies(), ", "))
		}
		switch importer.Input {
		case "credentials":
			if strings.TrimSpace(importer.CredentialRef) == "" {
				return fmt.Errorf("plugin %s: importer %q uses input %q but declares no credentialRef", bundle.ID, importer.ID, importer.Input)
			}
		case "export-file":
			if strings.TrimSpace(importer.CredentialRef) != "" {
				return fmt.Errorf("plugin %s: importer %q uses input %q and must not declare a credentialRef", bundle.ID, importer.ID, importer.Input)
			}
		}
	}

	scopeValues := map[string]bool{}
	scopeDefaults := 0
	for _, scope := range bundle.FamilyScopes {
		if !bundleSlugPattern.MatchString(scope.Value) {
			return fmt.Errorf("plugin %s: family scope value %q must be a lowercase slug", bundle.ID, scope.Value)
		}
		if scopeValues[scope.Value] {
			return fmt.Errorf("plugin %s: duplicate family scope value %q", bundle.ID, scope.Value)
		}
		scopeValues[scope.Value] = true
		if strings.TrimSpace(scope.DisplayName.Resolve("")) == "" {
			return fmt.Errorf("plugin %s: family scope %q display name is required", bundle.ID, scope.Value)
		}
		if err := validateLocaleString(scope.DisplayName); err != nil {
			return fmt.Errorf("plugin %s: family scope %q display name: %w", bundle.ID, scope.Value, err)
		}
		if scope.Default {
			scopeDefaults++
		}
	}
	if len(bundle.FamilyScopes) > 0 && scopeDefaults != 1 {
		return fmt.Errorf("plugin %s: a plugin declaring family scopes marks exactly one default, got %d", bundle.ID, scopeDefaults)
	}
	if err := validateInterop(bundle, mediaFields, containerKinds, scopeValues); err != nil {
		return err
	}

	eventKinds := map[string]bool{}
	for _, kind := range bundle.Engagement.EventKinds {
		if strings.TrimSpace(kind) == "" {
			return fmt.Errorf("plugin %s: engagement event kinds must be non-blank", bundle.ID)
		}
		if eventKinds[kind] {
			return fmt.Errorf("plugin %s: duplicate engagement event kind %q", bundle.ID, kind)
		}
		eventKinds[kind] = true
	}

	if err := validateInteractiveSidecars(bundle); err != nil {
		return err
	}
	if err := validateScrobbleMatch(bundle); err != nil {
		return err
	}
	return validateLabelMaps(bundle)
}

func validateSDKCompatibility(bundle Bundle) error {
	if bundle.SDKFloor == "" {
		return fmt.Errorf("plugin %s: SDK floor is required", bundle.ID)
	}
	if !semverPattern.MatchString(bundle.SDKFloor) {
		return fmt.Errorf("plugin %s: SDK floor %q must be semver", bundle.ID, bundle.SDKFloor)
	}
	if bundle.SDKCeiling == "" {
		return fmt.Errorf("plugin %s: SDK ceiling is required", bundle.ID)
	}
	if !semverPattern.MatchString(bundle.SDKCeiling) {
		return fmt.Errorf("plugin %s: SDK ceiling %q must be semver", bundle.ID, bundle.SDKCeiling)
	}
	if releaseTriple(bundle.SDKFloor)[0] != releaseTriple(bundle.SDKCeiling)[0] {
		return fmt.Errorf("plugin %s: SDK floor %q and ceiling %q must declare one supported major", bundle.ID, bundle.SDKFloor, bundle.SDKCeiling)
	}
	if compareSemver(bundle.SDKFloor, bundle.SDKCeiling) > 0 {
		return fmt.Errorf("plugin %s: SDK floor %q is newer than SDK ceiling %q", bundle.ID, bundle.SDKFloor, bundle.SDKCeiling)
	}
	return nil
}

// validateInteractiveSidecars enforces the documented InteractiveSidecars
// contract: lowercase file extensions, no dot, non-empty, no duplicates.
func validateInteractiveSidecars(bundle Bundle) error {
	seen := map[string]bool{}
	for _, ext := range bundle.InteractiveSidecars {
		if !sidecarExtPattern.MatchString(ext) {
			return fmt.Errorf("plugin %s: interactive sidecar extension %q must be a lowercase extension with no dot", bundle.ID, ext)
		}
		if seen[ext] {
			return fmt.Errorf("plugin %s: duplicate interactive sidecar extension %q", bundle.ID, ext)
		}
		seen[ext] = true
	}
	return nil
}

// validateScrobbleMatch enforces the documented ScrobbleMatchSpec contract:
// every ArtistConfirm/AlbumTieBreak entry is a non-blank bare relation slug
// matching a declared Media-targeted relation on one of the bundle's Entity
// Kinds (the same class of check labelMapTarget applies to entity-targeted
// label maps), with no duplicates within a list.
func validateScrobbleMatch(bundle Bundle) error {
	if bundle.ScrobbleMatch == nil {
		return nil
	}
	mediaRelations := map[string]bool{}
	for _, kind := range bundle.EntityKinds {
		for _, r := range kind.Relations {
			if r.ToKind == MediaKindName {
				mediaRelations[r.Name] = true
			}
		}
	}
	for listName, slugs := range map[string][]string{
		"artistConfirm": bundle.ScrobbleMatch.ArtistConfirm,
		"albumTieBreak": bundle.ScrobbleMatch.AlbumTieBreak,
	} {
		seen := map[string]bool{}
		for _, slug := range slugs {
			if !bundleSlugPattern.MatchString(slug) {
				return fmt.Errorf("plugin %s: scrobble match %s relation %q must be a lowercase slug", bundle.ID, listName, slug)
			}
			if seen[slug] {
				return fmt.Errorf("plugin %s: scrobble match %s duplicates relation %q", bundle.ID, listName, slug)
			}
			seen[slug] = true
			if !mediaRelations[slug] {
				return fmt.Errorf("plugin %s: scrobble match %s relation %q is not a declared Media-targeted relation", bundle.ID, listName, slug)
			}
		}
	}
	return nil
}

// validateScrobbleTarget enforces the fail-closed ScrobbleTargetSpec contract:
// exactly one known default, or one declared text Media field with a non-empty
// exact-value mapping whose hints all belong to the bounded SDK vocabulary.
func validateScrobbleTarget(bundle Bundle, mediaFields map[string]string) error {
	spec := bundle.ScrobbleTarget
	if spec == nil {
		return nil
	}

	hasDefault := spec.Default != ""
	hasField := spec.MediaField != ""
	hasMappings := len(spec.Mappings) > 0
	if hasDefault {
		if hasField || hasMappings {
			return fmt.Errorf("plugin %s: scrobble target must declare exactly one of default or media field mappings", bundle.ID)
		}
		if !spec.Default.Known() {
			return fmt.Errorf("plugin %s: scrobble target default hint %q is unknown", bundle.ID, spec.Default)
		}
		return nil
	}

	if !hasField {
		return fmt.Errorf("plugin %s: scrobble target must declare exactly one of default or media field mappings", bundle.ID)
	}
	fieldType, declared := mediaFields[spec.MediaField]
	if !declared {
		return fmt.Errorf("plugin %s: scrobble target media field %q is not declared", bundle.ID, spec.MediaField)
	}
	if fieldType != "text" {
		return fmt.Errorf("plugin %s: scrobble target media field %q has type %q, want text", bundle.ID, spec.MediaField, fieldType)
	}
	if !hasMappings {
		return fmt.Errorf("plugin %s: scrobble target media field %q requires at least one mapping", bundle.ID, spec.MediaField)
	}

	seen := map[string]bool{}
	for _, mapping := range spec.Mappings {
		if strings.TrimSpace(mapping.Value) == "" {
			return fmt.Errorf("plugin %s: scrobble target mapping values must be non-blank", bundle.ID)
		}
		if seen[mapping.Value] {
			return fmt.Errorf("plugin %s: duplicate scrobble target mapping value %q", bundle.ID, mapping.Value)
		}
		seen[mapping.Value] = true
		if !mapping.Hint.Known() {
			return fmt.Errorf("plugin %s: scrobble target mapping value %q uses unknown hint %q", bundle.ID, mapping.Value, mapping.Hint)
		}
	}
	return nil
}

var interopTypePattern = regexp.MustCompile(`^[A-Za-z]{1,32}$`)

// validateInterop enforces the InteropSpec contract: Media values come from one
// declared text Media field, Kinds name this bundle's container kinds, Scopes
// name its declared family scopes, every token matches interopTypePattern, and
// an item type identifies either Media or exactly one kind, never both.
func validateInterop(bundle Bundle, mediaFields map[string]string, containerKinds, scopes map[string]bool) error {
	spec := bundle.Interop
	if spec == nil {
		return nil
	}
	if (spec.MediaField == "") != (len(spec.Media) == 0) {
		return fmt.Errorf("plugin %s: interop media field and media mappings must be declared together", bundle.ID)
	}
	if len(spec.Media) == 0 && len(spec.Kinds) == 0 && len(spec.Scopes) == 0 {
		return fmt.Errorf("plugin %s: interop declares no mappings", bundle.ID)
	}
	if spec.MediaField != "" {
		fieldType, declared := mediaFields[spec.MediaField]
		if !declared {
			return fmt.Errorf("plugin %s: interop media field %q is not declared", bundle.ID, spec.MediaField)
		}
		if fieldType != "text" {
			return fmt.Errorf("plugin %s: interop media field %q has type %q, want text", bundle.ID, spec.MediaField, fieldType)
		}
	}
	mediaTypes := map[string]bool{}
	if err := validateInteropMappings(bundle.ID, "media", spec.Media, func(m InteropMapping) error {
		mediaTypes[m.Type] = true
		return nil
	}); err != nil {
		return err
	}
	kindTypes := map[string]string{}
	if err := validateInteropMappings(bundle.ID, "kind", spec.Kinds, func(m InteropMapping) error {
		isContainer, declared := containerKinds[m.Value]
		if !declared {
			return fmt.Errorf("plugin %s: interop kind %q is not a declared entity kind", bundle.ID, m.Value)
		}
		if !isContainer {
			return fmt.Errorf("plugin %s: interop kind %q is not a container kind", bundle.ID, m.Value)
		}
		if other, dup := kindTypes[m.Type]; dup {
			return fmt.Errorf("plugin %s: interop kinds %q and %q share item type %q", bundle.ID, other, m.Value, m.Type)
		}
		if mediaTypes[m.Type] {
			return fmt.Errorf("plugin %s: interop item type %q names both media and kind %q", bundle.ID, m.Type, m.Value)
		}
		kindTypes[m.Type] = m.Value
		return nil
	}); err != nil {
		return err
	}
	return validateInteropMappings(bundle.ID, "scope", spec.Scopes, func(m InteropMapping) error {
		if !scopes[m.Value] {
			return fmt.Errorf("plugin %s: interop scope %q is not a declared family scope", bundle.ID, m.Value)
		}
		return nil
	})
}

func validateInteropMappings(id ID, label string, mappings []InteropMapping, check func(InteropMapping) error) error {
	seen := map[string]bool{}
	for _, m := range mappings {
		if strings.TrimSpace(m.Value) == "" {
			return fmt.Errorf("plugin %s: interop %s values must be non-blank", id, label)
		}
		if seen[m.Value] {
			return fmt.Errorf("plugin %s: duplicate interop %s value %q", id, label, m.Value)
		}
		seen[m.Value] = true
		if !interopTypePattern.MatchString(m.Type) {
			return fmt.Errorf("plugin %s: interop %s %q token %q must be 1-32 ASCII letters", id, label, m.Value, m.Type)
		}
		if err := check(m); err != nil {
			return err
		}
	}
	return nil
}

// ValidateBundles validates each bundle individually and then enforces the
// cross-plugin invariants a single ValidateBundle cannot see. Entity Kind names
// are a global namespace (store rows, relations, and Layouts key on the bare
// kind name), so two active plugins declaring the same kind name
// would collide on those rows; this rejects that. Bundles must carry distinct
// plugin IDs — a duplicate ID is itself a collision.
func ValidateBundles(bundles []Bundle) error {
	if err := validateEach(bundles); err != nil {
		return err
	}
	seenPlugin := map[ID]bool{}
	kindOwner := map[string]ID{}
	for _, bundle := range bundles {
		if seenPlugin[bundle.ID] {
			return fmt.Errorf("plugin %s: duplicate plugin ID across bundles", bundle.ID)
		}
		seenPlugin[bundle.ID] = true
		for _, kind := range bundle.EntityKinds {
			if owner, taken := kindOwner[kind.Name]; taken {
				return fmt.Errorf("entity kind %q is declared by both plugin %s and plugin %s; kind names are a global namespace", kind.Name, owner, bundle.ID)
			}
			kindOwner[kind.Name] = bundle.ID
		}
	}
	return nil
}

func validateEach(bundles []Bundle) error {
	for _, bundle := range bundles {
		if err := ValidateBundle(bundle); err != nil {
			return err
		}
	}
	return nil
}

func validateSource(source SourceSpec) error {
	if !slices.Contains(SourceKinds(), source.Kind) {
		return fmt.Errorf("unknown source kind %q", source.Kind)
	}
	if source.Kind == "entity-items" && source.EntityKind == "" {
		return fmt.Errorf("entity-items source requires an entity kind")
	}
	if source.Kind != "entity-items" && source.EntityKind != "" {
		return fmt.Errorf("source kind %q does not take an entity kind", source.Kind)
	}
	if source.Seed != "" && source.Kind != "random" {
		return fmt.Errorf("source kind %q does not take a seed (allowed on: random)", source.Kind)
	}
	if allowed, valued := ValuedSourceKinds()[source.Kind]; valued {
		if !slices.Contains(allowed, source.Value) {
			return fmt.Errorf("source kind %q requires a value in {%s}", source.Kind, strings.Join(allowed, ", "))
		}
	} else if allowed, optional := OptionalValueSourceKinds()[source.Kind]; optional {
		if source.Value != "" && !slices.Contains(allowed, source.Value) {
			return fmt.Errorf("source kind %q takes an empty value or one in {%s}", source.Kind, strings.Join(allowed, ", "))
		}
	} else if source.Value != "" {
		return fmt.Errorf("source kind %q does not take a value", source.Kind)
	}
	if len(source.FieldFilters) > 0 && !slices.Contains(FilterableSourceKinds(), source.Kind) {
		return fmt.Errorf("source kind %q does not take field filters (allowed on: %s)", source.Kind, strings.Join(FilterableSourceKinds(), ", "))
	}
	for field, value := range source.FieldFilters {
		if !bundleSlugPattern.MatchString(field) {
			return fmt.Errorf("field filter key %q must be a lowercase slug", field)
		}
		if value == nil {
			return fmt.Errorf("field filter %q requires a value", field)
		}
	}
	if filter := source.EntityFilter; filter != nil {
		if !slices.Contains(FilterableSourceKinds(), source.Kind) {
			return fmt.Errorf("source kind %q does not take an entity filter (allowed on: %s)", source.Kind, strings.Join(FilterableSourceKinds(), ", "))
		}
		if !bundleSlugPattern.MatchString(filter.Kind) {
			return fmt.Errorf("entity filter kind %q must be a lowercase slug", filter.Kind)
		}
		if strings.TrimSpace(filter.Name) == "" {
			return fmt.Errorf("entity filter on kind %q requires a name", filter.Kind)
		}
	}
	return nil
}

func validateAttribution(a *Attribution) error {
	if a == nil {
		return nil
	}
	if len(a.Text) == 0 {
		return errors.New("text is required")
	}
	if err := validateLocaleString(a.Text); err != nil {
		return fmt.Errorf("text: %w", err)
	}
	u, err := url.Parse(a.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("link %q must be an absolute https URL", a.URL)
	}
	return nil
}

func validatePartyTerms(terms *PartyTerms, partyEligible bool) error {
	if terms == nil {
		return nil
	}
	if !partyEligible {
		return fmt.Errorf("declared without feature eligibility %q", FeatureParty)
	}
	for _, term := range []struct {
		name  string
		value LocaleString
	}{
		{"party", terms.Party}, {"participant", terms.Participant}, {"participants", terms.Participants},
		{"request", terms.Request}, {"requests", terms.Requests}, {"stage", terms.Stage},
	} {
		if err := validateLocaleString(term.value); err != nil {
			return fmt.Errorf("%s: %w", term.name, err)
		}
	}
	if len(terms.Participants) > 0 && len(terms.Participant) == 0 {
		return fmt.Errorf("participants is declared without participant")
	}
	if len(terms.Requests) > 0 && len(terms.Request) == 0 {
		return fmt.Errorf("requests is declared without request")
	}
	return nil
}

func validateLocaleString(s LocaleString) error {
	if len(s) == 0 {
		return nil
	}
	if strings.TrimSpace(s["en"]) == "" {
		return fmt.Errorf("locale map must carry an %q entry (the English fallback anchor)", "en")
	}
	for tag, value := range s {
		if !localeTagPattern.MatchString(tag) {
			return fmt.Errorf("locale map tag %q is not a lowercase BCP-47 tag", tag)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("locale map value for %q is blank", tag)
		}
	}
	return nil
}

func validateConfigField(pluginID ID, label string, field ConfigFieldSpec) error {
	if !bundleSlugPattern.MatchString(field.Name) {
		return fmt.Errorf("plugin %s: %s %q must be a lowercase slug", pluginID, label, field.Name)
	}
	if !slices.Contains(FieldTypes(), field.Type) {
		return fmt.Errorf("plugin %s: %s %q uses unknown type %q", pluginID, label, field.Name, field.Type)
	}
	if strings.TrimSpace(field.Label.Resolve("")) == "" {
		return fmt.Errorf("plugin %s: %s %q label is required", pluginID, label, field.Name)
	}
	if err := validateLocaleString(field.Label); err != nil {
		return fmt.Errorf("plugin %s: %s %q label: %w", pluginID, label, field.Name, err)
	}
	seenOptions := map[string]bool{}
	for _, option := range field.Options {
		if strings.TrimSpace(option.Value) == "" {
			return fmt.Errorf("plugin %s: %s %q option values must be non-blank", pluginID, label, field.Name)
		}
		if seenOptions[option.Value] {
			return fmt.Errorf("plugin %s: %s %q duplicate option %q", pluginID, label, field.Name, option.Value)
		}
		seenOptions[option.Value] = true
		if strings.TrimSpace(option.Label.Resolve("")) == "" {
			return fmt.Errorf("plugin %s: %s %q option %q label is required", pluginID, label, field.Name, option.Value)
		}
		if err := validateLocaleString(option.Label); err != nil {
			return fmt.Errorf("plugin %s: %s %q option %q label: %w", pluginID, label, field.Name, option.Value, err)
		}
	}
	return nil
}

// validateCredentialHelp checks a scraper's credential presentation: it
// needs an AuthRef and an "en" label, and links only to an https page.
func validateCredentialHelp(scraper ScraperSpec) error {
	help := scraper.CredentialHelp
	if help == nil {
		return nil
	}
	if scraper.AuthRef == "" {
		return fmt.Errorf("declares credential help without an AuthRef")
	}
	if help.Label["en"] == "" {
		return fmt.Errorf("credential help needs an \"en\" label")
	}
	if err := validateLocaleString(help.Label); err != nil {
		return fmt.Errorf("credential help label: %w", err)
	}
	if help.HelpURL == "" {
		if len(help.HelpLabel) > 0 {
			return fmt.Errorf("credential help declares a help label without a help URL")
		}
		return nil
	}
	u, err := url.Parse(help.HelpURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("credential help URL %q must be an absolute https URL", help.HelpURL)
	}
	if help.HelpLabel["en"] == "" {
		return fmt.Errorf("credential help URL needs an \"en\" help label")
	}
	if err := validateLocaleString(help.HelpLabel); err != nil {
		return fmt.Errorf("credential help label: %w", err)
	}
	return nil
}

// maxReferenceDocumentCacheHours caps ReferenceDocument.CacheHours at a year.
const maxReferenceDocumentCacheHours = 8760

// validateReferenceDocuments checks a scraper's reference documents and that
// every {ref.<name>:<path>} placeholder in its maps names one of them.
func validateReferenceDocuments(scraper ScraperSpec, configFields map[string]bool) error {
	if len(scraper.ReferenceDocuments) > 0 && scraper.Kind != "http-json" {
		return fmt.Errorf("declares reference documents, which only http-json scrapers support")
	}
	for name, doc := range scraper.ReferenceDocuments {
		if !bundleSlugPattern.MatchString(name) {
			return fmt.Errorf("reference document name %q must be a lower-case slug", name)
		}
		if !strings.HasPrefix(doc.Path, "/") {
			return fmt.Errorf("reference document %q path %q must start with \"/\"", name, doc.Path)
		}
		if strings.Contains(configPlaceholderPattern.ReplaceAllString(doc.Path, ""), "{") {
			return fmt.Errorf("reference document %q path %q may use only {config.<name>} placeholders", name, doc.Path)
		}
		for _, match := range configPlaceholderPattern.FindAllStringSubmatch(doc.Path, -1) {
			if !configFields[match[1]] {
				return fmt.Errorf("reference document %q path references undeclared config field %q", name, match[1])
			}
		}
		if doc.CacheHours < 0 || doc.CacheHours > maxReferenceDocumentCacheHours {
			return fmt.Errorf("reference document %q cache hours %d must be between 0 and %d", name, doc.CacheHours, maxReferenceDocumentCacheHours)
		}
	}
	for _, m := range []map[string]string{scraper.FieldMap, scraper.ArtworkMap, scraper.SearchFieldMap, scraper.SearchArtworkMap, scraper.ItemFieldMap, scraper.ItemArtworkMap} {
		for key, path := range m {
			for _, match := range referencePlaceholderPattern.FindAllStringSubmatch(path, -1) {
				if _, ok := scraper.ReferenceDocuments[match[1]]; !ok {
					return fmt.Errorf("map entry %q references undeclared reference document %q", key, match[1])
				}
				if err := validateNonListPath("reference document path", match[2]); err != nil {
					return fmt.Errorf("map entry %q: %w", key, err)
				}
			}
		}
	}
	return nil
}

func validateEmbeddedTagHints(mappings map[string]string) error {
	if len(mappings) > maxParsedHints {
		return fmt.Errorf("EmbeddedTagHints may contain at most %d mappings", maxParsedHints)
	}
	keys := map[string]bool{}
	for tag, key := range mappings {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("EmbeddedTagHints tag name must not be blank")
		}
		if !ValidHintKey(key) {
			return fmt.Errorf("EmbeddedTagHints key %q must match [a-z0-9_]{1,32}", key)
		}
		if keys[key] {
			return fmt.Errorf("EmbeddedTagHints maps more than one tag to hint %q", key)
		}
		keys[key] = true
	}
	return nil
}

func validateChildListings(pluginID ID, scraper ScraperSpec) error {
	if len(scraper.Children) == 0 {
		return nil
	}
	const maxLevels = 3
	if len(scraper.Children) > maxLevels {
		return fmt.Errorf("plugin %s: scraper %q child level %d exceeds the limit of %d levels", pluginID, scraper.ID, maxLevels+1, maxLevels)
	}
	if scraper.Kind != "http-json" || scraper.SearchTemplate == "" {
		return fmt.Errorf("plugin %s: scraper %q child level 1 requires an http-json scraper with a SearchTemplate", pluginID, scraper.ID)
	}
	levels := make(map[string]bool, len(scraper.Children))
	for i, child := range scraper.Children {
		if err := validateChildListing(child, levels); err != nil {
			return fmt.Errorf("plugin %s: scraper %q child level %d: %w", pluginID, scraper.ID, i+1, err)
		}
		levels[child.Level] = true
	}
	return nil
}

func validateChildListing(child ChildListing, levels map[string]bool) error {
	switch {
	case !childLevelPattern.MatchString(child.Level):
		return fmt.Errorf("listing Level %q must be lower-kebab", child.Level)
	case levels[child.Level]:
		return fmt.Errorf("duplicate Level %q", child.Level)
	case !validChildListTarget(child.ListTemplate):
		return errors.New("listing ListTemplate must be an endpoint-relative path with an optional query")
	case child.MaxItems < 0 || child.MaxItems > 500:
		return errors.New("listing MaxItems must be between 0 and 500")
	}
	if err := validateJSONPath("ItemsPath", child.ItemsPath); err != nil {
		return err
	}
	for _, field := range []struct {
		name, path string
		optional   bool
	}{
		{"Key", child.Key, false},
		{"Label", child.Label, false},
		{"Sequence", child.Sequence, true},
		{"Index", child.Index, true},
		{"Year", child.Year, true},
		{"Artwork", child.Artwork, true},
	} {
		if field.optional && field.path == "" {
			continue
		}
		if err := validateNonListPath(field.name, field.path); err != nil {
			return err
		}
		if err := validateJSONPath(field.name, field.path); err != nil {
			return err
		}
	}
	return nil
}

func validateScraperResponsePaths(scraper ScraperSpec) error {
	if scraper.CandidatesPath != "" {
		if err := validateJSONPath("CandidatesPath", scraper.CandidatesPath); err != nil {
			return err
		}
	}
	if scraper.Login != nil {
		if err := validateNonListPath("Login.TokenPath", scraper.Login.TokenPath); err != nil {
			return err
		}
	}
	if err := validateNonListPath("CandidateIDPath", scraper.CandidateIDPath); err != nil {
		return err
	}
	if err := validateNonListPath("ArchiveBackfillGUIDPath", scraper.ArchiveBackfillGUIDPath); err != nil {
		return err
	}
	return validateNonListPathMaps(
		namedPathMap{"FieldMap", scraper.FieldMap},
		namedPathMap{"ArtworkMap", scraper.ArtworkMap},
		namedPathMap{"SearchFieldMap", scraper.SearchFieldMap},
		namedPathMap{"SearchArtworkMap", scraper.SearchArtworkMap},
		namedPathMap{"ItemFieldMap", scraper.ItemFieldMap},
		namedPathMap{"ItemArtworkMap", scraper.ItemArtworkMap},
		namedPathMap{"LookupByIDRecordIDPaths", scraper.LookupByIDRecordIDPaths},
	)
}

func validateEntityMapResponsePaths(entityMap EntityMap) error {
	for _, field := range []struct{ name, path string }{
		{"Path", entityMap.Path},
		{"NamePath", entityMap.NamePath},
		{"ArtworkPath", entityMap.ArtworkPath},
		{"ParentNamePath", entityMap.ParentNamePath},
		{"LookupIDPath", entityMap.LookupIDPath},
		{"TitlePath", entityMap.TitlePath},
	} {
		if err := validateNonListPath(field.name, field.path); err != nil {
			return err
		}
	}
	for path := range entityMap.ElementFilter {
		if err := validateNonListPath("ElementFilter key", path); err != nil {
			return err
		}
	}
	return validateNonListPathMaps(
		namedPathMap{"FieldMap", entityMap.FieldMap},
		namedPathMap{"ArtworkMap", entityMap.ArtworkMap},
		namedPathMap{"RelationAttributeMap", entityMap.RelationAttributeMap},
	)
}

type namedPathMap struct {
	name  string
	paths map[string]string
}

func validateNonListPathMaps(pathMaps ...namedPathMap) error {
	for _, pathMap := range pathMaps {
		for key, path := range pathMap.paths {
			if err := validateNonListPath(pathMap.name, path); err != nil {
				return fmt.Errorf("entry %q: %w", key, err)
			}
		}
	}
	return nil
}

func validateNonListPath(name, path string) error {
	if jsonpath.UsesListSyntax(path) {
		return fmt.Errorf("%s path %q uses $ or a filter-all predicate, which is allowed only in CandidatesPath and ItemsPath", name, path)
	}
	return nil
}

func validateJSONPath(name, path string) error {
	if _, ok := jsonpath.Parse(path); !ok {
		return fmt.Errorf("%s must be a non-empty JSON path with any filter-all predicate in the last segment, got %q", name, path)
	}
	return nil
}

func validChildListTarget(template string) bool {
	if !strings.HasPrefix(template, "/") || strings.Contains(template, "#") {
		return false
	}
	target, err := url.ParseRequestURI(template)
	return err == nil && !strings.HasPrefix(target.Path, "//") &&
		!strings.Contains(target.Path, "://") && !strings.Contains(target.Path, `\`) &&
		!slices.Contains(strings.Split(target.Path, "/"), "..")
}

func validateChildListPlaceholders(template string, level int) error {
	if strings.ContainsAny(childPlaceholderPattern.ReplaceAllString(template, ""), "{}") {
		return errors.New("has a malformed placeholder")
	}
	usesPrevious := false
	for _, match := range childPlaceholderPattern.FindAllStringSubmatch(template, -1) {
		switch match[1] {
		case "parent", "sequence", "index":
			if level == 1 {
				return fmt.Errorf("cannot use %s at the first level", match[0])
			}
			usesPrevious = true
		case "id":
		default:
			if !strings.HasPrefix(match[1], "config.") {
				return fmt.Errorf("uses unsupported placeholder %s", match[0])
			}
		}
	}
	if level > 1 && !usesPrevious {
		return errors.New("must use {parent}, {sequence}, or {index} from the previous level")
	}
	return nil
}

func validateScraperTemplatePlaceholders(pluginID ID, scraper ScraperSpec, configFields map[string]bool) error {
	type scraperTemplate struct {
		name       string
		value      string
		childLevel int
	}
	templates := []scraperTemplate{
		{name: "lookup", value: scraper.LookupTemplate},
		{name: "search", value: scraper.SearchTemplate},
		{name: "item lookup", value: scraper.ItemLookupTemplate},
	}
	for provider, template := range scraper.LookupByIDTemplate {
		templates = append(templates, scraperTemplate{name: fmt.Sprintf("lookup-by-ID[%q]", provider), value: template})
	}
	for i, child := range scraper.Children {
		templates = append(templates, scraperTemplate{
			name: fmt.Sprintf("child level %d list", i+1), value: child.ListTemplate, childLevel: i + 1,
		})
	}
	searchHints := map[string]bool{}
	for _, template := range templates {
		for _, match := range configPlaceholderPattern.FindAllStringSubmatch(template.value, -1) {
			if !configFields[match[1]] {
				return fmt.Errorf("plugin %s: scraper %q %s template references undeclared config field %q", pluginID, scraper.ID, template.name, match[1])
			}
		}
		hasHint := hintPlaceholderMarker.MatchString(template.value)
		if hasHint && template.name != "search" {
			return fmt.Errorf("plugin %s: scraper %q %s template uses a hint placeholder outside SearchTemplate", pluginID, scraper.ID, template.name)
		}
		if template.childLevel > 0 {
			if err := validateChildListPlaceholders(template.value, template.childLevel); err != nil {
				return fmt.Errorf("plugin %s: scraper %q %s template %w", pluginID, scraper.ID, template.name, err)
			}
		}
		if !hasHint {
			continue
		}
		beforeFragment, _, _ := strings.Cut(template.value, "#")
		queryStart := strings.IndexByte(beforeFragment, '?')
		uncheckedStart := 0
		for _, match := range hintPlaceholderPattern.FindAllStringSubmatchIndex(template.value, -1) {
			start, end := match[0], match[1]
			key := template.value[match[2]:match[3]]
			if hintPlaceholderMarker.MatchString(template.value[uncheckedStart:start]) ||
				!ValidHintKey(key) ||
				(start > 0 && template.value[start-1] == '{') ||
				(end < len(template.value) && template.value[end] == '}') {
				break
			}
			if scraper.Kind == "http-graphql" || queryStart < 0 || start <= queryStart || start >= len(beforeFragment) {
				return fmt.Errorf("plugin %s: scraper %q SearchTemplate hint placeholders require a URL query", pluginID, scraper.ID)
			}
			searchHints[key] = true
			// Keep the closing brace as context so the next span does not
			// make adjacent literal text look like the start of a URL component.
			uncheckedStart = end - 1
		}
		if hintPlaceholderMarker.MatchString(template.value[uncheckedStart:]) {
			return fmt.Errorf("plugin %s: scraper %q SearchTemplate has a malformed hint placeholder (want {hint.<key>} with a valid hint key)", pluginID, scraper.ID)
		}
	}
	if len(scraper.RequiredSearchHints) > maxParsedHints {
		return fmt.Errorf("plugin %s: scraper %q RequiredSearchHints may contain at most %d keys", pluginID, scraper.ID, maxParsedHints)
	}
	for i, key := range scraper.RequiredSearchHints {
		if !searchHints[key] {
			return fmt.Errorf("plugin %s: scraper %q required search hint %q is not referenced by SearchTemplate", pluginID, scraper.ID, key)
		}
		if slices.Contains(scraper.RequiredSearchHints[:i], key) {
			return fmt.Errorf("plugin %s: scraper %q duplicate required search hint %q", pluginID, scraper.ID, key)
		}
	}
	return nil
}

// validateImpliedMediaFields enforces the ImpliedMediaFields contract for one
// scraper: every key routes to a DECLARED plugin text media field (never a
// builtin) and carries a non-blank value. Shared by the remote and local-file
// scraper paths — local-file kinds skip transport rules, not this one.
func validateImpliedMediaFields(pluginID ID, scraper ScraperSpec, mediaFields map[string]string) error {
	for key, value := range scraper.ImpliedMediaFields {
		route, err := classifyMediaFieldKey(key, mediaFields)
		if err != nil {
			return fmt.Errorf("plugin %s: scraper %q implied media field: %w", pluginID, scraper.ID, err)
		}
		if route.Builtin {
			return fmt.Errorf("plugin %s: scraper %q implies built-in field %q; only plugin media fields (%s<name>) are implied", pluginID, scraper.ID, key, MediaFieldKeyPrefix)
		}
		if route.Type != "text" {
			return fmt.Errorf("plugin %s: scraper %q implies media field %q of type %q, only text fields can be implied", pluginID, scraper.ID, route.MediaField, route.Type)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("plugin %s: scraper %q implies media field %q without a value", pluginID, scraper.ID, route.MediaField)
		}
	}
	return nil
}

// validAuthRef reports whether ref is a plain credential name: non-empty, no
// path separators, not hidden. Hosts store one secret per AuthRef file.
func validAuthRef(ref string) bool {
	return ref != "" && !strings.ContainsAny(ref, "/\\") && !strings.HasPrefix(ref, ".")
}

func validateLocalFileScraper(pluginID ID, scraper ScraperSpec) error {
	switch {
	case scraper.Endpoint != "":
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q and must not declare an endpoint", pluginID, scraper.ID, scraper.Kind)
	case scraper.LookupTemplate != "" || scraper.SearchTemplate != "" || scraper.ItemLookupTemplate != "":
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q and must not declare URL templates", pluginID, scraper.ID, scraper.Kind)
	case len(scraper.RequiredSearchHints) != 0:
		return fmt.Errorf("plugin %s: scraper %q required search hints need a search template", pluginID, scraper.ID)
	case scraper.AuthRef != "" || scraper.AuthStyle != "" || scraper.Login != nil:
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q and must not declare auth", pluginID, scraper.ID, scraper.Kind)
	case scraper.CandidatesPath != "" || scraper.CandidateIDPath != "":
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q and must not declare candidate paths", pluginID, scraper.ID, scraper.Kind)
	case len(scraper.FieldMap) != 0 || len(scraper.SearchFieldMap) != 0 || len(scraper.ArtworkMap) != 0 || len(scraper.SearchArtworkMap) != 0 ||
		len(scraper.ItemFieldMap) != 0 || len(scraper.ItemArtworkMap) != 0:
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q: it emits a fixed generic field set, not document-path field maps", pluginID, scraper.ID, scraper.Kind)
	case len(scraper.EntityMaps) != 0:
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q and must not declare entity maps", pluginID, scraper.ID, scraper.Kind)
	case scraper.CredentialHelp != nil || len(scraper.ReferenceDocuments) != 0 || len(scraper.LookupByIDRecordIDPaths) != 0:
		return fmt.Errorf("plugin %s: scraper %q is a local-file kind %q and must not declare credential help, reference documents or an id-lookup record path", pluginID, scraper.ID, scraper.Kind)
	}
	return nil
}

func validateFieldFilterValue(fieldType string, value any) error {
	if raw, ok := value.(map[string]any); ok {
		if fieldType == "text" || fieldType == "url" || fieldType == "flag" {
			return fmt.Errorf("range filters are only valid on number and date fields")
		}
		if len(raw) == 0 {
			return fmt.Errorf("range filter requires gte or lte")
		}
		hasBound := false
		for key, bound := range raw {
			switch key {
			case "gte", "lte":
				hasBound = true
			default:
				return fmt.Errorf("range key %q is not supported", key)
			}
			switch fieldType {
			case "number":
				if !isNumberValue(bound) {
					return fmt.Errorf("range bound %q must be a number", key)
				}
			case "date":
				if _, ok := parseFilterTime(bound); !ok {
					return fmt.Errorf("range bound %q must be a date", key)
				}
			default:
				return fmt.Errorf("range filters are only valid on number and date fields")
			}
		}
		if !hasBound {
			return fmt.Errorf("range filter requires gte or lte")
		}
		return nil
	}
	switch fieldType {
	case "text", "url":
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return fmt.Errorf("text filters require a non-blank string")
		}
	case "number":
		if !isNumberValue(value) {
			return fmt.Errorf("number filters require a number")
		}
	case "date":
		if _, ok := parseFilterTime(value); !ok {
			return fmt.Errorf("date filters require a date string or time")
		}
	case "flag":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("flag filters require a boolean")
		}
	default:
		return fmt.Errorf("unknown field type %q", fieldType)
	}
	return nil
}

func validateLabelMaps(bundle Bundle) error {
	seen := map[string]bool{}
	for _, m := range bundle.LabelMaps {
		label := normalizedLabelMapLabel(m.Label)
		if label == "" {
			return fmt.Errorf("plugin %s: label map label is empty after normalization", bundle.ID)
		}
		if m.MinConfidence < 0 || m.MinConfidence > 1 || m.MinConfidence != m.MinConfidence {
			return fmt.Errorf("plugin %s: label map %q confidence %v is outside [0,1]", bundle.ID, m.Label, m.MinConfidence)
		}
		target, err := labelMapTarget(bundle, m)
		if err != nil {
			return err
		}
		key := label + "\x00" + target
		if seen[key] {
			return fmt.Errorf("plugin %s: label map %q duplicates normalized label %q and target %q", bundle.ID, m.Label, label, target)
		}
		seen[key] = true
	}
	return nil
}

func normalizedLabelMapLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(norm.NFC.String(s))), " ")
}

func labelMapTarget(bundle Bundle, m LabelMapSpec) (string, error) {
	tag := strings.TrimSpace(m.Tag)
	hasTag := tag != ""
	hasEntity := m.Kind != "" || m.Relation != "" || m.EntityName != ""
	if hasTag == hasEntity {
		return "", fmt.Errorf("plugin %s: label map %q must set exactly one target group: Tag or Kind+Relation(+EntityName)", bundle.ID, m.Label)
	}
	if hasTag {
		return "tag:" + tag, nil
	}
	if m.Kind == "" || m.Relation == "" {
		return "", fmt.Errorf("plugin %s: label map %q entity target requires Kind and Relation", bundle.ID, m.Label)
	}
	for _, k := range bundle.EntityKinds {
		if k.Name != m.Kind {
			continue
		}
		for _, r := range k.Relations {
			if r.Name == m.Relation && r.ToKind == MediaKindName {
				return "entity:" + m.Kind + ":" + m.Relation + ":" + m.EntityName, nil
			}
		}
		return "", fmt.Errorf("plugin %s: label map %q relation %q is not a Media-targeted relation on kind %q", bundle.ID, m.Label, m.Relation, m.Kind)
	}
	return "", fmt.Errorf("plugin %s: label map %q kind %q is not declared", bundle.ID, m.Label, m.Kind)
}

// isNumberValue reports whether value is a usable numeric filter operand.
// NaN and ±Inf are rejected: a NaN bound compares false against everything
// (silently empty feeds) and infinities are never a meaningful declared
// bound — consistent with the explicit NaN rejection on MinConfidence.
func isNumberValue(value any) bool {
	switch v := value.(type) {
	case float64:
		return !math.IsNaN(v) && !math.IsInf(v, 0)
	case float32:
		f := float64(v)
		return !math.IsNaN(f) && !math.IsInf(f, 0)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

func parseFilterTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		if typed.IsZero() {
			return time.Time{}, false
		}
		return typed.UTC(), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return time.Time{}, false
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
			when, err := time.Parse(layout, text)
			if err == nil {
				return when.UTC(), true
			}
		}
	}
	return time.Time{}, false
}

func validArtworkURLTemplate(tmpl string) bool {
	return strings.HasPrefix(tmpl, "https://") && strings.Contains(tmpl, "{value}")
}

// CompatibleWithSDK reports whether sdkVersion (a released plugin-sdk semver,
// e.g. sdk.Version) falls within bundle's required [SDKFloor, SDKCeiling]
// compat range and supported major. sdkVersion must be semver; malformed metadata is treated as
// incompatible rather than panicking.
func CompatibleWithSDK(bundle Bundle, sdkVersion string) bool {
	if validateSDKCompatibility(bundle) != nil {
		return false
	}
	if !semverPattern.MatchString(sdkVersion) {
		return false
	}
	if releaseTriple(sdkVersion)[0] != releaseTriple(bundle.SDKFloor)[0] {
		return false
	}
	if compareSemver(sdkVersion, bundle.SDKFloor) < 0 {
		return false
	}
	if compareSemver(sdkVersion, bundle.SDKCeiling) > 0 {
		return false
	}
	return true
}

// compareSemver orders two semver strings per SemVer §11 precedence: first by
// the numeric release triple (major.minor.patch), then — when the triples are
// equal — a version WITH a pre-release suffix sorts BEFORE the same triple
// without one, and two pre-releases of the same triple compare field-by-field
// by dot-separated identifier rules (numeric identifiers compare numerically
// and rank below alphanumeric ones, which compare in ASCII order; a shorter
// prefix of otherwise-equal fields sorts lower). Build metadata (+…) is always
// ignored. Both inputs must already match semverPattern. Returns <0, 0, >0 like
// strings.Compare.
func compareSemver(a, b string) int {
	ax, bx := releaseTriple(a), releaseTriple(b)
	for i := 0; i < 3; i++ {
		if ax[i] != bx[i] {
			return ax[i] - bx[i]
		}
	}
	return comparePreRelease(preRelease(a), preRelease(b))
}

// preRelease returns the pre-release identifier of a semver string (the segment
// after "-" in the version core), or "" when there is no pre-release suffix.
// Build metadata is stripped BEFORE looking for the pre-release hyphen: metadata
// identifiers may themselves contain "-" (e.g. "1.0.0+build-2"), and cutting on
// the whole string's first hyphen would fabricate a phantom pre-release from
// them.
func preRelease(v string) string {
	v, _, _ = strings.Cut(v, "+")
	_, pre, _ := strings.Cut(v, "-")
	return pre
}

// comparePreRelease orders two pre-release identifiers per SemVer §11: an empty
// pre-release (a normal release) ranks ABOVE any non-empty one; otherwise the
// dot-separated fields compare pairwise (numeric < alphanumeric; numerics
// numerically; alphanumerics in ASCII order), and a shorter run of equal fields
// sorts lower. Returns <0, 0, >0 like strings.Compare.
func comparePreRelease(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return 1
	}
	if b == "" {
		return -1
	}
	af, bf := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(af) && i < len(bf); i++ {
		if c := comparePreReleaseField(af[i], bf[i]); c != 0 {
			return c
		}
	}
	return len(af) - len(bf)
}

// comparePreReleaseField orders one pair of pre-release identifier fields:
// numeric fields rank below alphanumeric ones; two numerics compare
// numerically; two alphanumerics compare in ASCII order.
func comparePreReleaseField(a, b string) int {
	an, aNum := strconv.Atoi(a)
	bn, bNum := strconv.Atoi(b)
	switch {
	case aNum == nil && bNum == nil:
		return an - bn
	case aNum == nil:
		return -1
	case bNum == nil:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// releaseTriple extracts the [major, minor, patch] integers from a semver
// string, dropping any pre-release/build suffix.
func releaseTriple(v string) [3]int {
	core, _, _ := strings.Cut(v, "-")
	core, _, _ = strings.Cut(core, "+")
	var out [3]int
	for i, part := range strings.SplitN(core, ".", 3) {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(part)
		out[i] = n
	}
	return out
}

// validateLookupByIDRecordIDPaths checks that each id-lookup record path
// resolves a declared LookupByIDTemplate provider into this http-json
// scraper's own LookupTemplate.
func validateLookupByIDRecordIDPaths(scraper ScraperSpec) error {
	if len(scraper.LookupByIDRecordIDPaths) == 0 {
		return nil
	}
	if scraper.Kind != "http-json" || scraper.LookupTemplate == "" {
		return fmt.Errorf("scraper %q declares LookupByIDRecordIDPaths, which needs an http-json scraper with a LookupTemplate", scraper.ID)
	}
	for _, provider := range slices.Sorted(maps.Keys(scraper.LookupByIDRecordIDPaths)) {
		if _, ok := scraper.LookupByIDTemplate[provider]; !ok {
			return fmt.Errorf("scraper %q declares LookupByIDRecordIDPaths[%q] without a LookupByIDTemplate for it", scraper.ID, provider)
		}
		if strings.TrimSpace(scraper.LookupByIDRecordIDPaths[provider]) == "" {
			return fmt.Errorf("scraper %q declares an empty LookupByIDRecordIDPaths[%q]", scraper.ID, provider)
		}
	}
	return nil
}

// validateItemLookup checks a scraper's per-item lookup: the item maps need an
// ItemLookupTemplate, which needs an http-json scraper with a LookupTemplate
// and must contain {id}; item map keys follow FieldMap's and ArtworkMap's
// vocabularies.
func validateItemLookup(scraper ScraperSpec, mediaFields map[string]string) error {
	if scraper.ItemLookupTemplate == "" {
		if len(scraper.ItemFieldMap) != 0 || len(scraper.ItemArtworkMap) != 0 {
			return errors.New("item lookup: declares item maps without an ItemLookupTemplate")
		}
		return nil
	}
	if scraper.Kind != "http-json" || scraper.LookupTemplate == "" {
		return errors.New("item lookup: ItemLookupTemplate needs an http-json scraper with a LookupTemplate")
	}
	if !strings.Contains(scraper.ItemLookupTemplate, "{id}") {
		return errors.New("item lookup: ItemLookupTemplate must contain {id}")
	}
	if len(scraper.ItemFieldMap) == 0 && len(scraper.ItemArtworkMap) == 0 {
		return errors.New("item lookup: ItemLookupTemplate declares neither ItemFieldMap nor ItemArtworkMap")
	}
	for _, key := range slices.Sorted(maps.Keys(scraper.ItemFieldMap)) {
		if _, err := classifyMediaFieldKey(key, mediaFields); err != nil {
			return fmt.Errorf("item field map: %w", err)
		}
		if strings.TrimSpace(scraper.ItemFieldMap[key]) == "" {
			return fmt.Errorf("item field map entry %q has a blank path", key)
		}
	}
	for _, slot := range slices.Sorted(maps.Keys(scraper.ItemArtworkMap)) {
		if !slices.Contains(ArtworkSlots(), slot) {
			return fmt.Errorf("item artwork map: unknown artwork slot %q", slot)
		}
		if strings.TrimSpace(scraper.ItemArtworkMap[slot]) == "" {
			return fmt.Errorf("item artwork map entry %q has a blank path", slot)
		}
	}
	return nil
}

// validateEntityMapRelationValues checks the values an EntityMap writes on its
// relation row and the elements it keeps: RelationAttributeMap keys are plain
// attributes the relation declares, with non-blank paths; ElementFilter
// paths are non-blank with at least one accepted value; OrdinalField names a
// FieldMap or RelationAttributeMap key; OrdinalOffset is not negative.
// declared is nil for a Container map, which names no relation.
func validateEntityMapRelationValues(entityMap EntityMap, declared map[string]bool) error {
	for _, attribute := range slices.Sorted(maps.Keys(entityMap.RelationAttributeMap)) {
		if !declared[attribute] {
			return fmt.Errorf("entity map for kind %q maps relation attribute %q that relation %q does not declare", entityMap.Kind, attribute, entityMap.Relation)
		}
		if strings.TrimSpace(entityMap.RelationAttributeMap[attribute]) == "" {
			return fmt.Errorf("entity map for kind %q relation attribute %q has a blank path", entityMap.Kind, attribute)
		}
	}
	for _, path := range slices.Sorted(maps.Keys(entityMap.ElementFilter)) {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("entity map for kind %q has a blank element filter path", entityMap.Kind)
		}
		if len(entityMap.ElementFilter[path]) == 0 {
			return fmt.Errorf("entity map for kind %q element filter %q accepts no values", entityMap.Kind, path)
		}
	}
	if entityMap.OrdinalField != "" {
		_, field := entityMap.FieldMap[entityMap.OrdinalField]
		_, attribute := entityMap.RelationAttributeMap[entityMap.OrdinalField]
		if !field && !attribute {
			return fmt.Errorf("entity map for kind %q declares ordinal field %q that is not in its field map or relation attribute map", entityMap.Kind, entityMap.OrdinalField)
		}
	}
	if entityMap.OrdinalOffset < 0 {
		return fmt.Errorf("entity map for kind %q ordinal offset must not be negative", entityMap.Kind)
	}
	return nil
}

// validateContainerEntityMap checks a Container entity map: it hydrates an
// existing container Entity, so it names no relation, creates no parent and
// fetches no secondary documents, and it must hydrate something.
func validateContainerEntityMap(entityMap EntityMap, containerKinds map[string]bool) error {
	switch {
	case !containerKinds[entityMap.Kind]:
		return fmt.Errorf("container entity map for kind %q: kind is not a container archetype", entityMap.Kind)
	case entityMap.Relation != "":
		return fmt.Errorf("container entity map for kind %q must not declare a relation", entityMap.Kind)
	case entityMap.LookupTemplate != "" || entityMap.LookupIDPath != "" || entityMap.MaxLookups != 0:
		return fmt.Errorf("container entity map for kind %q must not declare a reference lookup", entityMap.Kind)
	case entityMap.ParentNamePath != "" || entityMap.ParentRelation != "":
		return fmt.Errorf("container entity map for kind %q must not declare a parent", entityMap.Kind)
	case entityMap.OrdinalField != "" || entityMap.OrdinalOffset != 0:
		return fmt.Errorf("container entity map for kind %q must not declare an ordinal field or offset", entityMap.Kind)
	case len(entityMap.RelationAttributeMap) != 0:
		return fmt.Errorf("container entity map for kind %q must not declare relation attributes", entityMap.Kind)
	case len(entityMap.ElementFilter) != 0:
		return fmt.Errorf("container entity map for kind %q must not declare an element filter", entityMap.Kind)
	case entityMap.NamePath != "":
		return fmt.Errorf("container entity map for kind %q must not declare a name path", entityMap.Kind)
	case strings.TrimSpace(entityMap.ArtworkPath) == "" && len(entityMap.FieldMap) == 0 && len(entityMap.ArtworkMap) == 0 && entityMap.TitlePath == "":
		return fmt.Errorf("container entity map for kind %q declares no artwork, field map or title path", entityMap.Kind)
	}
	return nil
}

// validateContainerMapExtras checks the Container-only EntityMap fields:
// ArtworkMap slots are core artwork slots with non-blank paths and do not
// repeat ArtworkPath's poster; TitlePath is non-blank; each ContainerRelations
// key is a relation Kind declares toward an Entity kind, and its value is the
// Relation of a non-container map of that kind in the same scraper.
func validateContainerMapExtras(bundle Bundle, scraper ScraperSpec, entityMap EntityMap) error {
	if !entityMap.Container {
		if len(entityMap.ArtworkMap) != 0 || entityMap.TitlePath != "" || len(entityMap.ContainerRelations) != 0 {
			return fmt.Errorf("entity map for kind %q declares an artwork map, title path or container relations without being a container map", entityMap.Kind)
		}
		return nil
	}
	for _, slot := range slices.Sorted(maps.Keys(entityMap.ArtworkMap)) {
		if !slices.Contains(ArtworkSlots(), slot) {
			return fmt.Errorf("container entity map for kind %q maps unknown artwork slot %q", entityMap.Kind, slot)
		}
		if strings.TrimSpace(entityMap.ArtworkMap[slot]) == "" {
			return fmt.Errorf("container entity map for kind %q artwork slot %q has a blank path", entityMap.Kind, slot)
		}
		if slot == "poster" && entityMap.ArtworkPath != "" {
			return fmt.Errorf("container entity map for kind %q declares the poster by both artwork path and artwork map", entityMap.Kind)
		}
	}
	if entityMap.TitlePath != "" && strings.TrimSpace(entityMap.TitlePath) == "" {
		return fmt.Errorf("container entity map for kind %q has a blank title path", entityMap.Kind)
	}
	toKinds := map[string]string{}
	for _, kind := range bundle.EntityKinds {
		if kind.Name != entityMap.Kind {
			continue
		}
		for _, relation := range kind.Relations {
			if relation.ToKind != MediaKindName {
				toKinds[relation.Name] = relation.ToKind
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(entityMap.ContainerRelations)) {
		toKind, declared := toKinds[name]
		if !declared {
			return fmt.Errorf("container entity map for kind %q links relation %q that the kind does not declare toward an entity kind", entityMap.Kind, name)
		}
		sibling := entityMap.ContainerRelations[name]
		if !slices.ContainsFunc(scraper.EntityMaps, func(other EntityMap) bool {
			return !other.Container && other.Kind == toKind && other.Relation == sibling
		}) {
			return fmt.Errorf("container entity map for kind %q relation %q names %q, which no %q entity map in the scraper declares", entityMap.Kind, name, sibling, toKind)
		}
		if slices.ContainsFunc(scraper.EntityMaps, func(other EntityMap) bool {
			return !other.Container && other.Kind != toKind && other.Relation == sibling
		}) {
			return fmt.Errorf("container entity map for kind %q relation %q names %q, which an entity map of a kind other than %q also declares", entityMap.Kind, name, sibling, toKind)
		}
	}
	return nil
}
