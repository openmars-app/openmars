package core_test

import (
	"context"
	"testing"

	"openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func setupTestEnvironment(t *testing.T) (*game.Game, *player.Player, gamecards.CardRegistry) {

	// Create test game with default settings
	settings := shared.GameSettings{
		MaxPlayers:      5,
		DevelopmentMode: true,
	}
	ds, _ := datastore.NewDataStore()
	g := game.NewGame(ds, "test-game", "player1", settings, board.GenerateMarsBoard(false))

	// Create and add test player
	ctx := context.Background()
	p, err := g.AddNewPlayer(ctx, "player1", "Test Player")
	if err != nil {
		t.Fatalf("Failed to add player: %v", err)
	}

	// Create minimal card registry for testing
	testCards := []gamecards.Card{
		{
			ID:   "card1",
			Name: "Test Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 10,
			Tags: []shared.CardTag{shared.TagBuilding},
			Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{
				{
					Type: gamecards.RequirementTemperature,
					Min:  testutil.IntPtr(-10),
				},
			}},
		},
		{
			ID:   "card2",
			Name: "Expensive Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 50,
			Tags: []shared.CardTag{shared.TagSpace},
		},
		{
			ID:   "card3",
			Name: "Card With Multiple Requirements",
			Type: gamecards.CardTypeAutomated,
			Cost: 15,
			Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{
				{
					Type: gamecards.RequirementOxygen,
					Min:  testutil.IntPtr(5),
				},
				{
					Type: gamecards.RequirementOceans,
					Min:  testutil.IntPtr(3),
				},
			}},
		},
		{
			ID:   "card-microbe",
			Name: "Microbe Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 6,
			Tags: []shared.CardTag{shared.TagMicrobe},
		},
		{
			ID:   "card-building",
			Name: "Building Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 10,
			Tags: []shared.CardTag{shared.TagBuilding},
		},
		{
			ID:   "card-space",
			Name: "Space Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 12,
			Tags: []shared.CardTag{shared.TagSpace},
		},
		{
			ID:   "card-venus",
			Name: "Venus Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 11,
			Tags: []shared.CardTag{shared.TagVenus},
		},
	}

	cardRegistry := gamecards.NewInMemoryCardRegistry(testCards)

	// Set game to action phase
	if err := g.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
		t.Fatalf("Failed to set game phase: %v", err)
	}

	return g, p, cardRegistry
}

// TestCalculatePlayerCardState_Available verifies that a playable card has no errors
func TestCalculatePlayerCardState_Available(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player enough credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})

	// Set temperature to meet requirement
	ctx := context.Background()
	testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, -10), "set temperature")

	// Get test card
	card, err := cardRegistry.GetByID("card1")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	// Calculate state
	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	// Verify card is available
	if !state.Available() {
		t.Errorf("Expected card to be available, got errors: %+v", state.Errors)
	}

	if len(state.Errors) != 0 {
		t.Errorf("Expected no errors, got %d errors", len(state.Errors))
	}

	if len(state.Cost) == 0 {
		t.Fatal("Expected cost to be set")
	}

	if state.Cost["credit"] != 10 {
		t.Errorf("Expected cost 10, got %d", state.Cost["credit"])
	}
}

// TestCalculatePlayerCardState_InsufficientCredits verifies affordability check
func TestCalculatePlayerCardState_InsufficientCredits(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player insufficient credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 5,
	})

	// Get test card (cost 10)
	card, err := cardRegistry.GetByID("card1")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	// Calculate state
	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	// Verify card is NOT available
	if state.Available() {
		t.Error("Expected card to be unavailable due to insufficient credits")
	}

	// Verify insufficient-credits error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientCredits && err.Category == player.ErrorCategoryCost {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected insufficient-credits error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardState_TemperatureRequirement verifies requirement validation
func TestCalculatePlayerCardState_TemperatureRequirement(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player enough credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})

	// Set temperature BELOW requirement
	ctx := context.Background()
	testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, -20), "set temperature")

	// Get test card (requires temp >= -10)
	card, err := cardRegistry.GetByID("card1")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	// Calculate state
	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	// Verify card is NOT available
	if state.Available() {
		t.Error("Expected card to be unavailable due to temperature requirement")
	}

	// Verify temperature-too-low error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeTemperatureTooLow && err.Category == player.ErrorCategoryRequirement {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected temperature-too-low error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardState_MultipleRequirements verifies multiple requirement checks
