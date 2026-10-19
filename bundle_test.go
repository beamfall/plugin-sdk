package sdk

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func validateReleaseVersion(version, candidateTag, buildTag string, tagExists func(string) bool) error {
	wantTag := "v" + version
	if buildTag != "" && buildTag != wantTag {
		return fmt.Errorf("tagged build %q embeds SDK Version %q; want tag %q", buildTag, version, wantTag)
	}
	if candidateTag == "" {
		return nil
	}
	if buildTag != "" {
		return fmt.Errorf("candidate mode cannot be combined with tagged-build mode")
	}
	if candidateTag != wantTag {
		return fmt.Errorf("candidate tag %q does not match SDK Version %q; want %q", candidateTag, version, wantTag)
	}
	if tagExists(candidateTag) {
		return fmt.Errorf("candidate tag %q already exists", candidateTag)
	}
	return nil
}

func TestReleaseVersionInvariant(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		candidateTag string
		buildTag     string
		usedTags     map[string]bool
		wantErr      bool
	}{
		{name: "historical v0.1.10 mismatch", version: "0.1.7", buildTag: "v0.1.10", wantErr: true},
		{name: "historical v0.1.11 mismatch", version: "0.1.7", buildTag: "v0.1.11", wantErr: true},
		{name: "historical v0.3.0 mismatch", version: "0.2.0+dev.audit0728.nc-sdk-compatibility-1", buildTag: "v0.3.0", wantErr: true},
		{name: "unused v0.3.2 candidate", version: "0.3.2", candidateTag: "v0.3.2"},
		{name: "reused candidate tag", version: "0.3.2", candidateTag: "v0.3.2", usedTags: map[string]bool{"v0.3.2": true}, wantErr: true},
		{name: "exact matching tagged build", version: "0.3.2", buildTag: "v0.3.2"},
		{name: "mismatched tagged build", version: "0.3.2", buildTag: "v0.3.3", wantErr: true},
		{name: "ordinary untagged development", version: "0.3.2"},
		{name: "development identity rejects immutable release tag", version: "0.2.0+dev.audit0728.nc-sdk-compatibility-1", buildTag: "v0.2.0", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tagExists := func(tag string) bool { return tt.usedTags[tag] }
			err := validateReleaseVersion(tt.version, tt.candidateTag, tt.buildTag, tagExists)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateReleaseVersion(%q, %q, %q) error = %v, wantErr %v", tt.version, tt.candidateTag, tt.buildTag, err, tt.wantErr)
			}
		})
	}
}

func TestReleaseVersionContext(t *testing.T) {
	candidateTag := os.Getenv("BEAMFALL_SDK_CANDIDATE_TAG")
	buildTag := ""
	if os.Getenv("GITHUB_REF_TYPE") == "tag" {
		buildTag = os.Getenv("GITHUB_REF_NAME")
		if buildTag == "" {
			t.Fatal("GITHUB_REF_TYPE=tag requires GITHUB_REF_NAME")
		}
	}
	tagExists := func(tag string) bool {
		out, err := exec.Command("git", "tag", "--list", tag).Output()
		if err != nil {
			t.Fatalf("inspect local tags: %v", err)
		}
		return strings.TrimSpace(string(out)) != ""
	}
	if err := validateReleaseVersion(Version, candidateTag, buildTag, tagExists); err != nil {
		t.Fatal(err)
	}
}

// TestBrowseLayoutFound: BrowseLayout returns the one Layout with Role
// "browse" among several, with ok=true.
func TestBrowseLayoutFound(t *testing.T) {
	b := Bundle{Layouts: []LayoutSpec{
		{ID: "detail-1", Role: "detail"},
		{ID: "home", Role: "browse"},
	}}
	got, ok := b.BrowseLayout()
	if !ok || got.ID != "home" {
		t.Errorf("BrowseLayout() = (%+v, %v), want (ID %q, true)", got, ok, "home")
	}
}

// TestBrowseLayoutAbsent: a bundle with no browse Layout (unvalidated) yields
// the zero LayoutSpec and ok=false rather than panicking.
func TestBrowseLayoutAbsent(t *testing.T) {
	b := Bundle{Layouts: []LayoutSpec{{ID: "detail-1", Role: "detail"}}}
	got, ok := b.BrowseLayout()
	if ok || got.ID != "" || got.Role != "" || got.Sections != nil {
		t.Errorf("BrowseLayout() = (%+v, %v), want (zero value, false)", got, ok)
	}
}

