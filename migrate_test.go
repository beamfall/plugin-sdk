package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

type smartCollectionMigrationSource struct {
	items []ForeignItem
}

func (s smartCollectionMigrationSource) Categories() []string {
	return []string{CategoryCollections}
}

func (s smartCollectionMigrationSource) Items(ctx context.Context) ([]ForeignItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]ForeignItem(nil), s.items...), nil
}

// TestForeignItemRatingAndCollectionMemberAreAdditive pins that an
// older producer that builds a ForeignItem with a keyed struct literal
// omitting Rating/CollectionMember must still compile and decode as "no
// rating / no membership" — the two new fields are additive, never required.
func TestForeignItemRatingAndCollectionMemberAreAdditive(t *testing.T) {
	older := ForeignItem{
		Path: "/library/movie.mkv",
		WatchState: WatchState{
			Present: true,
			Watched: true,
		},
	}

	if older.Rating.Present {
		t.Fatalf("older ForeignItem literal got a non-zero Rating: %+v", older.Rating)
	}
	if older.CollectionMember.Present {
		t.Fatalf("older ForeignItem literal got a non-zero CollectionMember: %+v", older.CollectionMember)
	}
	if older.SmartCollection.Identity != "" || older.SmartCollection.CurrentMembersPresent || older.SmartCollection.CurrentMembers != nil {
		t.Fatalf("older ForeignItem literal got non-zero SmartCollection identity/members: %+v", older.SmartCollection)
	}
}

// TestSmartCollectionMigrationShapeConformance drives the Source/Items seam and
// pins the collection snapshot contract consumed across plugin and Core
// repositories: identity survives a rename/replay, member identities retain
// source order, and explicit empty, missing, and legacy count-only states stay
// distinguishable.
func TestSmartCollectionMigrationShapeConformance(t *testing.T) {
	members := []ForeignItem{
		{ExternalIDs: []ExternalID{{Scheme: "catalog", Value: "first"}}},
		{Path: "/library/second.mkv"},
		{Scrobble: ScrobbleRecord{Present: true, Artist: "Third Artist", Track: "Third Track", Album: "Third Album"}},
	}
	var source Source = smartCollectionMigrationSource{
		items: []ForeignItem{
			{SmartCollection: SmartCollection{
				Present:               true,
				Identity:              "source-collection-42",
				Name:                  "Current Favorites",
				CurrentMembersPresent: true,
				CurrentMembers:        members,
				MemberCount:           3,
			}},
			{SmartCollection: SmartCollection{
				Present:               true,
				Identity:              "source-collection-42",
				Name:                  "Renamed Favorites",
				CurrentMembersPresent: true,
				CurrentMembers:        append([]ForeignItem(nil), members...),
				MemberCount:           3,
			}},
			{SmartCollection: SmartCollection{
				Present:               true,
				Identity:              "source-collection-empty",
				Name:                  "Empty Today",
				CurrentMembersPresent: true,
				CurrentMembers:        []ForeignItem{},
			}},
			{SmartCollection: SmartCollection{
				Present:     true,
				Identity:    "source-collection-legacy",
				Name:        "Legacy Favorites",
				MemberCount: 3,
			}},
			{},
		},
	}
	if got := source.Categories(); !reflect.DeepEqual(got, []string{CategoryCollections}) {
		t.Fatalf("Categories() = %v, want [%s]", got, CategoryCollections)
	}
	items, err := source.Items(context.Background())
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	if len(items) != 5 {
		t.Fatalf("Items() count = %d, want 5 conformance cases", len(items))
	}
	original := items[0].SmartCollection
	replay := items[1].SmartCollection
	empty := items[2].SmartCollection
	legacy := items[3].SmartCollection
	missing := items[4].SmartCollection

	t.Run("stable identity survives rename and replay", func(t *testing.T) {
		if replay.Identity != original.Identity {
			t.Fatalf("replay Identity = %q, want stable %q", replay.Identity, original.Identity)
		}
		if replay.Name == original.Name {
			t.Fatalf("replay Name = %q, want renamed display text", replay.Name)
		}
		if !reflect.DeepEqual(replay.CurrentMembers, original.CurrentMembers) {
			t.Fatalf("replay members = %+v, want ordered %+v", replay.CurrentMembers, original.CurrentMembers)
		}
	})

	t.Run("ordered mixed member identities", func(t *testing.T) {
		if !original.CurrentMembersPresent {
			t.Fatal("CurrentMembersPresent = false, want true")
		}
		if !reflect.DeepEqual(original.CurrentMembers, members) {
			t.Fatalf("CurrentMembers = %+v, want source order %+v", original.CurrentMembers, members)
		}
		if original.MemberCount != len(original.CurrentMembers) {
			t.Fatalf("MemberCount = %d, members = %d", original.MemberCount, len(original.CurrentMembers))
		}
	})

	t.Run("explicit empty snapshot", func(t *testing.T) {
		if !empty.CurrentMembersPresent || empty.CurrentMembers == nil || len(empty.CurrentMembers) != 0 {
			t.Fatalf("explicit empty snapshot = %+v, want present non-nil zero-length members", empty)
		}
		if !empty.Present {
			t.Fatal("explicit empty snapshot Present = false, want true")
		}
	})

	t.Run("missing and legacy count-only input are detectable", func(t *testing.T) {
		if legacy.CurrentMembersPresent || legacy.CurrentMembers != nil {
			t.Fatalf("legacy count-only input = %+v, want absent member snapshot", legacy)
		}
		if !legacy.Present || legacy.MemberCount != 3 {
			t.Fatalf("legacy count-only input = %+v, want present collection with count 3", legacy)
		}
		if missing.Present || missing.Identity != "" || missing.CurrentMembersPresent || missing.CurrentMembers != nil {
			t.Fatalf("missing collection = %+v, want all-zero", missing)
		}
		if reflect.DeepEqual(empty, legacy) || reflect.DeepEqual(legacy, missing) {
			t.Fatal("explicit empty, legacy count-only, and missing collection states must remain distinct")
		}
	})

	t.Run("repeatable source output", func(t *testing.T) {
		repeated, err := source.Items(context.Background())
		if err != nil {
			t.Fatalf("second Items() error = %v", err)
		}
		if !reflect.DeepEqual(repeated, items) {
			t.Fatalf("second Items() = %+v, want %+v", repeated, items)
		}
	})
}