func TestCalculatePlayerCardState_MultipleRequirements(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player enough credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})

	// Set global parameters to NOT meet requirements
	ctx := context.Background()
	testutil.AssertNoError(t, g.GlobalParameters().SetOxygen(ctx, 2), "set oxygen")
	_, err := g.GlobalParameters().PlaceOcean(ctx, "") // 1 ocean, need 3
	testutil.AssertNoError(t, err, "place ocean 1")
	_, err = g.GlobalParameters().PlaceOcean(ctx, "") // 2 oceans, need 3
	testutil.AssertNoError(t, err, "place ocean 2")

	// Get test card (requires oxygen >= 5 AND oceans >= 3)
	card, err := cardRegistry.GetByID("card3")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	// Calculate state
	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	// Verify card is NOT available
	if state.Available() {
		t.Error("Expected card to be unavailable due to multiple unmet requirements")
	}

	// Verify we have multiple errors (oxygen AND oceans)
	if len(state.Errors) < 2 {
		t.Errorf("Expected at least 2 errors, got %d: %+v", len(state.Errors), state.Errors)
	}

	// Check for both error types
	hasOxygenError := false
	hasOceanError := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeOxygenTooLow {
			hasOxygenError = true
		}
		if err.Code == player.ErrorCodeOceansTooLow {
			hasOceanError = true
		}
	}

	if !hasOxygenError {
		t.Error("Expected oxygen-too-low error")
	}
	if !hasOceanError {
		t.Error("Expected oceans-too-low error")
	}
}

// TestCalculatePlayerCardState_WrongPhase verifies phase validation
func TestCalculatePlayerCardState_WrongPhase(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})

	// Set game to WRONG phase (waiting for game start instead of action)
	ctx := context.Background()
	if err := g.UpdatePhase(ctx, shared.GamePhaseWaitingForGameStart); err != nil {
		t.Fatalf("Failed to set game phase: %v", err)
	}

	// Get test card
	card, err := cardRegistry.GetByID("card1")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	// Calculate state
	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	// Verify card is NOT available
	if state.Available() {
		t.Error("Expected card to be unavailable due to wrong phase")
	}

	// Verify wrong-phase error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeWrongPhase && err.Category == player.ErrorCategoryPhase {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected wrong-phase error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardActionState_Available verifies usable action
func TestCalculatePlayerCardActionState_Available(t *testing.T) {
	g, p, _ := setupTestEnvironment(t)

	// Set current turn to this player
	ctx := context.Background()
	if err := g.SetCurrentTurn(ctx, "player1", 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	// Give player resources for action inputs
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 10,
	})

	// Create test action with input requirements
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, ""),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceSteel, 2, ""),
		},
	}

	// Calculate state
	state := action.CalculatePlayerCardActionState("card1", behavior, 0, p, g)

	// Verify action is available
	if !state.Available() {
		t.Errorf("Expected action to be available, got errors: %+v", state.Errors)
	}

	if len(state.Errors) != 0 {
		t.Errorf("Expected no errors, got %d errors", len(state.Errors))
	}
}

// TestCalculatePlayerCardActionState_InsufficientResources verifies input check
func TestCalculatePlayerCardActionState_InsufficientResources(t *testing.T) {
	g, p, _ := setupTestEnvironment(t)

	// Set current turn to this player
	ctx := context.Background()
	if err := g.SetCurrentTurn(ctx, "player1", 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	// Give player INSUFFICIENT resources
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 2, // Need 4
	})

	// Create test action requiring 4 energy
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, ""),
		},
	}

	// Calculate state
	state := action.CalculatePlayerCardActionState("card1", behavior, 0, p, g)

	// Verify action is NOT available
	if state.Available() {
		t.Error("Expected action to be unavailable due to insufficient resources")
	}

	// Verify insufficient-resources error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientResources && err.Category == player.ErrorCategoryInput {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected insufficient-resources error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardActionState_NotPlayerTurn verifies turn check