// TestFamilyScopeValid covers the empty-string-always-valid rule plus the
// declared/undeclared scope-value branches.
func TestFamilyScopeValid(t *testing.T) {
	b := Bundle{FamilyScopes: []FamilyScopeSpec{
		{Value: "films", DisplayName: L("Films"), Default: true},
		{Value: "tvshows", DisplayName: L("TV Shows")},
	}}
	if !b.FamilyScopeValid("") {
		t.Error("the empty scope must always be valid")
	}
	if !b.FamilyScopeValid("films") {
		t.Error("a declared scope must be valid")
	}
	if b.FamilyScopeValid("music") {
		t.Error("an undeclared scope must be invalid")
	}
}

// TestFamilyScopeValidNoScopesDeclared: a bundle declaring no FamilyScopes
// still accepts the empty string.
func TestFamilyScopeValidNoScopesDeclared(t *testing.T) {
	var b Bundle
	if !b.FamilyScopeValid("") {
		t.Error("the empty scope must be valid even with no declared scopes")
	}
	if b.FamilyScopeValid("anything") {
		t.Error("any non-empty scope must be invalid with no declared scopes")
	}
}

func TestValidateBundleRejectsDeclaredVocabularyMiss(t *testing.T) {
	b := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts: []LayoutSpec{{
			ID:   "browse",
			Role: "browse",
			Sections: []SectionSpec{{
				ID:     "bad",
				Kind:   "shelf",
				Source: SourceSpec{Kind: "availability", Value: "not-a-band"},
			}},
		}},
	}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an invalid declared Source.Value vocabulary entry")
	}
}

// TestValidateBundleFieldFilters exercises the per-class/per-genre filter-
// parameter seam: a Source.FieldFilters entry must name a real
// declared MediaField of the correct type, and only FilterableSourceKinds
// accept filters at all.
func TestValidateBundleFieldFilters(t *testing.T) {
	base := func() Bundle {
		return Bundle{
			ID:           "smoke",
			Version:      "0.1.0",
			SDKFloor:     Version,
			SDKCeiling:   Version,
			DisplayName:  L("Smoke"),
			MediaLabel:   L("Item"),
			MediaClasses: []string{"video"},
			MediaFields: []FieldSpec{
				{Name: "genre", Type: "text", Label: L("Genre")},
				{Name: "is-hd", Type: "flag", Label: L("HD")},
			},
			Layouts: []LayoutSpec{{
				ID:   "browse",
				Role: "browse",
				Sections: []SectionSpec{{
					ID:     "s",
					Kind:   "shelf",
					Source: SourceSpec{Kind: "all-items", FieldFilters: map[string]any{"genre": "comedy"}},
				}},
			}},
		}
	}

	if err := ValidateBundle(base()); err != nil {
		t.Fatalf("ValidateBundle rejected a filter on a real declared field of the correct type: %v", err)
	}

	unknownField := base()
	unknownField.Layouts[0].Sections[0].Source.FieldFilters = map[string]any{"not-a-field": "x"}
	if err := ValidateBundle(unknownField); err == nil {
		t.Fatal("ValidateBundle accepted a filter naming an undeclared media field")
	}

	wrongType := base()
	wrongType.Layouts[0].Sections[0].Source.FieldFilters = map[string]any{"is-hd": "not-a-bool"}
	if err := ValidateBundle(wrongType); err == nil {
		t.Fatal("ValidateBundle accepted a filter value of the wrong shape for the field's declared type")
	}

	nonFilterable := base()
	nonFilterable.Layouts[0].Sections[0].Source = SourceSpec{Kind: "entity-media", FieldFilters: map[string]any{"genre": "comedy"}}
	if err := ValidateBundle(nonFilterable); err == nil {
		t.Fatal("ValidateBundle accepted a field filter on a non-filterable Source kind")
	}
}

func TestValidateBundleFeatureEligibility(t *testing.T) {
	b := Bundle{
		ID:                 "smoke",
		Version:            "0.1.0",
		SDKFloor:           Version,
		SDKCeiling:         Version,
		DisplayName:        L("Smoke"),
		MediaLabel:         L("Item"),
		MediaClasses:       []string{"audio"},
		FeatureEligibility: []string{FeatureAudioStation},
		Layouts:            []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	if err := ValidateBundle(b); err != nil {
		t.Fatalf("ValidateBundle rejected feature eligibility: %v", err)
	}

	b.FeatureEligibility = []string{"not-real"}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an unknown feature eligibility")
	}

	b.FeatureEligibility = []string{FeatureAudioStation, FeatureAudioStation}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted duplicate feature eligibility")
	}
}

