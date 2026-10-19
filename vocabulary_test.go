package sdk

import "testing"

func TestVocabularyAccessorsReturnCopies(t *testing.T) {
	permissions := PermissionKinds()
	permissions[0] = "mutated"
	permissions = append(permissions, "extra")

	got := PermissionKinds()
	if got[0] == "mutated" {
		t.Fatal("mutating returned PermissionKinds slice changed SDK vocabulary")
	}
	if len(got) == len(permissions) {
		t.Fatal("appending returned PermissionKinds slice changed SDK vocabulary length")
	}

	policy, ok := PermissionPolicyFor("request-file-mutation")
	if !ok {
		t.Fatal("PermissionPolicyFor rejected a known permission")
	}
	policy.Label["en"] = "mutated"
	policy.Description["en"] = "mutated"
	nextPolicy, _ := PermissionPolicyFor("request-file-mutation")
	if nextPolicy.Label["en"] == "mutated" || nextPolicy.Description["en"] == "mutated" {
		t.Fatal("mutating returned PermissionPolicy changed SDK policy")
	}

	valued := ValuedSourceKinds()
	valued["resume-progress"][0] = "mutated"
	valued["extra"] = []string{"value"}

	next := ValuedSourceKinds()
	if next["resume-progress"][0] == "mutated" {
		t.Fatal("mutating returned ValuedSourceKinds nested slice changed SDK vocabulary")
	}
	if _, ok := next["extra"]; ok {
		t.Fatal("mutating returned ValuedSourceKinds map changed SDK vocabulary")
	}
}

func TestPermissionPolicyCoversClosedVocabulary(t *testing.T) {
	highRisk := map[string]bool{
		"read-catalog":          true,
		"profile-link":          true,
		"request-ingest":        true,
		"request-file-mutation": true,
	}
	if _, ok := PermissionPolicyFor("not-a-permission"); ok {
		t.Fatal("PermissionPolicyFor accepted an unknown permission")
	}
	if len(permissionPolicies) != len(permissionKinds) {
		t.Fatalf("permission policy count = %d, permission kind count = %d", len(permissionPolicies), len(permissionKinds))
	}
	for _, kind := range permissionKinds {
		policy, ok := PermissionPolicyFor(kind)
		if !ok {
			t.Errorf("missing permission policy for %q", kind)
			continue
		}
		if policy.Label.Resolve("") == "" {
			t.Errorf("permission policy %q has no label", kind)
		}
		if policy.Description.Resolve("") == "" {
			t.Errorf("permission policy %q has no description", kind)
		}
		if policy.HighRisk != highRisk[kind] {
			t.Errorf("permission policy %q HighRisk = %v, want %v", kind, policy.HighRisk, highRisk[kind])
		}
	}
}