// TestRatingCarriesSourceScale pins the generic (non-normalized) rating shape:
// Value stays on the source's own scale, Scale is carried alongside it so a
// handler can rescale generically instead of assuming any one source
// convention.
func TestRatingCarriesSourceScale(t *testing.T) {
	item := ForeignItem{
		Path:   "/library/movie.mkv",
		Rating: Rating{Present: true, Value: 8.5, Scale: 10},
	}

	if !item.Rating.Present {
		t.Fatalf("Rating.Present = false, want true")
	}
	if item.Rating.Value != 8.5 || item.Rating.Scale != 10 {
		t.Fatalf("Rating = %+v, want Value=8.5 Scale=10", item.Rating)
	}

	var zero Rating
	if zero.Present || zero.Value != 0 || zero.Scale != 0 {
		t.Fatalf("zero Rating = %+v, want all-zero", zero)
	}
}

// TestCollectionMemberReconstructsOrder pins the ordered-membership shape: a
// static collection's member identities are each item's own Path/ExternalIDs,
// grouped by CollectionMember.Name and ordered by Position — no separate
// member-identity list is carried on the collection itself.
func TestCollectionMemberReconstructsOrder(t *testing.T) {
	items := []ForeignItem{
		{Path: "/library/c.mkv", CollectionMember: CollectionMember{Present: true, Name: "Trilogy", Position: 2}},
		{Path: "/library/a.mkv", CollectionMember: CollectionMember{Present: true, Name: "Trilogy", Position: 0}},
		{Path: "/library/other.mkv", CollectionMember: CollectionMember{Present: true, Name: "Unrelated", Position: 0}},
		{Path: "/library/b.mkv", CollectionMember: CollectionMember{Present: true, Name: "Trilogy", Position: 1}},
		{Path: "/library/no-membership.mkv"},
	}

	byPosition := map[int]string{}
	unmembered := 0
	for _, item := range items {
		if !item.CollectionMember.Present {
			unmembered++
			continue
		}
		if item.CollectionMember.Name != "Trilogy" {
			continue
		}
		byPosition[item.CollectionMember.Position] = item.Path
	}

	if unmembered != 1 {
		t.Fatalf("unmembered count = %d, want 1", unmembered)
	}
	want := map[int]string{0: "/library/a.mkv", 1: "/library/b.mkv", 2: "/library/c.mkv"}
	for pos, path := range want {
		if byPosition[pos] != path {
			t.Fatalf("Trilogy position %d = %q, want %q", pos, byPosition[pos], path)
		}
	}

	var zero CollectionMember
	if zero.Present || zero.Name != "" || zero.Position != 0 {
		t.Fatalf("zero CollectionMember = %+v, want all-zero", zero)
	}
}