func TestValidateBundlePlaybackAffordancesSection(t *testing.T) {
	b := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		EntityKinds:  []EntityKind{{Name: "mission", Archetype: "container"}},
		Layouts: []LayoutSpec{{
			ID:   "browse",
			Role: "browse",
		}, {
			ID:         "mission-detail",
			Role:       "detail",
			EntityKind: "mission",
			Sections: []SectionSpec{{
				ID:                  "playback",
				Kind:                "playback-affordances",
				Source:              SourceSpec{Kind: "entity-media"},
				PlaybackAffordances: []string{"resume", "direct-play", "airplay"},
			}},
		}},
	}
	if err := ValidateBundle(b); err != nil {
		t.Fatalf("ValidateBundle rejected playback affordances: %v", err)
	}

	b.Layouts[1].Sections[0].Kind = "shelf"
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted playback affordances on a shelf")
	}

	b.Layouts[1].Sections[0].Kind = "playback-affordances"
	b.Layouts[1].Sections[0].PlaybackAffordances = []string{"cast"}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an unknown playback affordance")
	}

	b.Layouts[1].Sections[0].PlaybackAffordances = nil
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted playback-affordances without affordances")
	}
}

func TestValidateBundleIdentificationPolicy(t *testing.T) {
	base := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	for _, policy := range []IdentificationPolicy{"", IdentificationPolicyAuto, IdentificationPolicyQueue} {
		b := base
		b.IdentificationPolicy = policy
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle(%q): %v", policy, err)
		}
	}

	b := base
	b.IdentificationPolicy = "manual"
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an invalid identification policy")
	}
}

func TestValidateBundleRejectsInvalidSDKFloor(t *testing.T) {
	b := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     "next",
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an invalid SDK floor")
	}
}

func TestValidateBundleRequiresSDKCompatibilityBounds(t *testing.T) {
	missingFloor := validBundle()
	missingFloor.SDKFloor = ""
	if err := ValidateBundle(missingFloor); err == nil {
		t.Fatal("ValidateBundle accepted a missing SDK floor")
	}

	missingCeiling := validBundle()
	missingCeiling.SDKCeiling = ""
	if err := ValidateBundle(missingCeiling); err == nil {
		t.Fatal("ValidateBundle accepted a missing SDK ceiling")
	}
}

func TestValidateBundleRejectsInvalidSDKCeiling(t *testing.T) {
	b := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   "next",
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an invalid SDK ceiling")
	}
}

func TestValidateBundleRejectsCrossMajorSDKRange(t *testing.T) {
	b := validBundle()
	b.SDKFloor = "0.2.0"
	b.SDKCeiling = "1.0.0"
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an SDK range spanning majors")
	}
}

func TestValidateBundleRejectsInvertedSDKRange(t *testing.T) {
	b := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     "0.5.0",
		SDKCeiling:   "0.2.0",
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	if err := ValidateBundle(b); err == nil {
		t.Fatal("ValidateBundle accepted an SDK floor newer than its ceiling")
	}
}

func TestCompatibleWithSDK(t *testing.T) {
	base := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	cases := []struct {
		name           string
		floor, ceiling string
		sdkVersion     string
		want           bool
	}{
		{"missing bounds are incompatible", "", "", "9.9.9", false},
		{"within range", "0.1.0", "0.2.0", "0.1.6", true},
		{"below floor", "0.5.0", "0.6.0", "0.1.6", false},
		{"above ceiling", "0.1.0", "0.1.5", "0.1.6", false},
		{"equals floor", "0.1.6", "0.2.0", "0.1.6", true},
		{"equals ceiling", "0.1.0", "0.1.6", "0.1.6", true},
		{"malformed sdk version", "0.1.0", "0.2.0", "next", false},
		// SemVer §11: a pre-release sorts BELOW its release triple.
		{"prerelease sdk below its release floor", "0.1.6", "0.2.0", "0.1.6-rc1", false},
		{"release sdk meets floor its prerelease misses", "0.1.6", "0.2.0", "0.1.6", true},
		{"prerelease ceiling excludes the release", "0.1.0", "0.1.6-rc1", "0.1.6", false},
		{"prerelease ceiling admits an earlier prerelease", "0.1.0", "0.1.6-rc2", "0.1.6-rc1", true},
		{"prerelease ceiling excludes a later prerelease", "0.1.0", "0.1.6-rc1", "0.1.6-rc2", false},
		{"new SDK major is incompatible", "0.1.0", "0.9.9", "1.0.0", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := base
			b.SDKFloor = tc.floor
			b.SDKCeiling = tc.ceiling
			if got := CompatibleWithSDK(b, tc.sdkVersion); got != tc.want {
				t.Errorf("CompatibleWithSDK(floor=%q, ceiling=%q, sdkVersion=%q) = %v, want %v",
					tc.floor, tc.ceiling, tc.sdkVersion, got, tc.want)
			}
		})
	}
}

