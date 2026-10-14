package dto_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
	"openmars/test/testutil"
)

// loadStandardProjectRegistryDto loads the real standard-project registry so the mapper
// iterates the same action surfaces a live game does.
func loadStandardProjectRegistryDto(t *testing.T) standardproject.StandardProjectRegistry {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	stdProjPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "assets", "standard_projects.json")
	stdProjData, err := standardproject.LoadStandardProjectsFromJSON(stdProjPath)
	if err != nil {
		t.Fatalf("Failed to load standard projects JSON: %v", err)
	}
	return standardproject.NewInMemoryStandardProjectRegistry(stdProjData)
}

// setupStuckPlayerGame builds a single-player base game in the action phase where the
// player has no legal move: empty hand, zero of every resource, no played-card actions,
// colonies disabled. It mirrors the #571 repro fixture at the DTO layer.
func setupStuckPlayerGame(t *testing.T) (*game.Game, *player.Player, gamecards.CardRegistry) {
	t.Helper()

	settings := shared.GameSettings{
		MaxPlayers:      5,
		DevelopmentMode: true,
		GameModeID:      "classic", Features: []string{},
	}
	ds, err := datastore.NewDataStore()
	if err != nil {
		t.Fatalf("Failed to create datastore: %v", err)
	}
	g := game.NewGame(ds, "dto-571-game", "player1", settings, board.GenerateMarsBoard(false))

	ctx := context.Background()
	p, err := g.AddNewPlayer(ctx, "player1", "Stuck Player")
	if err != nil {
		t.Fatalf("Failed to add player: %v", err)
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{})

	p.Resources().Set(shared.Resources{})
	p.Actions().SetActions([]shared.CardAction{})
	p.Hand().SetCards([]string{})

	if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
		t.Fatalf("Failed to set action phase: %v", err)
	}
	if err := g.SetTurnOrder(ctx, []string{p.ID()}); err != nil {
		t.Fatalf("Failed to set turn order: %v", err)
	}
	if err := g.SetCurrentTurn(ctx, p.ID(), 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	return g, p, cardRegistry
}

// TestToPlayerDto_HasAvailableActions_FalseWhenStuck proves that with every registry
// present and a genuinely stuck player, the mapper sets a non-nil pointer to false.
func TestToPlayerDto_HasAvailableActions_FalseWhenStuck(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayerGame(t)
	stdProjRegistry := loadStandardProjectRegistryDto(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	playerDto := dto.ToPlayerDto(p, g, cardRegistry, stdProjRegistry, awardRegistry, milestoneRegistry, nil)

	if playerDto.HasAvailableActions == nil {
		t.Fatalf("expected non-nil HasAvailableActions when all registries are present")
	}
	if *playerDto.HasAvailableActions {
		t.Fatalf("expected HasAvailableActions to be false for a stuck player, got true")
	}
}

// TestToPlayerDto_HasAvailableActions_TrueWhenAffordable proves the pointer is non-nil
// true once the player can reach any single surface (here, an affordable standard project).
func TestToPlayerDto_HasAvailableActions_TrueWhenAffordable(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayerGame(t)
	stdProjRegistry := loadStandardProjectRegistryDto(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	// Enough credits to afford the PowerPlant standard project.
	powerPlant, err := stdProjRegistry.GetByID(string(shared.StandardProjectPowerPlant))
	if err != nil {
		t.Fatalf("failed to get PowerPlant definition: %v", err)
	}
	p.Resources().Set(shared.Resources{Credits: powerPlant.CreditCost()})

	playerDto := dto.ToPlayerDto(p, g, cardRegistry, stdProjRegistry, awardRegistry, milestoneRegistry, nil)

	if playerDto.HasAvailableActions == nil {
		t.Fatalf("expected non-nil HasAvailableActions when all registries are present")
	}
	if !*playerDto.HasAvailableActions {
		t.Fatalf("expected HasAvailableActions to be true once PowerPlant is affordable, got false")
	}
}

// TestToPlayerDto_HasAvailableActions_NilWhenRegistriesMissing proves the HTTP path
// (nil registries) leaves the pointer nil so JSON omits the field entirely -- an explicit
// "unknown", never a fabricated false.
func TestToPlayerDto_HasAvailableActions_NilWhenRegistriesMissing(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayerGame(t)

	playerDto := dto.ToPlayerDto(p, g, cardRegistry, nil, nil, nil, nil)

	if playerDto.HasAvailableActions != nil {
		t.Fatalf("expected nil HasAvailableActions on the nil-registry HTTP path, got %v", *playerDto.HasAvailableActions)
	}
}

func TestPendingCardPlay_PrivateDiscountAndBadges(t *testing.T) {
	g, _ := testutil.CreateTestGameWithPlayers(t, 2)
	p := g.GetAllPlayers()[0]
	registry := testutil.CreateTestCardRegistry()
	p.Hand().AddCard(testutil.CardID("Space Elevator"))
	p.Selection().SetPendingCardPlaySelection(&shared.PendingCardPlaySelection{Source: "Sponsor", SourceCardID: "P11", CostReduction: 25})
	own := dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil)
	testutil.AssertTrue(t, own.PendingCardPlaySelection != nil, "owner sees selection")
	testutil.AssertEqual(t, 1, len(own.PendingCardPlaySelection.AvailableCards), "hand options")
	testutil.AssertEqual(t, 2, own.PendingCardPlaySelection.AvailableCards[0].EffectiveCost, "discounted quote")
	other, err := json.Marshal(dto.ToOtherPlayerDto(p, g, registry))
	testutil.AssertNoError(t, err, "opponent DTO")
	testutil.AssertTrue(t, !bytes.Contains(other, []byte("pendingCardPlaySelection")), "opponents cannot see pending hand play")
	for _, tc := range []struct{ card, location string }{{"Phobos Space Haven", "phobos"}, {"Dawn City", "mercury"}, {"Ganymede Colony", "ganymede"}, {"Maxwell Base", "venus"}} {
		card := testutil.GetCardByName(tc.card)
		card.ID = "unrelated-id"
		view := dto.ToCardDto(card)
		found := false
		for _, behavior := range view.Behaviors {
			for _, output := range behavior.Outputs {
				if tile, ok := output.(dto.TilePlacementConditionDto); ok && tile.TileRestrictions != nil && tile.TileRestrictions.PlacementLocation == tc.location {
					found = true
				}
			}
		}
		testutil.AssertTrue(t, found, "placement badge is derived from site metadata")
	}
}
