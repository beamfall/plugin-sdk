package sdktest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
)

// ImporterVector is one golden Migration Wizard importer case:
// a Source under test plus the categories and foreign items one run must stream.
type ImporterVector struct {
	Name string
	// Source is the plugin's compiled importer for this run.
	Source sdk.Source //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	// WantCategories is the exact set of data categories the run carries, from
	// the fixed ImportCategories vocabulary — order-insensitive. Every entry must
	// be a recognized category; an importer that carries nothing is meaningless.
	WantCategories []string
	// WantItems is the foreign-item slice a successful run yields, compared with
	// reflect.DeepEqual. Ignored when WantErr is set.
	WantItems []sdk.ForeignItem
	// WantErr asserts both Stream and the deprecated Items compatibility path
	// fail before emitting items. Repeatability, resume, and cancellation are
	// not graded on an error vector.
	WantErr bool
}

// RunImporterConformance grades a plugin's bounded StreamingSource and its
// deprecated Source.Items compatibility path against golden vectors.
//
// RequireAllowedImports runs FIRST, before any source method is called. Because
// the import gate is a static, whole-module AST check, an importer whose code is
// capable of outbound network — directly or through a local helper package —
// reds the run here, before execution, no matter what Stream does at runtime (an
// importer cannot launder a network capability past the gate by swallowing its
// own transport error, because no request is ever reached).
//
// Per-vector the runner asserts, in order:
//
//   - Categories() is the declared WantCategories set, and every entry is a
//     recognized ImportCategories value;
//   - Stream emits WantItems in pages no larger than sdk.MaxImportPageItems;
//   - a second Stream produces identical items and checkpoints;
//   - resuming after the first and a later committed page produces no duplicate/lost items;
//   - Stream(cancelledCtx) returns an error and emits no items;
//   - deprecated Items returns the same result during the compatibility window;
//   - on a WantErr vector, both APIs fail before emitting any item.
func RunImporterConformance(t *testing.T, vectors []ImporterVector) {
	t.Helper()
	RequireAllowedImports(t)
	if len(vectors) == 0 {
		t.Fatal("sdktest.RunImporterConformance: no importer vectors supplied")
	}
	for _, v := range vectors {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			runImporterVector(t, v)
		})
	}
}

func runImporterVector(t *testing.T, v ImporterVector) {
	t.Helper()
	for _, reason := range importerCategoryViolations(v.Source.Categories(), v.WantCategories) {
		t.Error(reason)
	}
	streaming, ok := v.Source.(sdk.StreamingSource)
	if !ok {
		t.Error("streaming: Source does not implement sdk.StreamingSource")
		return
	}
	if v.WantErr {
		gradeImporterErrorVector(t, v.Source)
		return
	}
	gradeImporterSuccessVector(t, v.Source, v.WantItems)
	gradeImporterCancellation(t, streaming)
}

type importerRun struct {
	items       []sdk.ForeignItem
	pageEnds    []int
	checkpoints []string
}

// gradeImporterSuccessVector asserts streaming output, deterministic
// checkpoints, exact resume, and the deprecated Items compatibility path.
func gradeImporterSuccessVector(
	t *testing.T,
	src sdk.Source, //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	want []sdk.ForeignItem,
) {
	t.Helper()
	first, err := collectImporterRun(context.Background(), src, "")
	if err != nil {
		t.Fatalf("Stream: unexpected error on a success vector: %v", err)
	}
	if reason := foreignItemsViolation("streamed items", want, first.items); reason != "" {
		t.Error(reason)
	}
	second, err := collectImporterRun(context.Background(), src, "")
	if err != nil {
		t.Fatalf("Stream: unexpected error on repeat call: %v", err)
	}
	if reason := foreignItemsViolation("repeatability (run 1 vs run 2)", first.items, second.items); reason != "" {
		t.Error(reason)
	}
	if !slices.Equal(first.checkpoints, second.checkpoints) {
		t.Errorf("checkpoint determinism: run 1 = %v, run 2 = %v", first.checkpoints, second.checkpoints)
	}
	gradeImporterResume(t, src, first, want)
	gradeLegacyItems(t, src, want)
}

func collectImporterRun(ctx context.Context, src sdk.Source, checkpoint string) (importerRun, error) { //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	run := importerRun{}
	err := sdk.StreamSource(ctx, src, checkpoint, func(
		_ context.Context,
		page []sdk.ForeignItem,
		nextCheckpoint string,
	) error {
		run.items = append(run.items, page...)
		run.pageEnds = append(run.pageEnds, len(run.items))
		run.checkpoints = append(run.checkpoints, nextCheckpoint)
		return nil
	})
	return run, err
}

func gradeImporterResume(t *testing.T, src sdk.Source, first importerRun, want []sdk.ForeignItem) { //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	t.Helper()
	if len(first.checkpoints) == 0 {
		return
	}
	checkpointIndexes := []int{0}
	if len(first.checkpoints) > 1 {
		checkpointIndexes = append(checkpointIndexes, 1)
	}
	for _, checkpointIndex := range checkpointIndexes {
		checkpoint := first.checkpoints[checkpointIndex]
		prefixEnd := first.pageEnds[checkpointIndex]
		resumed, err := collectImporterRun(context.Background(), src, checkpoint)
		if err != nil {
			t.Fatalf("resume from checkpoint %q: %v", checkpoint, err)
		}
		combined := append([]sdk.ForeignItem(nil), first.items[:prefixEnd]...)
		combined = append(combined, resumed.items...)
		if reason := foreignItemsViolation("resume (committed prefix + continuation)", want, combined); reason != "" {
			t.Error(reason)
		}
		wantCheckpoints := first.checkpoints[checkpointIndex+1:]
		if !slices.Equal(wantCheckpoints, resumed.checkpoints) {
			t.Errorf("resume from checkpoint %q checkpoints = %v, want %v", checkpoint, resumed.checkpoints, wantCheckpoints)
		}
	}
}