func TestValidateBundleContainerPolicyFields(t *testing.T) {
	base := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		EntityKinds: []EntityKind{{
			Name:            "season",
			Archetype:       "container",
			OrderedChildren: true,
			Fields: []FieldSpec{
				{Name: "special", Type: "flag"},
				{Name: "bonus", Type: "flag"},
				{Name: "ordinal", Type: "number"},
			},
		}},
		Layouts: []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	base.EntityKinds[0].SortLastField = "special"
	base.EntityKinds[0].ExcludeAutoAdvanceField = "bonus"
	if err := ValidateBundle(base); err != nil {
		t.Fatalf("ValidateBundle rejected valid container-policy fields: %v", err)
	}

	unknown := base
	unknown.EntityKinds = []EntityKind{base.EntityKinds[0]}
	unknown.EntityKinds[0].SortLastField = "nope"
	if err := ValidateBundle(unknown); err == nil {
		t.Fatal("ValidateBundle accepted a sort-last field naming an undeclared field")
	}

	nonFlag := base
	nonFlag.EntityKinds = []EntityKind{base.EntityKinds[0]}
	nonFlag.EntityKinds[0].ExcludeAutoAdvanceField = "ordinal"
	if err := ValidateBundle(nonFlag); err == nil {
		t.Fatal("ValidateBundle accepted an exclude-auto-advance field of non-flag type")
	}

	unordered := base
	unordered.EntityKinds = []EntityKind{base.EntityKinds[0]}
	unordered.EntityKinds[0].OrderedChildren = false
	unordered.EntityKinds[0].ExcludeAutoAdvanceField = ""
	if err := ValidateBundle(unordered); err == nil {
		t.Fatal("ValidateBundle accepted a sort-last field on a non-ordered container")
	}
}

// TestValidateBundleContainerPolicyArchetypes exercises the remaining two
// container-policy declarations: a multi-episode title-join rule and a rating-scale archetype,
// each naming a fixed archetype from a compiled-in vocabulary on a
// "container" kind.
func TestValidateBundleContainerPolicyArchetypes(t *testing.T) {
	base := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		EntityKinds: []EntityKind{{
			Name:      "season",
			Archetype: "container",
		}},
		Layouts: []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	base.EntityKinds[0].TitleJoinRule = "slash"
	base.EntityKinds[0].RatingScaleArchetype = "ordered-certification"
	if err := ValidateBundle(base); err != nil {
		t.Fatalf("ValidateBundle rejected valid container-policy archetypes: %v", err)
	}

	unknownJoin := base
	unknownJoin.EntityKinds = []EntityKind{base.EntityKinds[0]}
	unknownJoin.EntityKinds[0].TitleJoinRule = "not-a-real-rule"
	if err := ValidateBundle(unknownJoin); err == nil {
		t.Fatal("ValidateBundle accepted an unknown title-join rule")
	}

	unknownScale := base
	unknownScale.EntityKinds = []EntityKind{base.EntityKinds[0]}
	unknownScale.EntityKinds[0].RatingScaleArchetype = "not-a-real-scale"
	if err := ValidateBundle(unknownScale); err == nil {
		t.Fatal("ValidateBundle accepted an unknown rating-scale archetype")
	}

	nonContainerJoin := base
	nonContainerJoin.EntityKinds = []EntityKind{base.EntityKinds[0]}
	nonContainerJoin.EntityKinds[0].Archetype = "person"
	nonContainerJoin.EntityKinds[0].RatingScaleArchetype = ""
	if err := ValidateBundle(nonContainerJoin); err == nil {
		t.Fatal("ValidateBundle accepted a title-join rule on a non-container kind")
	}

	nonContainerScale := base
	nonContainerScale.EntityKinds = []EntityKind{base.EntityKinds[0]}
	nonContainerScale.EntityKinds[0].Archetype = "person"
	nonContainerScale.EntityKinds[0].TitleJoinRule = ""
	if err := ValidateBundle(nonContainerScale); err == nil {
		t.Fatal("ValidateBundle accepted a rating-scale archetype on a non-container kind")
	}
}