func TestCalculatePlayerCardActionState_NotPlayerTurn(t *testing.T) {
	g, p, _ := setupTestEnvironment(t)

	// Set current turn to DIFFERENT player
	ctx := context.Background()
	if err := g.SetCurrentTurn(ctx, "player2", 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	// Create test action
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
	}

	// Calculate state
	state := action.CalculatePlayerCardActionState("card1", behavior, 0, p, g)

	// Verify action is NOT available
	if state.Available() {
		t.Error("Expected action to be unavailable because it's not player's turn")
	}

	// Verify not-your-turn error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeNotYourTurn && err.Category == player.ErrorCategoryTurn {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected not-your-turn error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerStandardProjectState_Available verifies affordable project
func TestCalculatePlayerStandardProjectState_Available(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player enough credits for asteroid (cost 14)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})

	// Calculate state for asteroid project
	state := action.CalculatePlayerStandardProjectState(
		shared.StandardProjectAsteroid,
		p,
		g,
		cardRegistry,
	)

	// Verify project is available
	if !state.Available() {
		t.Errorf("Expected project to be available, got errors: %+v", state.Errors)
	}

	if len(state.Errors) != 0 {
		t.Errorf("Expected no errors, got %d errors", len(state.Errors))
	}

	expectedCost := shared.StandardProjectCost[shared.StandardProjectAsteroid]
	if len(state.Cost) == 0 {
		t.Fatal("Expected cost to be set")
	}
	if state.Cost["credit"] != expectedCost {
		t.Errorf("Expected cost %d, got %d", expectedCost, state.Cost["credit"])
	}
}

// TestCalculatePlayerStandardProjectState_InsufficientCredits verifies affordability
func TestCalculatePlayerStandardProjectState_InsufficientCredits(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player insufficient credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 5,
	})

	// Calculate state for asteroid project (cost 14)
	state := action.CalculatePlayerStandardProjectState(
		shared.StandardProjectAsteroid,
		p,
		g,
		cardRegistry,
	)

	// Verify project is NOT available
	if state.Available() {
		t.Error("Expected project to be unavailable due to insufficient credits")
	}

	// Verify insufficient-credits error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientCredits && err.Category == player.ErrorCategoryCost {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected insufficient-credits error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerStandardProjectState_NoOceansRemaining verifies ocean availability
func TestCalculatePlayerStandardProjectState_NoOceansRemaining(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player enough credits
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})

	// Place all 9 oceans
	ctx := context.Background()
	for i := 0; i < 9; i++ {
		if _, err := g.GlobalParameters().PlaceOcean(ctx, ""); err != nil {
			t.Fatalf("Failed to place ocean: %v", err)
		}
	}

	// Calculate state for aquifer project
	state := action.CalculatePlayerStandardProjectState(
		shared.StandardProjectAquifer,
		p,
		g,
		cardRegistry,
	)

	// Verify project is NOT available
	if state.Available() {
		t.Error("Expected aquifer project to be unavailable when no oceans remaining")
	}

	// Verify no-ocean-tiles error exists
	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeNoOceanTiles && err.Category == player.ErrorCategoryAvailability {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected no-ocean-tiles error, got errors: %+v", state.Errors)
	}

	// Verify metadata shows oceansRemaining = 0
	if oceansRemaining, ok := state.Metadata["oceansRemaining"].(int); !ok || oceansRemaining != 0 {
		t.Errorf("Expected oceansRemaining=0 in metadata, got %v", state.Metadata["oceansRemaining"])
	}
}

