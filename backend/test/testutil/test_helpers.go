package testutil

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"

	"path/filepath"
	"runtime"

	"openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/milestone"
	"openmars/internal/game/shared"
	"openmars/internal/logger"
)

// TestContext provides a reusable test context
func TestContext() context.Context {
	return context.Background()
}

// TestLogger returns the process logger, the one the server's own code logs to.
func TestLogger() *slog.Logger {
	return logger.Get()
}

// CreateTestCardRegistry returns the real card registry loaded from the JSON database.
func CreateTestCardRegistry() gamecards.CardRegistry {
	return GetCardDB()
}

// CreateTestModeCatalog loads the complete mode-aware application catalog.
func CreateTestModeCatalog() gamecards.CardRegistry {
	_, currentFile, _, _ := runtime.Caller(0)
	catalog, err := gamecards.LoadCatalog(filepath.Join(filepath.Dir(currentFile), "..", "..", "assets", "cards.json"))
	if err != nil {
		panic(fmt.Sprintf("failed to load mode catalog: %v", err))
	}
	return catalog
}

// CreateTestAwardRegistry returns an award registry loaded from the JSON database.
func CreateTestAwardRegistry() award.AwardRegistry {
	_, currentFile, _, _ := runtime.Caller(0)
	jsonPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "assets", "awards.json")
	awardList, err := award.LoadAwardsFromJSON(jsonPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load award DB for tests: %v", err))
	}
	return award.NewInMemoryAwardRegistry(awardList)
}

// CreateTestMilestoneRegistry returns a milestone registry loaded from the JSON database.
func CreateTestMilestoneRegistry() milestone.MilestoneRegistry {
	_, currentFile, _, _ := runtime.Caller(0)
	jsonPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "assets", "milestones.json")
	milestoneList, err := milestone.LoadMilestonesFromJSON(jsonPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load milestone DB for tests: %v", err))
	}
	return milestone.NewInMemoryMilestoneRegistry(milestoneList)
}

// CreateTestCardRegistryWithAdditionalCards creates a card registry with real cards plus additional synthetic cards.
func CreateTestCardRegistryWithAdditionalCards(additionalCards []gamecards.Card) gamecards.CardRegistry {
	allCards := GetCardDB().GetAll()
	allCards = append(allCards, additionalCards...)
	return gamecards.NewInMemoryCardRegistry(allCards)
}

// NewTestDataStoreWithGame creates a DataStore with an empty GameState pre-inserted.
func NewTestDataStoreWithGame(t *testing.T, gameID string) *datastore.DataStore {
	t.Helper()
	ds, err := datastore.NewDataStore()
	if err != nil {
		t.Fatalf("Failed to create datastore: %v", err)
	}
	txn := ds.BeginTxn()
	if err := txn.InsertGame(&datastore.GameState{ID: gameID}); err != nil {
		t.Fatalf("Failed to insert game state: %v", err)
	}
	txn.Commit()
	return ds
}

// NewTestGameRepository creates a MemDB-backed game repository for tests.
func NewTestGameRepository(t testing.TB) game.GameRepository {
	t.Helper()
	ds, err := datastore.NewDataStore()
	if err != nil {
		t.Fatalf("Failed to create datastore: %v", err)
	}
	rm := datastore.NewRuntimeManager()
	return game.NewMemDBGameRepository(ds, rm)
}

// uniqueGameID names a test game after its test, so two games in one repository never
// share an ID.
func uniqueGameID(t *testing.T) string {
	return fmt.Sprintf("%s-%d", t.Name(), gameIDSeq.Add(1))
}

var gameIDSeq atomic.Int64

// CreateTestGameWithPlayers creates a game with specified number of players
func CreateTestGameWithPlayers(t *testing.T, numPlayers int) (*game.Game, game.GameRepository) {
	t.Helper()

	repo := NewTestGameRepository(t)
	cardRegistry := CreateTestCardRegistry()

	// Create game
	settings := shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{},
	}

	testGame := game.NewGame(repo.DataStore(), uniqueGameID(t), "", settings, board.GenerateMarsBoard(false))
	allCards := cardRegistry.GetAll()

	// Separate cards by type
	projectCards := make([]string, 0)
	corpCards := make([]string, 0)
	preludeCards := make([]string, 0)

	for _, card := range allCards {
		switch card.Type {
		case gamecards.CardTypeCorporation:
			corpCards = append(corpCards, card.ID)
		case gamecards.CardTypePrelude:
			preludeCards = append(preludeCards, card.ID)
		default:
			projectCards = append(projectCards, card.ID)
		}
	}

	// Create and set deck
	testGame.InitDeck(projectCards, corpCards, preludeCards)
	testGame.SetVPCardLookup(gamecards.NewVPCardLookupAdapter(cardRegistry))

	err := repo.Create(context.Background(), testGame)
	if err != nil {
		t.Fatalf("Failed to create test game: %v", err)
	}

	// Add players
	ctx := context.Background()
	for i := 0; i < numPlayers; i++ {
		playerID := fmt.Sprintf("player-%d", i+1)
		playerName := "Player " + string(rune('A'+i))

		// Create and add player
		p, err := testGame.AddNewPlayer(ctx, playerID, playerName)
		if err != nil {
			t.Fatalf("Failed to add player %d: %v", i, err)
		}
		action.SetupPlayerCardStore(p, testGame, cardRegistry)
	}

	// Set first player as host (like JoinGameAction does)
	if numPlayers > 0 {
		if err := testGame.SetHostPlayerID(ctx, "player-1"); err != nil {
			t.Fatalf("Failed to set host player: %v", err)
		}
	}

	return testGame, repo
}

