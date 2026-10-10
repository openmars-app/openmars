package cards_test

import (
	"context"
	"testing"

	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// TestFixedVPCard verifies that a card with a fixed VP condition
// (e.g. Colonizer Training Camp, 2 VP) is correctly counted.
func TestFixedVPCard(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	// Colonizer Training Camp (001): vpConditions: [{amount:2, condition:"fixed"}]
	cardID := testutil.CardID("Colonizer Training Camp")
	p.PlayedCards().AddCard(cardID, "Colonizer Training Camp", "automated", []string{"building", "jovian"})

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil, // no milestones
		nil, // no awards
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 2 {
		t.Fatalf("expected CardVP=2 for Colonizer Training Camp, got %d", breakdown.CardVP)
	}
	if len(breakdown.CardVPDetails) != 1 {
		t.Fatalf("expected 1 card VP detail, got %d", len(breakdown.CardVPDetails))
	}
	if breakdown.CardVPDetails[0].CardID != cardID {
		t.Errorf("expected card ID %s, got %s", cardID, breakdown.CardVPDetails[0].CardID)
	}
	if breakdown.CardVPDetails[0].TotalVP != 2 {
		t.Errorf("expected card detail TotalVP=2, got %d", breakdown.CardVPDetails[0].TotalVP)
	}
}

// TestPerStorageVPCard verifies per-resource-on-self-card VP calculation.
// Predators (024): 1 VP per animal on this card.
func TestPerStorageVPCard(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	cardID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(cardID, "Predators", "active", []string{"animal"})

	// Add 5 animals to the card
	p.Resources().AddToStorage(cardID, 5)

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 5 {
		t.Fatalf("expected CardVP=5 for Predators with 5 animals, got %d", breakdown.CardVP)
	}
}

// TestPerOceanTileVPCard verifies per-ocean-tile VP calculation.
// Capital (008): 1 VP per ocean tile adjacent to Capital's city tile.
func TestPerOceanTileVPCard(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	ctx := context.Background()

	cardID := testutil.CardID("Capital")
	p.PlayedCards().AddCard(cardID, "Capital", "automated", []string{"building", "city"})

	// Place Capital's city tile at (0, 0, 0) with the source tag
	cityPos := shared.HexPosition{Q: 0, R: 0, S: 0}
	err := g.Board().UpdateTileOccupancy(ctx, cityPos, board.TileOccupant{
		Type: shared.ResourceCityTile,
		Tags: []string{"source:" + cardID},
	}, playerID)
	if err != nil {
		t.Fatalf("failed to place Capital city tile: %v", err)
	}

	// Place 2 ocean tiles adjacent to the city
	adjacentOceanPositions := []shared.HexPosition{
		{Q: 1, R: -1, S: 0},
		{Q: 0, R: -1, S: 1},
	}
	for _, pos := range adjacentOceanPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceOceanTile,
		}, "neutral")
		if err != nil {
			t.Fatalf("failed to place ocean tile: %v", err)
		}
	}

	// Place 1 ocean tile NOT adjacent to the city (should not count)
	err = g.Board().UpdateTileOccupancy(ctx, shared.HexPosition{Q: 3, R: -3, S: 0}, board.TileOccupant{
		Type: shared.ResourceOceanTile,
	}, "neutral")
	if err != nil {
		t.Fatalf("failed to place distant ocean tile: %v", err)
	}

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	// Only 2 adjacent oceans should count, not the distant one
	if breakdown.CardVP != 2 {
		t.Fatalf("expected CardVP=2 for Capital with 2 adjacent ocean tiles, got %d", breakdown.CardVP)
	}
}

// TestGreeneryVP verifies 1 VP per owned greenery tile.
func TestGreeneryVP(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	ctx := context.Background()

	// Place 2 greenery tiles owned by the player
	greeneryPositions := []shared.HexPosition{
		{Q: 0, R: 0, S: 0},
		{Q: 1, R: 0, S: -1},
	}
	for _, pos := range greeneryPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceGreeneryTile,
		}, p.ID())
		if err != nil {
			t.Fatalf("failed to place greenery: %v", err)
		}
	}

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.GreeneryVP != 2 {
		t.Fatalf("expected GreeneryVP=2, got %d", breakdown.GreeneryVP)
	}
}

