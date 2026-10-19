package compatibility

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

const wantSDKVersion = "0.3.2"

func matrixBundle(entry Entry) sdk.Bundle {
	return sdk.Bundle{
		ID:           entry.ConsumerID,
		Version:      entry.ConsumerVersion,
		SDKFloor:     entry.SDKFloor,
		SDKCeiling:   entry.SDKCeiling,
		DisplayName:  sdk.L("Matrix"),
		MediaLabel:   sdk.L("Item"),
		MediaClasses: []string{"video"},
		Layouts:      []sdk.LayoutSpec{{ID: "browse", Role: "browse"}},
	}
}

func TestCanonicalMatrixIsExactAndCurrent(t *testing.T) {
	wantIDs := []sdk.ID{"adult", "audiobook", "familyarchive", "filmtv", "music", "photos", "podcasts"}
	got := Entries()
	if len(got) != len(wantIDs) {
		t.Fatalf("matrix entries = %d, want %d", len(got), len(wantIDs))
	}
	seen := map[sdk.ID]bool{}
	revisionPattern := regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern := regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	var adult sdk.Bundle
	for _, entry := range got {
		if entry.SDKVersion != wantSDKVersion {
			t.Errorf("%s matrix SDK = %s, want consumer-pinned %s", entry.ConsumerID, entry.SDKVersion, wantSDKVersion)
		}
		if seen[entry.ConsumerID] {
			t.Errorf("duplicate matrix consumer %s", entry.ConsumerID)
		}
		if !revisionPattern.MatchString(entry.ConsumerRevision) {
			t.Errorf("%s consumer revision = %q, want full Git commit", entry.ConsumerID, entry.ConsumerRevision)
		}
		if !digestPattern.MatchString(entry.ConsumerDigest) {
			t.Errorf("%s consumer digest = %q, want sha256 digest", entry.ConsumerID, entry.ConsumerDigest)
		}
		seen[entry.ConsumerID] = true
		if err := sdk.ValidateBundle(matrixBundle(entry)); err != nil {
			t.Errorf("%s matrix metadata: %v", entry.ConsumerID, err)
		}
		if !sdk.CompatibleWithSDK(matrixBundle(entry), entry.SDKFloor) {
			t.Errorf("%s matrix floor %s is not admissible by its declared range", entry.ConsumerID, entry.SDKFloor)
		}
		if err := Verify(matrixBundle(entry), entry.SDKVersion); err != nil {
			t.Errorf("%s matrix verification: %v", entry.ConsumerID, err)
		}
		if entry.ConsumerID == "adult" {
			adult = matrixBundle(entry)
		}
	}
	if adult.ID != "adult" || adult.SDKFloor != "0.1.6" {
		t.Errorf("adult matrix identity/floor = %s/%s, want adult/0.1.6", adult.ID, adult.SDKFloor)
	}
	if err := Verify(adult, wantSDKVersion); err != nil {
		t.Errorf("adult historical floor %s does not verify against recorded SDK %s: %v", adult.SDKFloor, wantSDKVersion, err)
	}
	for _, id := range wantIDs {
		if !seen[id] {
			t.Errorf("canonical consumer %s is unregistered", id)
		}
	}
}

func TestCanonicalMatrixMatchesRecordedConsumerHistory(t *testing.T) {
	workspace := consumerWorkspace(t)
	for _, entry := range Entries() {
		entry := entry
		t.Run(string(entry.ConsumerID), func(t *testing.T) {
			repo := filepath.Join(workspace, "beamfall-plugin-"+string(entry.ConsumerID))
			if _, err := os.Stat(filepath.Join(repo, ".git")); err != nil {
				t.Skipf("consumer workspace unavailable: %v", err)
			}
			// Relaxed cycle-breaker: only requires the commit object to exist in the
			// consumer repository (not that it is an ancestor of main), because a matrix
			// row must be able to name an unmerged plugin branch tip before that branch lands.
			// A stricter "is-ancestor-of-main" check can return once the plugins land.
			command := exec.Command("git", "-C", repo, "cat-file", "-e", entry.ConsumerRevision+"^{commit}")
			if err := command.Run(); err != nil {
				t.Fatalf("recorded revision %s does not resolve in consumer repository: %v", entry.ConsumerRevision, err)
			}
			mainHead := gitOutput(t, repo, "rev-parse", "main")
			if mainHead != entry.ConsumerRevision {
				t.Logf("consumer main has advanced to %s; matrix records ancestor %s", mainHead, entry.ConsumerRevision)
			}
			bundleSource := gitOutput(t, repo, "show", entry.ConsumerRevision+":bundle.go")
			assertBundleRange(t, entry, bundleSource)
			archive := gitBytes(t, repo, "archive", "--format=tar", entry.ConsumerRevision)
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(archive))
			if digest != entry.ConsumerDigest {
				t.Errorf("archive digest = %s, matrix records %s", digest, entry.ConsumerDigest)
			}
		})
	}
}

func consumerWorkspace(t *testing.T) string {
	t.Helper()
	commonDir := gitOutput(t, ".", "rev-parse", "--git-common-dir")
	if !filepath.IsAbs(commonDir) {
		absolute, err := filepath.Abs(commonDir)
		if err != nil {
			t.Fatal(err)
		}
		commonDir = absolute
	}
	return filepath.Dir(filepath.Dir(commonDir))
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(string(gitBytes(t, repo, args...)))
}

func gitBytes(t *testing.T, repo string, args ...string) []byte {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := command.Output()
	if err != nil {
		t.Fatalf("git -C %s %s: %v", repo, strings.Join(args, " "), err)
	}
	return out
}

func assertBundleRange(t *testing.T, entry Entry, source string) {
	t.Helper()
	for field, want := range map[string]string{"SDKFloor": entry.SDKFloor, "SDKCeiling": entry.SDKCeiling} {
		pattern := regexp.MustCompile(`(?m)^\s*` + field + `:\s*"([^"]+)",\s*$`)
		match := pattern.FindStringSubmatch(source)
		if len(match) != 2 {
			t.Fatalf("%s bundle.go has no unambiguous %s declaration", entry.ConsumerID, field)
		}
		if match[1] != want {
			t.Errorf("%s bundle.go %s = %q, matrix records %q", entry.ConsumerID, field, match[1], want)
		}
	}
}

func TestVerifyRejectsUnprovedCompatibility(t *testing.T) {
	base := matrixBundle(Entries()[0])
	tests := []struct {
		name string
		edit func(*sdk.Bundle) string
		want string
	}{
		{"unregistered consumer", func(bundle *sdk.Bundle) string {
			bundle.ID = "unknown"
			return "0.2.0"
		}, "no producer compatibility evidence"},
		{"unregistered consumer version", func(bundle *sdk.Bundle) string {
			bundle.Version = "0.1.1"
			return "0.2.0"
		}, "no producer compatibility evidence"},
		{"unregistered SDK version", func(_ *sdk.Bundle) string {
			return "0.1.5"
		}, "does not support"},
		{"new SDK major", func(bundle *sdk.Bundle) string {
			bundle.SDKCeiling = "1.0.0"
			return "1.0.0"
		}, "does not support"},
		{"unproved range", func(bundle *sdk.Bundle) string {
			bundle.SDKFloor = "0.1.9"
			return wantSDKVersion
		}, "lacks exact producer evidence"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle := base
			sdkVersion := test.edit(&bundle)
			err := Verify(bundle, sdkVersion)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Verify error = %v, want %q", err, test.want)
			}
		})
	}
}