// CreateTestGameWithVenus creates a test game with Venus Next enabled and the specified number of players
func CreateTestGameWithVenus(t *testing.T, numPlayers int) (*game.Game, game.GameRepository) {
	t.Helper()

	repo := NewTestGameRepository(t)
	cardRegistry := CreateTestCardRegistry()

	settings := shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{}, Systems: []string{"venus"},
	}

	testGame := game.NewGame(repo.DataStore(), uniqueGameID(t), "", settings, board.GenerateMarsBoard(settings.HasVenus()))
	allCards := cardRegistry.GetAll()

	projectCards := make([]string, 0)
	corpCards := make([]string, 0)
	preludeCards := make([]string, 0)

	for _, card := range allCards {
		switch card.Type {
		case gamecards.CardTypeCorporation:
			corpCards = append(corpCards, card.ID)
		case gamecards.CardTypePrelude:
			preludeCards = append(preludeCards, card.ID)
		default:
			projectCards = append(projectCards, card.ID)
		}
	}

	testGame.InitDeck(projectCards, corpCards, preludeCards)
	testGame.SetVPCardLookup(gamecards.NewVPCardLookupAdapter(cardRegistry))

	err := repo.Create(context.Background(), testGame)
	if err != nil {
		t.Fatalf("Failed to create test game: %v", err)
	}

	ctx := context.Background()
	for i := 0; i < numPlayers; i++ {
		playerID := fmt.Sprintf("player-%d", i+1)
		playerName := "Player " + string(rune('A'+i))
		p, err := testGame.AddNewPlayer(ctx, playerID, playerName)
		if err != nil {
			t.Fatalf("Failed to add player %d: %v", i, err)
		}
		action.SetupPlayerCardStore(p, testGame, cardRegistry)
	}

	if numPlayers > 0 {
		if err := testGame.SetHostPlayerID(ctx, "player-1"); err != nil {
			t.Fatalf("Failed to set host player: %v", err)
		}
	}

	return testGame, repo
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t testing.TB, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}

// AssertErrorContains fails the test unless err is an error whose message contains want.
func AssertErrorContains(t *testing.T, err error, want string, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected an error containing %q, got nil", msg, want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("%s: expected an error containing %q, got %q", msg, want, err.Error())
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual[T comparable](t *testing.T, expected, actual T, message string) {
	t.Helper()
	if expected != actual {
		t.Fatalf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotEqual fails the test if expected == actual
func AssertNotEqual[T comparable](t *testing.T, expected, actual T, message string) {
	t.Helper()
	if expected == actual {
		t.Fatalf("%s: expected not equal to %v", message, expected)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool, message string) {
	t.Helper()
	if !condition {
		t.Fatalf("%s: expected true, got false", message)
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool, message string) {
	t.Helper()
	if condition {
		t.Fatalf("%s: expected false, got true", message)
	}
}

// IntPtr returns a pointer to the given int value.
func IntPtr(v int) *int { return &v }

// StrPtr returns a pointer to the given string value.
func StrPtr(v string) *string { return &v }

// TagPtr returns a pointer to the given CardTag value.
func TagPtr(v shared.CardTag) *shared.CardTag { return &v }

// ResourceTypePtr returns a pointer to the given ResourceType value.
func ResourceTypePtr(v shared.ResourceType) *shared.ResourceType { return &v }

// CreateTestMapRegistry returns a MapRegistry loaded from the JSON database.
func CreateTestMapRegistry() *board.MapRegistry {
	_, currentFile, _, _ := runtime.Caller(0)
	jsonPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "assets", "maps.json")
	registry, err := board.LoadMapsFromJSON(jsonPath)
	if err != nil {
		panic(fmt.Sprintf("failed to load map DB for tests: %v", err))
	}
	return registry
}

// PlaceholderCorporationID is a corporation with no card behind it, for tests that need
// a player to have chosen a corporation without any corporation effects.
const PlaceholderCorporationID = "test-placeholder-corporation"