// TestCityVP verifies VP from adjacent greenery tiles to owned cities.
func TestCityVP(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	ctx := context.Background()

	// Place a city at (0,0,0)
	err := g.Board().UpdateTileOccupancy(ctx, shared.HexPosition{Q: 0, R: 0, S: 0}, board.TileOccupant{
		Type: shared.ResourceCityTile,
	}, p.ID())
	if err != nil {
		t.Fatalf("failed to place city: %v", err)
	}

	// Place 2 greenery tiles adjacent to the city (neighbors of 0,0,0)
	greeneryPositions := []shared.HexPosition{
		{Q: 1, R: -1, S: 0}, // neighbor of (0,0,0)
		{Q: 0, R: -1, S: 1}, // neighbor of (0,0,0)
	}
	for _, pos := range greeneryPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceGreeneryTile,
		}, playerID)
		if err != nil {
			t.Fatalf("failed to place greenery: %v", err)
		}
	}

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CityVP != 2 {
		t.Fatalf("expected CityVP=2, got %d", breakdown.CityVP)
	}
}

// TestMultipleVPCards verifies total VP across multiple VP-granting cards.
func TestMultipleVPCards(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	// Play Colonizer Training Camp (001): 2 VP fixed
	p.PlayedCards().AddCard(testutil.CardID("Colonizer Training Camp"), "Colonizer Training Camp", "automated", []string{"building", "jovian"})

	// Play Predators (024): 1 VP per animal on this card → add 3 animals = 3 VP
	predatorsID := testutil.CardID("Predators")
	p.PlayedCards().AddCard(predatorsID, "Predators", "active", []string{"animal"})
	for i := 0; i < 3; i++ {
		p.Resources().AddToStorage(predatorsID, 1)
	}

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 5 {
		t.Fatalf("expected CardVP=5 (2+3), got %d", breakdown.CardVP)
	}
}

// TestTotalVPSumsAllCategories verifies that TotalVP is the sum of all components.
func TestTotalVPSumsAllCategories(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	ctx := context.Background()

	// Card VP: Colonizer Training Camp = 2 VP
	p.PlayedCards().AddCard(testutil.CardID("Colonizer Training Camp"), "Colonizer Training Camp", "automated", []string{"building", "jovian"})

	// Greenery VP: 1 greenery = 1 VP
	err := g.Board().UpdateTileOccupancy(ctx, shared.HexPosition{Q: 0, R: 0, S: 0}, board.TileOccupant{
		Type: shared.ResourceGreeneryTile,
	}, p.ID())
	if err != nil {
		t.Fatalf("failed to place greenery: %v", err)
	}

	// Milestone VP: claim 1 milestone = 5 VP
	milestones := []gamecards.ClaimedMilestoneInfo{
		{Type: "terraformer", PlayerID: playerID},
	}

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		milestones,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	expectedTotal := breakdown.TerraformRating + breakdown.CardVP + breakdown.MilestoneVP +
		breakdown.AwardVP + breakdown.GreeneryVP + breakdown.CityVP

	if breakdown.TotalVP != expectedTotal {
		t.Fatalf("TotalVP %d != sum of parts %d (TR=%d Card=%d Milestone=%d Award=%d Greenery=%d City=%d)",
			breakdown.TotalVP, expectedTotal,
			breakdown.TerraformRating, breakdown.CardVP, breakdown.MilestoneVP,
			breakdown.AwardVP, breakdown.GreeneryVP, breakdown.CityVP)
	}

	if breakdown.CardVP != 2 {
		t.Errorf("expected CardVP=2, got %d", breakdown.CardVP)
	}
	if breakdown.GreeneryVP != 1 {
		t.Errorf("expected GreeneryVP=1, got %d", breakdown.GreeneryVP)
	}
	if breakdown.MilestoneVP != 5 {
		t.Errorf("expected MilestoneVP=5, got %d", breakdown.MilestoneVP)
	}
}

