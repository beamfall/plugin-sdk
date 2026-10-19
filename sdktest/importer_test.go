package sdktest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// fakeSource is a conformant importer for the smoke test: it yields a fixed,
// deterministic item slice, honors cancellation, and never emits partial state.
type fakeSource struct {
	categories []string
	items      []sdk.ForeignItem
	starts     *[]string
}

func (s fakeSource) Categories() []string { return s.categories }

func (s fakeSource) Items(ctx context.Context) ([]sdk.ForeignItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]sdk.ForeignItem(nil), s.items...), nil
}

func (s fakeSource) Stream(
	ctx context.Context,
	checkpoint string,
	emit func(context.Context, []sdk.ForeignItem, string) error,
) error {
	if s.starts != nil {
		*s.starts = append(*s.starts, checkpoint)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	start := 0
	if checkpoint != "" {
		var err error
		start, err = strconv.Atoi(checkpoint)
		if err != nil || start < 0 || start > len(s.items) {
			return fmt.Errorf("invalid fake checkpoint %q", checkpoint)
		}
	}
	for start < len(s.items) {
		end := min(start+sdk.MaxImportPageItems, len(s.items))
		if err := emit(ctx, s.items[start:end], strconv.Itoa(end)); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func conformantForeignItems() []sdk.ForeignItem {
	return []sdk.ForeignItem{
		{Path: "/media/a.mkv", WatchState: sdk.WatchState{Present: true, PositionSeconds: 120, Watched: false}},
		{Path: "/media/b.mkv", Rating: sdk.Rating{Present: true, Value: 4, Scale: 5}},
	}
}

// TestRunImporterConformanceGreenOnConformantSource is the importer smoke check:
// a deterministic, cancellation-honoring, atomic Source passes every dimension.
func TestRunImporterConformanceGreenOnConformantSource(t *testing.T) {
	RunImporterConformance(t, []ImporterVector{{
		Name:           "watch+ratings",
		Source:         fakeSource{categories: []string{"watch-state", "ratings"}, items: conformantForeignItems()},
		WantCategories: []string{"ratings", "watch-state"},
		WantItems:      conformantForeignItems(),
	}})
}

func TestRunImporterConformanceGreenOnEmptyStream(t *testing.T) {
	RunImporterConformance(t, []ImporterVector{{
		Name:           "empty",
		Source:         fakeSource{categories: []string{sdk.CategoryWatchState}, items: []sdk.ForeignItem{}},
		WantCategories: []string{sdk.CategoryWatchState},
		WantItems:      []sdk.ForeignItem{},
	}})
}

type legacyOnlySource struct{}

func (legacyOnlySource) Categories() []string { return []string{sdk.CategoryWatchState} }
func (legacyOnlySource) Items(context.Context) ([]sdk.ForeignItem, error) {
	return nil, nil
}

func TestRunImporterConformanceRejectsLegacyOnlySource(t *testing.T) {
	const helperEnv = "BEAMFALL_TEST_LEGACY_ONLY_SOURCE"
	if os.Getenv(helperEnv) == "1" {
		RunImporterConformance(t, []ImporterVector{{
			Name:           "legacy-only",
			Source:         legacyOnlySource{},
			WantCategories: []string{sdk.CategoryWatchState},
		}})
		return
	}

	args := []string{"-test.run=^TestRunImporterConformanceRejectsLegacyOnlySource$"}
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("RunImporterConformance accepted a Source without Stream; output:\n%s", output)
	}
	if !strings.Contains(string(output), "Source does not implement sdk.StreamingSource") {
		t.Fatalf("failure output did not identify the missing streaming contract:\n%s", output)
	}
}

func TestRunImporterConformanceExercisesMultiPageResume(t *testing.T) {
	items := make([]sdk.ForeignItem, sdk.MaxImportPageItems*2+1)
	for i := range items {
		items[i].Path = fmt.Sprintf("/media/%06d.mkv", i)
	}
	starts := []string{}
	RunImporterConformance(t, []ImporterVector{{
		Name:           "multi-page",
		Source:         fakeSource{categories: []string{sdk.CategoryWatchState}, items: items, starts: &starts},
		WantCategories: []string{sdk.CategoryWatchState},
		WantItems:      items,
	}})
	laterCheckpoint := strconv.Itoa(sdk.MaxImportPageItems * 2)
	for _, checkpoint := range starts {
		if checkpoint == laterCheckpoint {
			return
		}
	}
	t.Fatalf("conformance Stream starts = %v, want later checkpoint %q", starts, laterCheckpoint)
}

// erroringSource aborts atomically: it returns an error and no partial items.
type erroringSource struct{ categories []string }

func (s erroringSource) Categories() []string { return s.categories }
func (s erroringSource) Items(context.Context) ([]sdk.ForeignItem, error) {
	return nil, errors.New("source unavailable")
}
func (s erroringSource) Stream(
	context.Context,
	string,
	func(context.Context, []sdk.ForeignItem, string) error,
) error {
	return errors.New("source unavailable")
}

// TestRunImporterConformanceGreenOnErrorVector: an atomic failure vector passes.
func TestRunImporterConformanceGreenOnErrorVector(t *testing.T) {
	RunImporterConformance(t, []ImporterVector{{
		Name:           "unreachable",
		Source:         erroringSource{categories: []string{"subscriptions"}},
		WantCategories: []string{"subscriptions"},
		WantErr:        true,
	}})
}

// TestImporterCategoryViolations covers the category dimension diagnostically:
// unrecognized vocabulary, missing expected, unexpected extra, and empty want.
func TestImporterCategoryViolations(t *testing.T) {
	cases := []struct {
		name    string
		got     []string
		want    []string
		wantSub string
	}{
		{"unrecognized", []string{"bogus"}, []string{"bogus"}, `"bogus" is not a recognized`},
		{"missing expected", []string{"ratings"}, []string{"ratings", "watch-state"}, `missing expected category "watch-state"`},
		{"unexpected extra", []string{"ratings", "collections"}, []string{"ratings"}, `unexpected category "collections"`},
		{"empty want", []string{"ratings"}, nil, "no WantCategories"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reasons := importerCategoryViolations(tc.got, tc.want)
			if !containsSubstring(reasons, tc.wantSub) {
				t.Fatalf("violations %v do not name %q", reasons, tc.wantSub)
			}
		})
	}
}

// TestImporterNondeterminismViolation: two runs that differ red under the
// repeatability dimension with a message showing the first differing item.
func TestImporterNondeterminismViolation(t *testing.T) {
	run1 := conformantForeignItems()
	run2 := conformantForeignItems()
	run2[0].WatchState.PositionSeconds = 999
	reason := foreignItemsViolation("repeatability (run 1 vs run 2)", run1, run2)
	if !strings.Contains(reason, "repeatability") || !strings.Contains(reason, "item 0") {
		t.Fatalf("nondeterminism diagnostic = %q, want repeatability + item index", reason)
	}
}

// TestImporterCountMismatchViolation: a differing item count is reported as such.
func TestImporterCountMismatchViolation(t *testing.T) {
	reason := foreignItemsViolation("emitted items", conformantForeignItems(), conformantForeignItems()[:1])
	if !strings.Contains(reason, "item count 1, want 2") {
		t.Fatalf("count diagnostic = %q", reason)
	}
}

func TestImporterEmptyNilAndAllocatedSlicesAreEquivalent(t *testing.T) {
	if reason := foreignItemsViolation("empty import", []sdk.ForeignItem{}, nil); reason != "" {
		t.Fatalf("empty import diagnostic = %q, want none", reason)
	}
}

// TestImporterAtomicityViolation: emitting partial items alongside an error, or
// returning no error at all on an abort vector, both red the atomicity dimension.
func TestImporterAtomicityViolation(t *testing.T) {
	partial := atomicityViolation(errors.New("boom"), conformantForeignItems())
	if !strings.Contains(partial, "failure atomicity") || !strings.Contains(partial, "partial item") {
		t.Fatalf("partial-emission diagnostic = %q", partial)
	}
	noErr := atomicityViolation(nil, nil)
	if !strings.Contains(noErr, "expected to abort") {
		t.Fatalf("no-error diagnostic = %q", noErr)
	}
}

// TestImporterCancellationViolation: a Source that ignores a cancelled context —
// returning items or a nil error — reds the cancellation dimension.
func TestImporterCancellationViolation(t *testing.T) {
	ignoredNilErr := cancellationViolation(nil, nil)
	if !strings.Contains(ignoredNilErr, "cancellation") || !strings.Contains(ignoredNilErr, "honor cancellation") {
		t.Fatalf("cancellation nil-error diagnostic = %q", ignoredNilErr)
	}
	emittedItems := cancellationViolation(context.Canceled, conformantForeignItems())
	if !strings.Contains(emittedItems, "cancelled run must emit none") {
		t.Fatalf("cancellation emitted-items diagnostic = %q", emittedItems)
	}
	wrongErr := cancellationViolation(errors.New("stopped"), nil)
	if !strings.Contains(wrongErr, "want context.Canceled") {
		t.Fatalf("cancellation wrong-error diagnostic = %q", wrongErr)
	}
}

// TestImporterGateRejectsDirectNetworkImporter: an importer whose own code
// imports a network package reds the static import gate — before Items is ever
// called, and regardless of whether Items would swallow its network error. The
// gate is an AST check over the module, so runtime error handling cannot launder
// the capability past it.
func TestImporterGateRejectsDirectNetworkImporter(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"importer.go": `package plugin

import (
	"context"
	"net/http"
)

type Importer struct{}

func (Importer) Categories() []string { return []string{"watch-state"} }

// Items would reach the network but swallows the error — the gate reds anyway.
func (Importer) Items(ctx context.Context) ([]struct{}, error) {
	_, _ = http.Get("https://predecessor.example.test/library")
	return nil, nil
}
`,
	})
	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), `"net/http" is allowed only in registered scrobble.Adapter seam files`) {
		t.Fatalf("checkAllowedImports() = %v, want direct-network importer rejection", err)
	}
}

