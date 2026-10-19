package sdk

import (
	"testing"

	"github.com/beamfall/plugin-sdk/scrobble"
)

func TestValidateBundleScrobbleTarget(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		b := validBundle()
		b.ScrobbleTarget = &ScrobbleTargetSpec{Default: scrobble.TargetStandalone}
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle rejected known default scrobble target: %v", err)
		}
	})

	t.Run("mapped", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			MediaField: "class",
			Mappings: []ScrobbleTargetMapping{
				{Value: "first", Hint: scrobble.TargetStandalone},
				{Value: "second", Hint: scrobble.TargetIndexed},
			},
		}
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle rejected mapped scrobble targets: %v", err)
		}
	})

	t.Run("undeclared field", func(t *testing.T) {
		b := validBundle()
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			MediaField: "class",
			Mappings:   []ScrobbleTargetMapping{{Value: "first", Hint: scrobble.TargetStandalone}},
		}
		wantBundleError(t, b, `scrobble target media field "class" is not declared`)
	})

	t.Run("non-text field", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "number", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			MediaField: "class",
			Mappings:   []ScrobbleTargetMapping{{Value: "1", Hint: scrobble.TargetStandalone}},
		}
		wantBundleError(t, b, `scrobble target media field "class" has type "number", want text`)
	})

	t.Run("absent map", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{MediaField: "class"}
		wantBundleError(t, b, `scrobble target media field "class" requires at least one mapping`)
	})

	t.Run("duplicate", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			MediaField: "class",
			Mappings: []ScrobbleTargetMapping{
				{Value: "first", Hint: scrobble.TargetStandalone},
				{Value: "first", Hint: scrobble.TargetIndexed},
			},
		}
		wantBundleError(t, b, `duplicate scrobble target mapping value "first"`)
	})

	t.Run("unknown default hint", func(t *testing.T) {
		b := validBundle()
		b.ScrobbleTarget = &ScrobbleTargetSpec{Default: scrobble.TargetHint("future")}
		wantBundleError(t, b, `scrobble target default hint "future" is unknown`)
	})

	t.Run("unknown mapped hint", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			MediaField: "class",
			Mappings:   []ScrobbleTargetMapping{{Value: "first", Hint: scrobble.TargetHint("future")}},
		}
		wantBundleError(t, b, `scrobble target mapping value "first" uses unknown hint "future"`)
	})

	t.Run("mixed modes", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			Default:    scrobble.TargetStandalone,
			MediaField: "class",
			Mappings:   []ScrobbleTargetMapping{{Value: "first", Hint: scrobble.TargetIndexed}},
		}
		wantBundleError(t, b, "scrobble target must declare exactly one of default or media field mappings")
	})

	t.Run("present but empty", func(t *testing.T) {
		b := validBundle()
		b.ScrobbleTarget = &ScrobbleTargetSpec{}
		wantBundleError(t, b, "scrobble target must declare exactly one of default or media field mappings")
	})

	t.Run("blank mapping value", func(t *testing.T) {
		b := validBundle()
		b.MediaFields = []FieldSpec{{Name: "class", Type: "text", Label: L("Class")}}
		b.ScrobbleTarget = &ScrobbleTargetSpec{
			MediaField: "class",
			Mappings:   []ScrobbleTargetMapping{{Value: " ", Hint: scrobble.TargetStandalone}},
		}
		wantBundleError(t, b, "scrobble target mapping values must be non-blank")
	})
}