// TestNegativeFixedVPCard verifies that a card with negative fixed VP
// (e.g. Nuclear Zone, -2 VP) correctly subtracts from the player's score.
func TestNegativeFixedVPCard(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	// Nuclear Zone (097): vpConditions: [{amount:-2, condition:"fixed"}]
	cardID := testutil.CardID("Nuclear Zone")
	p.PlayedCards().AddCard(cardID, "Nuclear Zone", "automated", []string{"earth"})

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != -2 {
		t.Fatalf("expected CardVP=-2 for Nuclear Zone, got %d", breakdown.CardVP)
	}
	if len(breakdown.CardVPDetails) != 1 {
		t.Fatalf("expected 1 card VP detail, got %d", len(breakdown.CardVPDetails))
	}
	if breakdown.CardVPDetails[0].TotalVP != -2 {
		t.Errorf("expected card detail TotalVP=-2, got %d", breakdown.CardVPDetails[0].TotalVP)
	}
}

// TestNegativeVPReducesTotalScore verifies that negative VP cards reduce the total score.
func TestNegativeVPReducesTotalScore(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	// Play Colonizer Training Camp (001): +2 VP fixed
	p.PlayedCards().AddCard(testutil.CardID("Colonizer Training Camp"), "Colonizer Training Camp", "automated", []string{"building", "jovian"})

	// Play Nuclear Zone (097): -2 VP fixed
	p.PlayedCards().AddCard(testutil.CardID("Nuclear Zone"), "Nuclear Zone", "automated", []string{"earth"})

	// Play Hackers (125): -1 VP fixed
	p.PlayedCards().AddCard(testutil.CardID("Hackers"), "Hackers", "automated", []string{"building"})

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	// 2 + (-2) + (-1) = -1
	if breakdown.CardVP != -1 {
		t.Fatalf("expected CardVP=-1 (2 - 2 - 1), got %d", breakdown.CardVP)
	}
	if len(breakdown.CardVPDetails) != 3 {
		t.Fatalf("expected 3 card VP details, got %d", len(breakdown.CardVPDetails))
	}
}

// --- Pets (172) ---
// "1 VP per 2 animals here."

func TestPetsVP_ZeroAnimals(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	cardID := testutil.CardID("Pets")
	p.PlayedCards().AddCard(cardID, "Pets", "active", []string{"animal", "earth"})

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 0 {
		t.Fatalf("expected CardVP=0 for Pets with 0 animals, got %d", breakdown.CardVP)
	}
}

func TestPetsVP_OneAnimal(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	cardID := testutil.CardID("Pets")
	p.PlayedCards().AddCard(cardID, "Pets", "active", []string{"animal", "earth"})
	p.Resources().AddToStorage(cardID, 1)

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 0 {
		t.Fatalf("expected CardVP=0 for Pets with 1 animal (floor division), got %d", breakdown.CardVP)
	}
}

func TestPetsVP_TwoAnimals(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	cardID := testutil.CardID("Pets")
	p.PlayedCards().AddCard(cardID, "Pets", "active", []string{"animal", "earth"})
	p.Resources().AddToStorage(cardID, 2)

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 1 {
		t.Fatalf("expected CardVP=1 for Pets with 2 animals, got %d", breakdown.CardVP)
	}
}

func TestPetsVP_FiveAnimals(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	cardID := testutil.CardID("Pets")
	p.PlayedCards().AddCard(cardID, "Pets", "active", []string{"animal", "earth"})
	p.Resources().AddToStorage(cardID, 5)

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 2 {
		t.Fatalf("expected CardVP=2 for Pets with 5 animals, got %d", breakdown.CardVP)
	}
}

// TestNoVPCards verifies that cards without VP conditions don't contribute.
func TestNoVPCards(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	// Deep Well Heating (003) has no vpConditions
	cardID := testutil.CardID("Deep Well Heating")
	p.PlayedCards().AddCard(cardID, "Deep Well Heating", "automated", []string{"building", "power"})

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	if breakdown.CardVP != 0 {
		t.Fatalf("expected CardVP=0 for non-VP card, got %d", breakdown.CardVP)
	}
}

