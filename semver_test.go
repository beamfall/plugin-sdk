package sdk

import "testing"

// semverSign normalizes a compareSemver result to -1/0/1 so table cases can
// assert ordering without depending on the exact magnitudes the comparison
// arithmetic yields.
func semverSign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// TestPreRelease: preRelease strips build metadata BEFORE cutting on the
// pre-release hyphen, so metadata identifiers that themselves contain "-"
// ("1.0.0+build-2") never fabricate a phantom pre-release.
func TestPreRelease(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    string
	}{
		{"plain release", "1.0.0", ""},
		{"pre-release", "1.0.0-rc.1", "rc.1"},
		{"build metadata only", "1.0.0+build", ""},
		{"build metadata containing a hyphen", "1.0.0+build-2", ""},
		{"pre-release plus build metadata", "1.0.0-rc.1+build", "rc.1"},
		{"pre-release plus hyphenated build metadata", "1.0.0-rc.1+build-2", "rc.1"},
		{"hyphen inside the pre-release itself", "1.0.0-alpha-beta", "alpha-beta"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := preRelease(tc.version); got != tc.want {
				t.Errorf("preRelease(%q) = %q, want %q", tc.version, got, tc.want)
			}
		})
	}
}

// TestCompareSemver exercises SemVer §11 precedence directly: release triples
// numerically, build metadata ignored, pre-release below release, numeric
// pre-release fields below alphanumeric ones, numerics numerically, and a
// shorter prefix of equal fields sorting lower.
func TestCompareSemver(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want int // sign of compareSemver(a, b)
	}{
		{"equal releases", "1.0.0", "1.0.0", 0},
		{"triple compares numerically", "1.2.0", "1.10.0", -1},
		{"patch orders after minor and major", "1.0.9", "1.0.10", -1},
		{"build metadata ignored", "1.0.0+build", "1.0.0", 0},
		{"build metadata containing a hyphen ignored", "1.0.0+build-2", "1.0.0", 0},
		{"hyphenated build metadata on both sides", "1.0.0+build-2", "1.0.0+other-9", 0},
		{"pre-release sorts below release", "1.0.0-rc.1", "1.0.0", -1},
		{"release sorts above pre-release", "1.0.0", "1.0.0-rc.1", 1},
		{"numeric pre-release field ranks below alphanumeric", "1.0.0-1", "1.0.0-alpha", -1},
		{"numeric pre-release fields compare numerically", "1.0.0-rc.9", "1.0.0-rc.10", -1},
		{"alphanumeric pre-release fields compare in ASCII order", "1.0.0-alpha", "1.0.0-beta", -1},
		{"shorter prefix of equal fields sorts lower", "1.0.0-rc", "1.0.0-rc.1", -1},
		{"equal pre-releases", "1.0.0-rc.1", "1.0.0-rc.1", 0},
		{"triple outranks pre-release presence", "1.0.1-rc.1", "1.0.0", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := semverSign(compareSemver(tc.a, tc.b)); got != tc.want {
				t.Errorf("compareSemver(%q, %q) sign = %d, want %d", tc.a, tc.b, got, tc.want)
			}
			if got := semverSign(compareSemver(tc.b, tc.a)); got != -tc.want {
				t.Errorf("compareSemver(%q, %q) sign = %d, want %d (antisymmetry)", tc.b, tc.a, got, -tc.want)
			}
		})
	}
}

// TestCompatibleWithSDKSemverPrecedence: CompatibleWithSDK end-to-end over the
// compareSemver/preRelease seams — build metadata (hyphenated or not) never
// moves a version relative to its bounds, pre-release precedence orders
// numerically, and a malformed sdkVersion is incompatible, never a panic.
func TestCompatibleWithSDKSemverPrecedence(t *testing.T) {
	base := Bundle{
		ID:           "smoke",
		Version:      "0.1.0",
		SDKFloor:     "1.0.0",
		SDKCeiling:   "1.0.0",
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
		{"hyphenated build metadata satisfies an equal floor", "1.0.0", "1.0.1", "1.0.0+build-2", true},
		{"hyphenated build metadata satisfies an equal ceiling", "1.0.0-rc.1", "1.0.0", "1.0.0+build-2", true},
		{"plain build metadata satisfies an equal floor", "1.0.0", "1.0.1", "1.0.0+build", true},
		{"numeric pre-release fields compare numerically against a floor", "1.0.0-rc.10", "1.0.0", "1.0.0-rc.9", false},
		{"equal numeric pre-release meets its floor", "1.0.0-rc.10", "1.0.0", "1.0.0-rc.10", true},
		{"shorter pre-release prefix misses the longer floor", "1.0.0-rc.1", "1.0.0", "1.0.0-rc", false},
		{"malformed sdk version is incompatible", "1.0.0", "2.0.0", "not-semver", false},
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