// storageSeamBundle is a minimal valid bundle carrying an ordered container
// kind with a number field, reused across the entity-storage seam tests.
func storageSeamBundle() Bundle {
	return Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		EntityKinds: []EntityKind{{
			Name:            "season",
			Archetype:       "container",
			OrderedChildren: true,
			Fields: []FieldSpec{
				{Name: "episode_no", Type: "number"},
				{Name: "special", Type: "flag"},
			},
		}},
		Layouts: []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

// TestValidateBundleEntityStorageSeams: the FTS-participation flag, orphan-GC
// policy, cursor-pagination capability, and ordering-source field are accepted
// generic-by-shape, and their reject paths fire.
func TestValidateBundleEntityStorageSeams(t *testing.T) {
	valid := storageSeamBundle()
	valid.EntityKinds[0].FullTextSearchable = true
	valid.EntityKinds[0].OrphanPolicy = "gc"
	valid.EntityKinds[0].CursorPaginated = true
	valid.EntityKinds[0].OrderingSourceField = "episode_no"
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid entity-storage seams: %v", err)
	}

	undeclared := storageSeamBundle()
	undeclared.EntityKinds[0].OrderingSourceField = "nope"
	if err := ValidateBundle(undeclared); err == nil {
		t.Fatal("ValidateBundle accepted an ordering-source field naming an undeclared field")
	}

	nonNumber := storageSeamBundle()
	nonNumber.EntityKinds[0].OrderingSourceField = "special"
	if err := ValidateBundle(nonNumber); err == nil {
		t.Fatal("ValidateBundle accepted an ordering-source field of non-number type")
	}

	unordered := storageSeamBundle()
	unordered.EntityKinds[0].OrderedChildren = false
	unordered.EntityKinds[0].OrderingSourceField = "episode_no"
	if err := ValidateBundle(unordered); err == nil {
		t.Fatal("ValidateBundle accepted an ordering-source field on a non-ordered container")
	}

	badPolicy := storageSeamBundle()
	badPolicy.EntityKinds[0].OrphanPolicy = "delete"
	if err := ValidateBundle(badPolicy); err == nil {
		t.Fatal("ValidateBundle accepted an unknown orphan policy")
	}
}

// TestValidateBundleRelationTypedAttributes: typed relation attributes are
// accepted with a valid FieldTypes type and rejected on unknown type or a name
// colliding with a bare string attribute.
func TestValidateBundleRelationTypedAttributes(t *testing.T) {
	base := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		EntityKinds: []EntityKind{{
			Name:      "person",
			Archetype: "person",
			Relations: []RelationSpec{{
				Name:       "credit",
				ToKind:     MediaKindName,
				Attributes: []string{"character"},
				TypedAttributes: []RelationAttributeSpec{
					{Name: "billing", Type: "number"},
				},
			}},
		}},
		Layouts: []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	if err := ValidateBundle(base); err != nil {
		t.Fatalf("ValidateBundle rejected a valid typed relation attribute: %v", err)
	}

	badType := base
	badType.EntityKinds = []EntityKind{base.EntityKinds[0]}
	badType.EntityKinds[0].Relations = []RelationSpec{{
		Name:            "credit",
		ToKind:          MediaKindName,
		TypedAttributes: []RelationAttributeSpec{{Name: "billing", Type: "bogus"}},
	}}
	if err := ValidateBundle(badType); err == nil {
		t.Fatal("ValidateBundle accepted a typed attribute with an unknown type")
	}

	collide := base
	collide.EntityKinds = []EntityKind{base.EntityKinds[0]}
	collide.EntityKinds[0].Relations = []RelationSpec{{
		Name:            "credit",
		ToKind:          MediaKindName,
		Attributes:      []string{"billing"},
		TypedAttributes: []RelationAttributeSpec{{Name: "billing", Type: "number"}},
	}}
	if err := ValidateBundle(collide); err == nil {
		t.Fatal("ValidateBundle accepted a typed attribute name colliding with a string attribute")
	}
}