// TestImporterGateRejectsTransitiveNetworkImporter: the network capability is
// reached through a local helper package (net/rpc), not the importer file
// itself. The whole-module walk plus the broadened net/* family catch it — the
// transitive-reach guarantee.
func TestImporterGateRejectsTransitiveNetworkImporter(t *testing.T) {
	root := writeAllowedImportFixture(t, map[string]string{
		"go.mod": "module example.com/plugin\n\ngo 1.26.4\n",
		"importer.go": `package plugin

import (
	"context"

	"example.com/plugin/netutil"
)

type Importer struct{}

func (Importer) Categories() []string { return []string{"watch-state"} }

func (Importer) Items(ctx context.Context) ([]struct{}, error) {
	_ = netutil.Fetch
	return nil, nil
}
`,
		"netutil/client.go": `package netutil

import "net/rpc"

// Fetch reaches a predecessor server over RPC — a network capability the
// importer pulls in transitively.
func Fetch(addr string) (*rpc.Client, error) { return rpc.Dial("tcp", addr) }
`,
	})
	err := checkAllowedImports(root)
	if err == nil || !strings.Contains(err.Error(), `"net/rpc" is a network-capable package`) {
		t.Fatalf("checkAllowedImports() = %v, want transitive-network importer rejection", err)
	}
}