// TestCalculatePlayerCardState_TitaniumNotCountedForNonSpaceCard verifies that titanium
// is not counted as payment for cards without the Space tag.
// This was a real bug: Phobolog with 8 titanium showed Archaebacteria (Microbe, cost 6)
// as playable because titanium was unconditionally added to effective credits.
func TestCalculatePlayerCardState_TitaniumNotCountedForNonSpaceCard(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player NO credits but lots of titanium (like Phobolog)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceTitanium: 8,
	})

	// Get microbe card (cost 6, no Space tag)
	card, err := cardRegistry.GetByID("card-microbe")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	if state.Available() {
		t.Error("Expected card to be unavailable: titanium should not count for non-Space card")
	}

	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientCredits && err.Category == player.ErrorCategoryCost {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected insufficient-credits error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardState_SteelNotCountedForNonBuildingCard verifies that steel
// is not counted as payment for cards without the Building tag.
func TestCalculatePlayerCardState_SteelNotCountedForNonBuildingCard(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player NO credits but lots of steel
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceSteel: 10,
	})

	// Get space card (cost 12, Space tag but no Building tag)
	card, err := cardRegistry.GetByID("card-space")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	if state.Available() {
		t.Error("Expected card to be unavailable: steel should not count for non-Building card")
	}

	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientCredits && err.Category == player.ErrorCategoryCost {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected insufficient-credits error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardState_SteelCountedForBuildingCard verifies that steel
// IS counted as payment for cards with the Building tag.
func TestCalculatePlayerCardState_SteelCountedForBuildingCard(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player NO credits but enough steel (5 steel * 2 MC = 10 MC, card costs 10)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceSteel: 5,
	})

	// Set temperature to meet requirement for card1 (Building tag, cost 10, requires temp >= -10)
	ctx := context.Background()
	testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, -10), "set temperature")

	card, err := cardRegistry.GetByID("card-building")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	if !state.Available() {
		t.Errorf("Expected card to be available: steel should count for Building card, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardState_TitaniumCountedForSpaceCard verifies that titanium
// IS counted as payment for cards with the Space tag.
func TestCalculatePlayerCardState_TitaniumCountedForSpaceCard(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player NO credits but enough titanium (4 titanium * 3 MC = 12 MC, card costs 12)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceTitanium: 4,
	})

	card, err := cardRegistry.GetByID("card-space")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}

	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	if !state.Available() {
		t.Errorf("Expected card to be available: titanium should count for Space card, got errors: %+v", state.Errors)
	}
}

func TestCalculateChoiceErrors_NoRequirements(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	choice := shared.Choice{
		Outputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	}

	errors := action.CalculateChoiceErrors(choice, p, g, cardRegistry)
	if len(errors) != 0 {
		t.Errorf("Choice without requirements should always be available, got %d errors", len(errors))
	}
}

func TestCalculateChoiceErrors_TagRequirementNotMet(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	choice := shared.Choice{
		Outputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 3, "self-player"),
		},
		Requirements: &shared.ChoiceRequirements{
			Items: []shared.ChoiceRequirement{
				{Type: "tags", Min: testutil.IntPtr(3), Tag: testutil.TagPtr(shared.TagVenus)},
			},
		},
	}

	errors := action.CalculateChoiceErrors(choice, p, g, cardRegistry)
	if len(errors) == 0 {
		t.Errorf("Choice with 3+ venus tag requirement should fail when player has 0 venus tags")
	}
}

func TestCalculateChoiceErrors_TagRequirementMet(t *testing.T) {
	g, p, _ := setupTestEnvironment(t)

	p.PlayedCards().AddCard("venus-1", "Venus 1", "automated", []string{"venus"})
	p.PlayedCards().AddCard("venus-2", "Venus 2", "automated", []string{"venus"})
	p.PlayedCards().AddCard("venus-3", "Venus 3", "automated", []string{"venus"})

	venusCards := []gamecards.Card{
		{ID: "venus-1", Name: "Venus 1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus}},
		{ID: "venus-2", Name: "Venus 2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus}},
		{ID: "venus-3", Name: "Venus 3", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus}},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(venusCards)

	choice := shared.Choice{
		Outputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 3, "self-player"),
		},
		Requirements: &shared.ChoiceRequirements{
			Items: []shared.ChoiceRequirement{
				{Type: "tags", Min: testutil.IntPtr(3), Tag: testutil.TagPtr(shared.TagVenus)},
			},
		},
	}

	errors := action.CalculateChoiceErrors(choice, p, g, cardRegistry)
	if len(errors) != 0 {
		t.Errorf("Choice with 3+ venus tag requirement should pass when player has 3 venus tags, got %d errors: %+v", len(errors), errors)
	}
}