// TestValidateBundlesCrossPlugin: ValidateBundles rejects two plugins that
// declare the same Entity Kind name (global namespace) or the same plugin ID,
// and accepts distinct ones.
func TestValidateBundlesCrossPlugin(t *testing.T) {
	one := Bundle{
		ID: "plugin-a", Version: "0.1.0", DisplayName: L("A"), MediaLabel: L("Item"),
		SDKFloor: Version, SDKCeiling: Version,
		MediaClasses: []string{"video"},
		EntityKinds:  []EntityKind{{Name: "show", Archetype: "container"}},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	two := Bundle{
		ID: "plugin-b", Version: "0.1.0", DisplayName: L("B"), MediaLabel: L("Item"),
		SDKFloor: Version, SDKCeiling: Version,
		MediaClasses: []string{"video"},
		EntityKinds:  []EntityKind{{Name: "album", Archetype: "container"}},
		Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
	if err := ValidateBundles([]Bundle{one, two}); err != nil {
		t.Fatalf("ValidateBundles rejected distinct plugins: %v", err)
	}

	collide := two
	collide.EntityKinds = []EntityKind{{Name: "show", Archetype: "container"}}
	if err := ValidateBundles([]Bundle{one, collide}); err == nil {
		t.Fatal("ValidateBundles accepted two plugins declaring the same kind name")
	}

	dupID := two
	dupID.ID = one.ID
	if err := ValidateBundles([]Bundle{one, dupID}); err == nil {
		t.Fatal("ValidateBundles accepted two bundles with the same plugin ID")
	}
}

// scraperBundle is a minimal valid bundle carrying a "genre" Entity Kind, a
// Media-targeted relation on it, and an http-json scraper with a lookup
// template — the fixture the scraper contract-seam tests mutate.
func scraperBundle() Bundle {
	return Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     Version,
		SDKCeiling:   Version,
		DisplayName:  L("Smoke"),
		MediaLabel:   L("Item"),
		MediaClasses: []string{"video"},
		EntityKinds: []EntityKind{{
			Name:      "genre",
			Archetype: "category",
			Relations: []RelationSpec{{Name: "genres", ToKind: MediaKindName}},
		}},
		Scrapers: []ScraperSpec{{
			ID:             "example",
			Kind:           "http-json",
			Endpoint:       "https://example.test",
			LookupTemplate: "/lookup/{id}",
			FieldMap:       map[string]string{"title": "name"},
		}},
		Layouts: []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

// TestValidateBundleScraperEntityMaps: an EntityMap mapping a lookup-document
// path to a declared kind's Media-targeted relation is accepted; targeting an
// undeclared kind or a relation that is not Media-targeted is rejected.
func TestValidateBundleScraperEntityMaps(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres"}}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid entity map: %v", err)
	}

	undeclaredKind := scraperBundle()
	undeclaredKind.Scrapers[0].EntityMaps = []EntityMap{{Kind: "nope", Relation: "genres", Path: "genres"}}
	if err := ValidateBundle(undeclaredKind); err == nil {
		t.Fatal("ValidateBundle accepted an entity map targeting an undeclared kind")
	}

	notMediaRelation := scraperBundle()
	notMediaRelation.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "nope", Path: "genres"}}
	if err := ValidateBundle(notMediaRelation); err == nil {
		t.Fatal("ValidateBundle accepted an entity map relation that is not Media-targeted")
	}

	noLookupTemplate := scraperBundle()
	noLookupTemplate.Scrapers[0].LookupTemplate = ""
	noLookupTemplate.Scrapers[0].SearchTemplate = "/search?q={query}"
	noLookupTemplate.Scrapers[0].CandidatesPath = "results"
	noLookupTemplate.Scrapers[0].EntityMaps = []EntityMap{{Kind: "genre", Relation: "genres", Path: "genres"}}
	if err := ValidateBundle(noLookupTemplate); err == nil {
		t.Fatal("ValidateBundle accepted an entity map with no lookup template to resolve it against")
	}
}

// TestValidateBundleScraperSearchPagination: a search template's declared
// pagination (MaxPages, CandidatesPath, CandidateIDPath) is accepted when
// well-formed and rejected on a negative page cap, a missing candidates path,
// or a missing candidate ID path when both templates are declared.
func TestValidateBundleScraperSearchPagination(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].SearchTemplate = "/search?q={query}&page={page}"
	valid.Scrapers[0].MaxPages = 5
	valid.Scrapers[0].CandidatesPath = "results"
	valid.Scrapers[0].CandidateIDPath = "id"
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid search pagination: %v", err)
	}

	negativePages := scraperBundle()
	negativePages.Scrapers[0].SearchTemplate = "/search?q={query}"
	negativePages.Scrapers[0].CandidatesPath = "results"
	negativePages.Scrapers[0].MaxPages = -1
	if err := ValidateBundle(negativePages); err == nil {
		t.Fatal("ValidateBundle accepted a negative MaxPages")
	}

	noCandidatesPath := scraperBundle()
	noCandidatesPath.Scrapers[0].SearchTemplate = "/search?q={query}"
	if err := ValidateBundle(noCandidatesPath); err == nil {
		t.Fatal("ValidateBundle accepted a search template with no candidates path")
	}

	noCandidateIDPath := scraperBundle()
	noCandidateIDPath.Scrapers[0].SearchTemplate = "/search?q={query}"
	noCandidateIDPath.Scrapers[0].CandidatesPath = "results"
	if err := ValidateBundle(noCandidateIDPath); err == nil {
		t.Fatal("ValidateBundle accepted search+lookup templates with no candidate ID path to chain them")
	}
}

// TestValidateBundleScraperFieldTypes: a scraper's declared FieldTypes are
// accepted with a valid FieldTypes vocabulary entry and rejected on an
// unknown type.
func TestValidateBundleScraperFieldTypes(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].FieldTypes = map[string]string{"title": "text"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid scraper field type: %v", err)
	}

	unknown := scraperBundle()
	unknown.Scrapers[0].FieldTypes = map[string]string{"title": "not-a-type"}
	if err := ValidateBundle(unknown); err == nil {
		t.Fatal("ValidateBundle accepted an unknown scraper field type")
	}
}