// TestPerColonyVPCard verifies per-colony VP calculation.
// Space Port Colony: 1 VP per 2 colonies in play.
func TestPerColonyVPCard(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	players := g.GetAllPlayers()

	cardID := testutil.CardID("Space Port Colony")
	p.PlayedCards().AddCard(cardID, "Space Port Colony", "automated", []string{"space"})

	// Set up 3 colonies across 2 colony tiles
	g.Colonies().SetStates([]*colony.ColonyState{
		{
			DefinitionID:   "ganymede",
			MarkerPosition: 1,
			PlayerColonies: []string{playerID, players[1].ID()},
		},
		{
			DefinitionID:   "titan",
			MarkerPosition: 1,
			PlayerColonies: []string{playerID},
		},
	})

	breakdown := gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	// 3 colonies / 2 = 1 VP (integer division)
	if breakdown.CardVP != 1 {
		t.Fatalf("expected CardVP=1 for Space Port Colony with 3 colonies, got %d", breakdown.CardVP)
	}

	// Add more colonies to verify scaling
	g.Colonies().SetStates([]*colony.ColonyState{
		{
			DefinitionID:   "ganymede",
			MarkerPosition: 1,
			PlayerColonies: []string{playerID, players[1].ID()},
		},
		{
			DefinitionID:   "titan",
			MarkerPosition: 1,
			PlayerColonies: []string{playerID, players[1].ID()},
		},
	})

	breakdown = gamecards.CalculatePlayerVP(
		p,
		g,
		nil,
		nil,
		g.GetAllPlayers(),
		cardRegistry,
		nil,
		nil,
	)

	// 4 colonies / 2 = 2 VP
	if breakdown.CardVP != 2 {
		t.Fatalf("expected CardVP=2 for Space Port Colony with 4 colonies, got %d", breakdown.CardVP)
	}
}

func TestScenarioVPCountsOnlyPermanentProductionTypes(t *testing.T) {
	g, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	cap := 5
	card := gamecards.Card{ID: "scenario-vp", Name: "Independent economy", Type: gamecards.CardTypeAutomated, VPConditions: []gamecards.VictoryPointCondition{{Amount: 1, Condition: gamecards.VPConditionPer, MaxTrigger: &cap, Per: &shared.PerCondition{ResourceType: shared.ResourcePermanentProductionTypes, Amount: 1}, StateRequirements: []shared.StateRequirement{{State: "transition-complete", Equals: 1}}}}}
	registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
	p.PlayedCards().AddCard(card.ID, card.Name, "automated", nil)
	p.Resources().SetProduction(shared.Production{Credits: -2, Steel: 8, Titanium: 1, Plants: 0, Energy: 0, Heat: 1})
	dep := shared.DependencyGroup{ID: "supplied", State: "supply", Equals: 1, Importable: true}
	p.Resources().AddDependentProduction(shared.ProductionContribution{CardID: "factory", BehaviorID: "supply", Dependency: dep, Resource: shared.ResourceEnergyProduction, Amount: 3})
	score := func() int {
		return gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil).CardVP
	}
	configure := func(occurred int) {
		t.Helper()
		if err := g.ConfigureModeRuntime(shared.ModeRuntimeDefinition{State: map[string]int{"transition-complete": occurred, "supply": 1}}, nil); err != nil {
			t.Fatal(err)
		}
	}
	configure(0)
	if got := score(); got != 0 {
		t.Fatalf("before transition got %d VP", got)
	}
	configure(1)
	if got := score(); got != 3 {
		t.Fatalf("temporary production must not score, got %d VP", got)
	}
	if err := p.Resources().SetDependencyStatus("factory", "supplied", true); err != nil {
		t.Fatal(err)
	}
	if got := score(); got != 4 {
		t.Fatalf("imported energy must score once, got %d VP", got)
	}
	p.Resources().SetProduction(shared.Production{Credits: 1, Steel: 1, Titanium: 1, Plants: 1, Energy: 1, Heat: 1})
	if got := score(); got != 5 {
		t.Fatalf("six types must respect cap5, got %d VP", got)
	}
}