// TestImportCategoriesStillCoversRatingsAndCollections is a narrow regression
// pin: this change extends the existing fixed ratings/collections
// categories with real data carriers — it must not add a new category or
// rename an existing one (Forbidden: new category vocabulary).
func TestImportCategoriesStillCoversRatingsAndCollections(t *testing.T) {
	cats := ImportCategories()
	want := map[string]bool{CategoryRatings: false, CategoryCollections: false}
	for _, c := range cats {
		if _, ok := want[c]; ok {
			want[c] = true
		}
	}
	for cat, seen := range want {
		if !seen {
			t.Fatalf("ImportCategories() missing %q: %v", cat, cats)
		}
	}
	if len(cats) != 5 {
		t.Fatalf("ImportCategories() length = %d, want 5 (no new category added)", len(cats))
	}
}

type syntheticStreamingSource struct {
	total       int
	pageSize    int
	legacyCalls *int
}

func (syntheticStreamingSource) Categories() []string {
	return []string{CategoryWatchState}
}

func (s syntheticStreamingSource) Items(context.Context) ([]ForeignItem, error) {
	if s.legacyCalls != nil {
		(*s.legacyCalls)++
	}
	return nil, errors.New("legacy Items called")
}

func (s syntheticStreamingSource) Stream(
	ctx context.Context,
	checkpoint string,
	emit func(context.Context, []ForeignItem, string) error,
) error {
	start := 0
	if checkpoint != "" {
		var err error
		start, err = strconv.Atoi(checkpoint)
		if err != nil || start < 0 || start > s.total {
			return errors.New("invalid synthetic checkpoint")
		}
	}
	for start < s.total {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(start+s.pageSize, s.total)
		page := make([]ForeignItem, end-start)
		for i := range page {
			page[i].WatchState.PositionSeconds = start + i
		}
		if err := emit(ctx, page, strconv.Itoa(end)); err != nil {
			return err
		}
		start = end
	}
	return nil
}

func TestStreamSourceMillionItemsStayBoundedByPage(t *testing.T) {
	legacyCalls := 0
	source := syntheticStreamingSource{
		total:       1_000_000,
		pageSize:    MaxImportPageItems,
		legacyCalls: &legacyCalls,
	}
	itemCount := 0
	maxPage := 0
	err := StreamSource(context.Background(), source, "", func(_ context.Context, page []ForeignItem, _ string) error {
		itemCount += len(page)
		maxPage = max(maxPage, len(page))
		return nil
	})
	if err != nil {
		t.Fatalf("StreamSource: %v", err)
	}
	if itemCount != 1_000_000 {
		t.Fatalf("item count = %d, want 1000000", itemCount)
	}
	if maxPage != MaxImportPageItems {
		t.Fatalf("maximum page = %d, want %d", maxPage, MaxImportPageItems)
	}
	if legacyCalls != 0 {
		t.Fatalf("legacy Items calls = %d, want 0", legacyCalls)
	}
}

func TestStreamSourceCancellationStopsBeforeAnotherPage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	itemCount := 0
	err := StreamSource(ctx, syntheticStreamingSource{total: 10_000, pageSize: 64}, "", func(
		_ context.Context,
		page []ForeignItem,
		_ string,
	) error {
		itemCount += len(page)
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StreamSource error = %v, want context.Canceled", err)
	}
	if itemCount != 64 {
		t.Fatalf("items emitted after cancellation = %d, want exactly first page (64)", itemCount)
	}
}

func TestStreamSourceCancellationAfterFinalPageIsReported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	err := StreamSource(ctx, syntheticStreamingSource{total: 64, pageSize: 64}, "", func(
		context.Context,
		[]ForeignItem,
		string,
	) error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StreamSource error = %v, want context.Canceled", err)
	}
}

type onePageStreamingSource struct {
	pageCtx context.Context
	items   []ForeignItem
	next    string
}

func (onePageStreamingSource) Categories() []string {
	return []string{CategoryWatchState}
}

func (onePageStreamingSource) Items(context.Context) ([]ForeignItem, error) {
	return nil, errors.New("legacy Items called")
}

func (s onePageStreamingSource) Stream(
	_ context.Context,
	_ string,
	emit func(context.Context, []ForeignItem, string) error,
) error {
	return emit(s.pageCtx, s.items, s.next)
}