// TestValidateBundleLockableMediaFields exercises the field-lock declaration
// seam ("user edits outrank all sources"): a LockableMediaFields
// entry naming a built-in field or a declared "media.<name>" plugin field is
// accepted; a key naming an undeclared plugin field, or duplicated, is
// rejected.
func TestValidateBundleLockableMediaFields(t *testing.T) {
	base := func() Bundle {
		return Bundle{
			ID:           "smoke",
			Version:      "0.1.0",
			SDKFloor:     Version,
			SDKCeiling:   Version,
			DisplayName:  L("Smoke"),
			MediaLabel:   L("Item"),
			MediaClasses: []string{"video"},
			MediaFields: []FieldSpec{
				{Name: "tagline", Type: "text", Label: L("Tagline")},
			},
			Layouts: []LayoutSpec{{ID: "browse", Role: "browse"}},
		}
	}

	valid := base()
	valid.LockableMediaFields = []string{"summary", "year", MediaFieldKeyPrefix + "tagline"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid lockable media fields: %v", err)
	}

	undeclared := base()
	undeclared.LockableMediaFields = []string{MediaFieldKeyPrefix + "nope"}
	if err := ValidateBundle(undeclared); err == nil {
		t.Fatal("ValidateBundle accepted a lockable media field naming an undeclared plugin field")
	}

	duplicate := base()
	duplicate.LockableMediaFields = []string{"summary", "summary"}
	if err := ValidateBundle(duplicate); err == nil {
		t.Fatal("ValidateBundle accepted a duplicate lockable media field")
	}
}

// TestValidateBundleMediaFields exercises the Media-level typed field
// declaration seam (MediaFields carries the same FieldTypes vocabulary as
// Entity Kind fields): a valid declaration is accepted; an unknown field
// type or a duplicate field name is rejected.
func TestValidateBundleMediaFields(t *testing.T) {
	base := func() Bundle {
		return Bundle{
			ID:           "smoke",
			Version:      "0.1.0",
			SDKFloor:     Version,
			SDKCeiling:   Version,
			DisplayName:  L("Smoke"),
			MediaLabel:   L("Item"),
			MediaClasses: []string{"video"},
			Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
		}
	}

	valid := base()
	valid.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected a valid media field: %v", err)
	}

	unknownType := base()
	unknownType.MediaFields = []FieldSpec{{Name: "class", Type: "not-a-type"}}
	if err := ValidateBundle(unknownType); err == nil {
		t.Fatal("ValidateBundle accepted a media field with an unknown type")
	}

	duplicate := base()
	duplicate.MediaFields = []FieldSpec{
		{Name: "class", Type: "text"},
		{Name: "class", Type: "flag"},
	}
	if err := ValidateBundle(duplicate); err == nil {
		t.Fatal("ValidateBundle accepted a duplicate media field name")
	}
}

// TestValidateBundleMediaClassLabels exercises the per-class MediaLabel
// override seam: MediaClassLabels keys the conventional
// "class" Media field's declared values to a display label overriding the
// bundle-wide MediaLabel. A valid slug key with an English fallback is
// accepted; a non-slug key or a label missing the English anchor is rejected.
func TestValidateBundleMediaClassLabels(t *testing.T) {
	base := func() Bundle {
		return Bundle{
			ID:           "smoke",
			Version:      "0.1.0",
			SDKFloor:     Version,
			SDKCeiling:   Version,
			DisplayName:  L("Smoke"),
			MediaLabel:   L("Item"),
			MediaClasses: []string{"video"},
			MediaFields:  []FieldSpec{{Name: "class", Type: "text"}},
			Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
		}
	}

	valid := base()
	valid.MediaClassLabels = map[string]LocaleString{"movie": L("Movie"), "short": L("Short")}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid media class labels: %v", err)
	}

	badKey := base()
	badKey.MediaClassLabels = map[string]LocaleString{"Not A Slug": L("Movie")}
	if err := ValidateBundle(badKey); err == nil {
		t.Fatal("ValidateBundle accepted a media class label key that is not a lowercase slug")
	}

	missingFallback := base()
	missingFallback.MediaClassLabels = map[string]LocaleString{"movie": {"fr": "Film"}}
	if err := ValidateBundle(missingFallback); err == nil {
		t.Fatal("ValidateBundle accepted a media class label missing the English fallback")
	}

	badTag := base()
	badTag.MediaClassLabels = map[string]LocaleString{"movie": {"en": "Movie", "FR": "Film"}}
	if err := ValidateBundle(badTag); err == nil {
		t.Fatal("ValidateBundle accepted a media class label with an invalid BCP-47 locale tag")
	}

	blankTranslation := base()
	blankTranslation.MediaClassLabels = map[string]LocaleString{"movie": {"en": "Movie", "fr": "  "}}
	if err := ValidateBundle(blankTranslation); err == nil {
		t.Fatal("ValidateBundle accepted a media class label with a blank non-en translation")
	}
}

