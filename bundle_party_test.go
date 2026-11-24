package sdk

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func partyBundle() Bundle {
	return Bundle{
		ID:                 "smoke",
		Version:            "0.1.0",
		SDKFloor:           Version,
		SDKCeiling:         Version,
		DisplayName:        L("Smoke"),
		MediaLabel:         L("Item"),
		MediaClasses:       []string{"audio"},
		FeatureEligibility: []string{FeatureParty},
		Layouts:            []LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

func TestFeaturePartyInVocabulary(t *testing.T) {
	if !slices.Contains(FeatureEligibilityCapabilities(), FeatureParty) {
		t.Fatalf("FeatureEligibilityCapabilities() = %v, want it to include %q", FeatureEligibilityCapabilities(), FeatureParty)
	}
	b := partyBundle()
	if err := ValidateBundle(b); err != nil {
		t.Fatalf("ValidateBundle rejected party eligibility without terms: %v", err)
	}
}

func TestValidateBundlePartyTerms(t *testing.T) {
	full := &PartyTerms{
		Party:        LocaleString{"en": "Karaoke party", "fr": "Soirée karaoké"},
		Participant:  L("Singer"),
		Participants: L("Singers"),
		Request:      L("Song"),
		Requests:     L("Songs"),
		Stage:        L("Stage"),
	}
	cases := []struct {
		name     string
		features []string
		terms    *PartyTerms
		wantErr  string
	}{
		{name: "full terms", features: []string{FeatureParty}, terms: full},
		{name: "one term", features: []string{FeatureParty}, terms: &PartyTerms{Participant: L("Singer")}},
		{name: "empty terms", features: []string{FeatureParty}, terms: &PartyTerms{}},
		{name: "terms without feature", features: nil, terms: full, wantErr: "without feature eligibility"},
		{name: "terms with another feature only", features: []string{FeatureAudioStation}, terms: full, wantErr: "without feature eligibility"},
		{name: "participants without participant", features: []string{FeatureParty}, terms: &PartyTerms{Participants: L("Singers")}, wantErr: "participants is declared without participant"},
		{name: "requests without request", features: []string{FeatureParty}, terms: &PartyTerms{Requests: L("Songs")}, wantErr: "requests is declared without request"},
		{name: "no english", features: []string{FeatureParty}, terms: &PartyTerms{Stage: LocaleString{"fr": "Scène"}}, wantErr: "stage:"},
		{name: "bad tag", features: []string{FeatureParty}, terms: &PartyTerms{Party: LocaleString{"en": "Party", "FR": "Fête"}}, wantErr: "party:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := partyBundle()
			b.FeatureEligibility = tc.features
			b.PartyTerms = tc.terms
			err := ValidateBundle(b)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateBundle rejected valid party terms: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateBundle error = %v, want one containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestPartyTermsJSON(t *testing.T) {
	b := partyBundle()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "PartyTerms") {
		t.Fatalf("a bundle without party terms serialized them: %s", raw)
	}

	b.PartyTerms = &PartyTerms{Participant: LocaleString{"en": "Singer", "fr": "Chanteur"}, Stage: L("Stage")}
	raw, err = json.Marshal(b.PartyTerms)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"Participant":{"en":"Singer","fr":"Chanteur"},"Stage":"Stage"}`
	if string(raw) != want {
		t.Fatalf("PartyTerms JSON = %s, want %s", raw, want)
	}
	var back PartyTerms
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.Participant.Resolve("fr") != "Chanteur" || back.Stage.Resolve("") != "Stage" || back.Party != nil {
		t.Fatalf("round trip = %+v", back)
	}
}