func TestStreamSourceRejectsInvalidStreamingPages(t *testing.T) {
	tests := []struct {
		name   string
		source onePageStreamingSource
	}{
		{
			name:   "nil context",
			source: onePageStreamingSource{items: []ForeignItem{{}}, next: "1"},
		},
		{
			name:   "empty page",
			source: onePageStreamingSource{pageCtx: context.Background(), next: "1"},
		},
		{
			name: "oversized page",
			source: onePageStreamingSource{
				pageCtx: context.Background(),
				items:   make([]ForeignItem, MaxImportPageItems+1),
				next:    strconv.Itoa(MaxImportPageItems + 1),
			},
		},
		{
			name:   "empty checkpoint",
			source: onePageStreamingSource{pageCtx: context.Background(), items: []ForeignItem{{}}},
		},
		{
			name:   "checkpoint did not advance",
			source: onePageStreamingSource{pageCtx: context.Background(), items: []ForeignItem{{}}, next: "resume"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			checkpoint := ""
			if test.name == "checkpoint did not advance" {
				checkpoint = "resume"
			}
			err := StreamSource(context.Background(), test.source, checkpoint, func(
				context.Context,
				[]ForeignItem,
				string,
			) error {
				return nil
			})
			if !errors.Is(err, ErrInvalidImportPage) {
				t.Fatalf("StreamSource error = %v, want ErrInvalidImportPage", err)
			}
		})
	}
}

func TestStreamSourceRejectsNilCallback(t *testing.T) {
	err := StreamSource(context.Background(), legacySource{}, "", nil)
	if !errors.Is(err, ErrInvalidImportPage) {
		t.Fatalf("StreamSource error = %v, want ErrInvalidImportPage", err)
	}
}

func TestStreamSourceResumeHasNoDuplicatesOrLoss(t *testing.T) {
	source := syntheticStreamingSource{total: 1_000, pageSize: 128}
	crash := errors.New("synthetic crash")
	committed := make([]ForeignItem, 0, 1_000)
	checkpoint := ""
	pageCount := 0
	err := StreamSource(context.Background(), source, "", func(
		_ context.Context,
		page []ForeignItem,
		next string,
	) error {
		pageCount++
		if pageCount == 2 {
			return crash
		}
		committed = append(committed, page...)
		checkpoint = next
		return nil
	})
	if err != crash {
		t.Fatalf("interrupted StreamSource error = %v, want exact callback error", err)
	}
	err = StreamSource(context.Background(), source, checkpoint, func(
		_ context.Context,
		page []ForeignItem,
		next string,
	) error {
		committed = append(committed, page...)
		checkpoint = next
		return nil
	})
	if err != nil {
		t.Fatalf("resumed StreamSource: %v", err)
	}
	if len(committed) != 1_000 {
		t.Fatalf("committed item count = %d, want 1000", len(committed))
	}
	for i, item := range committed {
		if item.WatchState.PositionSeconds != i {
			t.Fatalf("committed item %d position = %d, want %d", i, item.WatchState.PositionSeconds, i)
		}
	}
	if checkpoint != "1000" {
		t.Fatalf("final checkpoint = %q, want %q", checkpoint, "1000")
	}
}

type durableImportState struct {
	Items      []ForeignItem `json:"items"`
	Checkpoint string        `json:"checkpoint"`
}

type durableImportStore struct {
	path  string
	state durableImportState
}

func openDurableImportStore(path string) (*durableImportStore, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &durableImportStore{path: path}, nil
	}
	if err != nil {
		return nil, err
	}
	var state durableImportState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &durableImportStore{path: path, state: state}, nil
}