func gradeLegacyItems(t *testing.T, src sdk.Source, want []sdk.ForeignItem) { //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	t.Helper()
	items, err := src.Items(context.Background()) //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	if err != nil {
		t.Fatalf("deprecated Items: unexpected error on a success vector: %v", err)
	}
	if reason := foreignItemsViolation("deprecated Items compatibility", want, items); reason != "" {
		t.Error(reason)
	}
}

// gradeImporterErrorVector asserts both APIs fail before emitting partial items.
func gradeImporterErrorVector(t *testing.T, src sdk.Source) { //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	t.Helper()
	run, streamErr := collectImporterRun(context.Background(), src, "")
	if reason := atomicityViolation(streamErr, run.items); reason != "" {
		t.Error("Stream " + reason)
	}
	items, itemsErr := src.Items(context.Background()) //nolint:staticcheck // deliberate 0.x compatibility path for pre-streaming sources
	if reason := atomicityViolation(itemsErr, items); reason != "" {
		t.Error("Items " + reason)
	}
}

// gradeImporterCancellation calls Stream directly so the source itself, not
// StreamSource's defensive preflight, must honor the cancelled context.
func gradeImporterCancellation(t *testing.T, src sdk.StreamingSource) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	emitted := []sdk.ForeignItem{}
	err := src.Stream(ctx, "", func(_ context.Context, page []sdk.ForeignItem, _ string) error {
		emitted = append(emitted, page...)
		return nil
	})
	if reason := cancellationViolation(err, emitted); reason != "" {
		t.Error(reason)
	}
}

// importerCategoryViolations reports how a Source's declared categories diverge
// from want: an unrecognized category (not in the fixed vocabulary), a missing
// expected category, or an unexpected extra. Order-insensitive; want must be
// non-empty (an importer that carries nothing is meaningless).
func importerCategoryViolations(got, want []string) []string {
	var violations []string
	valid := map[string]bool{}
	for _, c := range sdk.ImportCategories() {
		valid[c] = true
	}
	for _, c := range got {
		if !valid[c] {
			violations = append(violations, fmt.Sprintf("Categories: %q is not a recognized ImportCategories value", c))
		}
	}
	if len(want) == 0 {
		violations = append(violations, "importer vector declares no WantCategories; an importer that carries nothing is meaningless")
	}
	gotSet, wantSet := stringSet(got), stringSet(want)
	for _, c := range sortedKeys(wantSet) {
		if !gotSet[c] {
			violations = append(violations, fmt.Sprintf("Categories: missing expected category %q (got %v, want %v)", c, sortedKeys(gotSet), sortedKeys(wantSet)))
		}
	}
	for _, c := range sortedKeys(gotSet) {
		if !wantSet[c] {
			violations = append(violations, fmt.Sprintf("Categories: unexpected category %q (got %v, want %v)", c, sortedKeys(gotSet), sortedKeys(wantSet)))
		}
	}
	return violations
}

// foreignItemsViolation returns a diagnostic when got does not deeply equal want,
// naming the compared dimension and the first differing item; "" when equal.
func foreignItemsViolation(dimension string, want, got []sdk.ForeignItem) string {
	if len(want) == 0 && len(got) == 0 {
		return ""
	}
	if reflect.DeepEqual(want, got) {
		return ""
	}
	if len(want) != len(got) {
		return fmt.Sprintf("%s: item count %d, want %d", dimension, len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(want[i], got[i]) {
			return fmt.Sprintf("%s: item %d =\n  %+v\nwant\n  %+v", dimension, i, got[i], want[i])
		}
	}
	return fmt.Sprintf("%s: slices differ:\n  %+v\nwant\n  %+v", dimension, got, want)
}

// atomicityViolation grades an error vector: the run must report a non-nil error
// and emit no items before its first durable checkpoint.
func atomicityViolation(err error, items []sdk.ForeignItem) string {
	if err == nil {
		return "failure atomicity: returned nil error on a WantErr vector; the run was expected to abort"
	}
	if len(items) > 0 {
		return fmt.Sprintf("failure atomicity: emitted %d partial item(s) before error %v; this vector must abort before its first checkpoint", len(items), err)
	}
	return ""
}

// cancellationViolation grades a cancelled-context Stream call.
func cancellationViolation(err error, items []sdk.ForeignItem) string {
	if err == nil {
		return "cancellation: Stream returned nil error under a cancelled context; the StreamingSource must honor cancellation"
	}
	if !errors.Is(err, context.Canceled) {
		return fmt.Sprintf("cancellation: Stream returned %v, want context.Canceled", err)
	}
	if len(items) > 0 {
		return fmt.Sprintf("cancellation: Items emitted %d item(s) under a cancelled context; a cancelled run must emit none", len(items))
	}
	return ""
}

func stringSet(xs []string) map[string]bool {
	set := make(map[string]bool, len(xs))
	for _, x := range xs {
		set[x] = true
	}
	return set
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
