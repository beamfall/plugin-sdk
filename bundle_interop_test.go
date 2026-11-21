package sdk

import "testing"

func interopBundle() Bundle {
	b := validBundle()
	b.MediaFields = []FieldSpec{
		{Name: "class", Type: "text", Label: L("Class")},
		{Name: "rank", Type: "number", Label: L("Rank")},
	}
	b.EntityKinds = []EntityKind{
		{Name: "outer", DisplayName: L("Outer"), Archetype: "container"},
		{Name: "inner", DisplayName: L("Inner"), Archetype: "container"},
		{Name: "tag", DisplayName: L("Tag"), Archetype: "category"},
	}
	b.FamilyScopes = []FamilyScopeSpec{
		{Value: "first", DisplayName: L("First")},
		{Value: "both", DisplayName: L("Both"), Default: true},
	}
	b.Interop = &InteropSpec{
		MediaField: "class",
		Media:      []InteropMapping{{Value: "solo", Type: "Alpha"}, {Value: "part", Type: "Delta"}},
		Kinds:      []InteropMapping{{Value: "outer", Type: "Beta"}, {Value: "inner", Type: "Gamma"}},
		Scopes:     []InteropMapping{{Value: "first", Type: "alphas"}},
	}
	return b
}

func TestValidateBundleInterop(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		if err := ValidateBundle(interopBundle()); err != nil {
			t.Fatalf("ValidateBundle rejected a valid interop declaration: %v", err)
		}
	})
	t.Run("absent", func(t *testing.T) {
		b := interopBundle()
		b.Interop = nil
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle rejected a bundle without interop: %v", err)
		}
	})
	t.Run("media values may share a type", func(t *testing.T) {
		b := interopBundle()
		b.Interop.Media[1].Type = "Alpha"
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle rejected shared media item type: %v", err)
		}
	})
	t.Run("kinds only", func(t *testing.T) {
		b := interopBundle()
		b.Interop.MediaField, b.Interop.Media = "", nil
		if err := ValidateBundle(b); err != nil {
			t.Fatalf("ValidateBundle rejected kind-only interop: %v", err)
		}
	})

	cases := []struct {
		name   string
		mutate func(*InteropSpec)
		want   string
	}{
		{"empty", func(s *InteropSpec) { *s = InteropSpec{} }, `interop declares no mappings`},
		{"field without media", func(s *InteropSpec) { s.Media = nil }, `interop media field and media mappings must be declared together`},
		{"media without field", func(s *InteropSpec) { s.MediaField = "" }, `interop media field and media mappings must be declared together`},
		{"undeclared field", func(s *InteropSpec) { s.MediaField = "missing" }, `interop media field "missing" is not declared`},
		{"non-text field", func(s *InteropSpec) { s.MediaField = "rank" }, `interop media field "rank" has type "number", want text`},
		{"blank media value", func(s *InteropSpec) { s.Media[0].Value = " " }, `interop media values must be non-blank`},
		{"duplicate media value", func(s *InteropSpec) { s.Media[1].Value = "solo" }, `duplicate interop media value "solo"`},
		{"bad token", func(s *InteropSpec) { s.Media[0].Type = "Al pha" }, `interop media "solo" token "Al pha" must be 1-32 ASCII letters`},
		{"empty token", func(s *InteropSpec) { s.Kinds[0].Type = "" }, `interop kind "outer" token "" must be 1-32 ASCII letters`},
		{"long token", func(s *InteropSpec) { s.Scopes[0].Type = "abcdefghijklmnopqrstuvwxyzabcdefg" }, `interop scope "first" token "abcdefghijklmnopqrstuvwxyzabcdefg" must be 1-32 ASCII letters`},
		{"undeclared kind", func(s *InteropSpec) { s.Kinds[0].Value = "missing" }, `interop kind "missing" is not a declared entity kind`},
		{"non-container kind", func(s *InteropSpec) { s.Kinds[0].Value = "tag" }, `interop kind "tag" is not a container kind`},
		{"duplicate kind", func(s *InteropSpec) { s.Kinds[1].Value = "outer" }, `duplicate interop kind value "outer"`},
		{"kinds share a type", func(s *InteropSpec) { s.Kinds[1].Type = "Beta" }, `interop kinds "outer" and "inner" share item type "Beta"`},
		{"media and kind share a type", func(s *InteropSpec) { s.Kinds[0].Type = "Alpha" }, `interop item type "Alpha" names both media and kind "outer"`},
		{"undeclared scope", func(s *InteropSpec) { s.Scopes[0].Value = "missing" }, `interop scope "missing" is not a declared family scope`},
		{"duplicate scope", func(s *InteropSpec) { s.Scopes = append(s.Scopes, InteropMapping{Value: "first", Type: "betas"}) }, `duplicate interop scope value "first"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := interopBundle()
			tc.mutate(b.Interop)
			wantBundleError(t, b, tc.want)
		})
	}
}