func (s *durableImportStore) applyPage(
	page []ForeignItem,
	nextCheckpoint string,
	crashBeforeCheckpoint bool,
) error {
	next := durableImportState{
		Items:      append(append([]ForeignItem(nil), s.state.Items...), page...),
		Checkpoint: s.state.Checkpoint,
	}
	staged, err := os.CreateTemp(filepath.Dir(s.path), "import-state-*.tmp")
	if err != nil {
		return err
	}
	stagedPath := staged.Name()
	committed := false
	defer func() {
		_ = staged.Close()
		if committed {
			_ = os.Remove(stagedPath)
		}
	}()
	if err := json.NewEncoder(staged).Encode(next); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if crashBeforeCheckpoint {
		return errSimulatedImportCrash
	}
	next.Checkpoint = nextCheckpoint
	if err := staged.Truncate(0); err != nil {
		return err
	}
	if _, err := staged.Seek(0, 0); err != nil {
		return err
	}
	if err := json.NewEncoder(staged).Encode(next); err != nil {
		return err
	}
	if err := staged.Sync(); err != nil {
		return err
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := os.Rename(stagedPath, s.path); err != nil {
		return err
	}
	committed = true
	s.state = next
	return nil
}

var errSimulatedImportCrash = errors.New("simulated process crash before checkpoint commit")

func TestStreamSourceDurableCrashRestartHasNoDuplicatesOrLoss(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "import-state.json")
	store, err := openDurableImportStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	source := syntheticStreamingSource{total: 1_000, pageSize: 128}
	pageCount := 0
	err = StreamSource(context.Background(), source, store.state.Checkpoint, func(
		_ context.Context,
		page []ForeignItem,
		nextCheckpoint string,
	) error {
		pageCount++
		return store.applyPage(page, nextCheckpoint, pageCount == 2)
	})
	if !errors.Is(err, errSimulatedImportCrash) {
		t.Fatalf("interrupted StreamSource error = %v, want simulated crash", err)
	}
	stagedPaths, err := filepath.Glob(filepath.Join(filepath.Dir(statePath), "import-state-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(stagedPaths) != 1 {
		t.Fatalf("staged snapshots after crash = %v, want one", stagedPaths)
	}
	stagedStore, err := openDurableImportStore(stagedPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(stagedStore.state.Items) != 256 || stagedStore.state.Checkpoint != "128" {
		t.Fatalf("crashed staged state = %d items at %q, want 256 items at old checkpoint 128", len(stagedStore.state.Items), stagedStore.state.Checkpoint)
	}

	restartedStore, err := openDurableImportStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(restartedStore.state.Items) != 128 || restartedStore.state.Checkpoint != "128" {
		t.Fatalf("reopened state after crash = %d items at %q, want 128 items at checkpoint 128", len(restartedStore.state.Items), restartedStore.state.Checkpoint)
	}
	restartedSource := syntheticStreamingSource{total: 1_000, pageSize: 128}
	err = StreamSource(context.Background(), restartedSource, restartedStore.state.Checkpoint, func(
		_ context.Context,
		page []ForeignItem,
		nextCheckpoint string,
	) error {
		return restartedStore.applyPage(page, nextCheckpoint, false)
	})
	if err != nil {
		t.Fatalf("resumed StreamSource: %v", err)
	}

	verifiedStore, err := openDurableImportStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(verifiedStore.state.Items) != 1_000 {
		t.Fatalf("durable item count = %d, want 1000", len(verifiedStore.state.Items))
	}
	for i, item := range verifiedStore.state.Items {
		if item.WatchState.PositionSeconds != i {
			t.Fatalf("durable item %d position = %d, want %d", i, item.WatchState.PositionSeconds, i)
		}
	}
	if verifiedStore.state.Checkpoint != "1000" {
		t.Fatalf("final durable checkpoint = %q, want 1000", verifiedStore.state.Checkpoint)
	}
}

type legacySource struct {
	items []ForeignItem
}

func (legacySource) Categories() []string {
	return []string{CategoryWatchState}
}

func (s legacySource) Items(ctx context.Context) ([]ForeignItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]ForeignItem(nil), s.items...), nil
}

func TestStreamSourceAdaptsLegacySourceAndCheckpoint(t *testing.T) {
	items := make([]ForeignItem, MaxImportPageItems+44)
	source := legacySource{items: items}
	pageSizes := []int{}
	checkpoint := ""
	err := StreamSource(context.Background(), source, "", func(
		_ context.Context,
		page []ForeignItem,
		next string,
	) error {
		pageSizes = append(pageSizes, len(page))
		checkpoint = next
		return nil
	})
	if err != nil {
		t.Fatalf("StreamSource legacy adapter: %v", err)
	}
	if len(pageSizes) != 2 || pageSizes[0] != MaxImportPageItems || pageSizes[1] != 44 {
		t.Fatalf("legacy page sizes = %v, want [%d 44]", pageSizes, MaxImportPageItems)
	}
	if checkpoint != strconv.Itoa(len(items)) {
		t.Fatalf("legacy final checkpoint = %q, want %d", checkpoint, len(items))
	}

	resumed := 0
	err = StreamSource(context.Background(), source, strconv.Itoa(MaxImportPageItems), func(
		_ context.Context,
		page []ForeignItem,
		_ string,
	) error {
		resumed += len(page)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamSource legacy resume: %v", err)
	}
	if resumed != 44 {
		t.Fatalf("legacy resumed items = %d, want 44", resumed)
	}
}