// TestValidateBundleLayoutArchetypes exercises the multi-Layout-per-bundle
// archetype seam (Film & TV's five detail Layouts generalized to a fixed
// shape vocabulary): a detail Layout may name one
// LayoutArchetypes entry; a bundle may declare at most one Layout per
// archetype; the archetype is invalid on a "browse" Layout; and an unknown
// archetype name is rejected.
func TestValidateBundleLayoutArchetypes(t *testing.T) {
	base := func() Bundle {
		return Bundle{
			ID:           "smoke",
			Version:      "0.1.0",
			SDKFloor:     Version,
			SDKCeiling:   Version,
			DisplayName:  L("Smoke"),
			MediaLabel:   L("Item"),
			MediaClasses: []string{"video"},
			Layouts:      []LayoutSpec{{ID: "browse", Role: "browse"}},
		}
	}

	valid := base()
	valid.Layouts = append(valid.Layouts,
		LayoutSpec{ID: "movie", Role: "detail", Archetype: "item"},
		LayoutSpec{ID: "show", Role: "detail", Archetype: "container"},
		LayoutSpec{ID: "season", Role: "detail", Archetype: "ordered-children"},
		LayoutSpec{ID: "person", Role: "detail", Archetype: "person"},
		LayoutSpec{ID: "film-series", Role: "detail", Archetype: "collection"},
	)
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected the five detail-Layout archetypes: %v", err)
	}

	unknown := base()
	unknown.Layouts = append(unknown.Layouts, LayoutSpec{ID: "movie", Role: "detail", Archetype: "not-a-real-archetype"})
	if err := ValidateBundle(unknown); err == nil {
		t.Fatal("ValidateBundle accepted an unknown layout archetype")
	}

	duplicate := base()
	duplicate.Layouts = append(duplicate.Layouts,
		LayoutSpec{ID: "movie", Role: "detail", Archetype: "item"},
		LayoutSpec{ID: "short", Role: "detail", Archetype: "item"},
	)
	if err := ValidateBundle(duplicate); err == nil {
		t.Fatal("ValidateBundle accepted a duplicate layout archetype")
	}

	onBrowse := base()
	onBrowse.Layouts[0].Archetype = "item"
	if err := ValidateBundle(onBrowse); err == nil {
		t.Fatal("ValidateBundle accepted an archetype on a browse layout")
	}
}

// TestValidateBundleScraperCandidateArtworkSlots exercises the artwork-
// candidate declaration seam ("Artwork Slots retain candidates,
// not just winners"): a CandidateArtworkSlots entry naming a slot the scraper
// maps via ArtworkMap or SearchArtworkMap is accepted; an unknown slot, an
// unmapped slot, or a duplicate is rejected.
func TestValidateBundleScraperCandidateArtworkSlots(t *testing.T) {
	valid := scraperBundle()
	valid.Scrapers[0].ArtworkMap = map[string]string{"backdrop": "backdrop_path", "thumb": "thumb_path"}
	valid.Scrapers[0].CandidateArtworkSlots = []string{"backdrop", "thumb"}
	if err := ValidateBundle(valid); err != nil {
		t.Fatalf("ValidateBundle rejected valid candidate artwork slots: %v", err)
	}

	unknownSlot := scraperBundle()
	unknownSlot.Scrapers[0].CandidateArtworkSlots = []string{"not-a-slot"}
	if err := ValidateBundle(unknownSlot); err == nil {
		t.Fatal("ValidateBundle accepted an unknown candidate artwork slot")
	}

	unmappedSlot := scraperBundle()
	unmappedSlot.Scrapers[0].ArtworkMap = map[string]string{"poster": "poster_path"}
	unmappedSlot.Scrapers[0].CandidateArtworkSlots = []string{"backdrop"}
	if err := ValidateBundle(unmappedSlot); err == nil {
		t.Fatal("ValidateBundle accepted a candidate artwork slot the scraper does not map")
	}

	duplicateSlot := scraperBundle()
	duplicateSlot.Scrapers[0].ArtworkMap = map[string]string{"backdrop": "backdrop_path"}
	duplicateSlot.Scrapers[0].CandidateArtworkSlots = []string{"backdrop", "backdrop"}
	if err := ValidateBundle(duplicateSlot); err == nil {
		t.Fatal("ValidateBundle accepted a duplicate candidate artwork slot")
	}
}