// TestCalculatePlayerCardState_StoragePaymentSubstituteCountedForMatchingCard verifies that
// storage payment substitutes (e.g., Dirigibles floaters) are counted for cards with matching tags.
func TestCalculatePlayerCardState_StoragePaymentSubstituteCountedForMatchingCard(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player 8 credits (not enough alone for cost 11)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 8,
	})

	// Simulate Dirigibles played: register storage payment substitute for Venus cards
	dirigiblesID := "dirigibles-played"
	cardRegistry = gamecards.NewInMemoryCardRegistry(append(cardRegistry.GetAll(), gamecards.Card{ID: dirigiblesID, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater}}))
	p.PlayedCards().AddCard(dirigiblesID, "Dirigibles", "active", []string{"venus"})
	p.Resources().AddToStorage(dirigiblesID, 1) // 1 floater = 3 MC
	p.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: shared.PaymentSource{Target: "self-card", CardID: dirigiblesID, Resource: shared.ResourceFloater},
		ConversionRate: 3,
		Selectors:      []shared.Selector{{Tags: []shared.CardTag{shared.TagVenus}}},
		TargetResource: shared.ResourceCredit})

	// Get venus card (cost 11) — 8 credits + 1 floater * 3 = 11 >= 11
	card, err := cardRegistry.GetByID("card-venus")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}
	p.Hand().AddCard(card.ID)

	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	if !state.Available() {
		t.Errorf("Expected card to be available: 8 credits + 1 floater (3 MC) = 11 should cover cost 11, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardState_StoragePaymentSubstituteNotCountedForNonMatchingCard verifies that
// storage payment substitutes are NOT counted for cards without matching tags.
func TestCalculatePlayerCardState_StoragePaymentSubstituteNotCountedForNonMatchingCard(t *testing.T) {
	g, p, cardRegistry := setupTestEnvironment(t)

	// Give player 3 credits (not enough for cost 6)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 3,
	})

	// Simulate Dirigibles played: register storage payment substitute for Venus cards only
	dirigiblesID := "dirigibles-played"
	cardRegistry = gamecards.NewInMemoryCardRegistry(append(cardRegistry.GetAll(), gamecards.Card{ID: dirigiblesID, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater}}))
	p.PlayedCards().AddCard(dirigiblesID, "Dirigibles", "active", []string{"venus"})
	p.Resources().AddToStorage(dirigiblesID, 3) // 3 floaters = 9 MC (but only for Venus)
	p.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: shared.PaymentSource{Target: "self-card", CardID: dirigiblesID, Resource: shared.ResourceFloater},
		ConversionRate: 3,
		Selectors:      []shared.Selector{{Tags: []shared.CardTag{shared.TagVenus}}},
		TargetResource: shared.ResourceCredit})

	// Get microbe card (cost 6, no Venus tag) — floaters should NOT apply
	card, err := cardRegistry.GetByID("card-microbe")
	if err != nil {
		t.Fatalf("Failed to get card: %v", err)
	}
	p.Hand().AddCard(card.ID)

	state := action.CalculatePlayerCardState(card, p, g, cardRegistry)

	if state.Available() {
		t.Error("Expected card to be unavailable: Dirigibles floaters should not count for non-Venus card")
	}

	found := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientCredits && err.Category == player.ErrorCategoryCost {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected insufficient-credits error, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardActionState_ProductionInput_Available verifies production inputs check player production
func TestCalculatePlayerCardActionState_ProductionInput_Available(t *testing.T) {
	g, p, _ := setupTestEnvironment(t)

	ctx := context.Background()
	if err := g.SetCurrentTurn(ctx, "player1", 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})

	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceTR, 1, "self-player"),
		},
	}

	state := action.CalculatePlayerCardActionState("card1", behavior, 0, p, g)

	if !state.Available() {
		t.Errorf("Expected action to be available with sufficient energy production, got errors: %+v", state.Errors)
	}
}

// TestCalculatePlayerCardActionState_ProductionInput_Insufficient verifies production inputs fail when production is 0
func TestCalculatePlayerCardActionState_ProductionInput_Insufficient(t *testing.T) {
	g, p, _ := setupTestEnvironment(t)

	ctx := context.Background()
	if err := g.SetCurrentTurn(ctx, "player1", 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceTR, 1, "self-player"),
		},
	}

	state := action.CalculatePlayerCardActionState("card1", behavior, 0, p, g)

	if state.Available() {
		t.Error("Expected action to be unavailable with 0 energy production")
	}

	prodFound := false
	for _, err := range state.Errors {
		if err.Code == player.ErrorCodeInsufficientResources && err.Category == player.ErrorCategoryInput {
			prodFound = true
			break
		}
	}
	if !prodFound {
		t.Errorf("Expected insufficient-resources error for production input, got errors: %+v", state.Errors)
	}
}
