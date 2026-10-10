package core_test

import (
	"context"
	"testing"

	"openmars/internal/action"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// setupTradeStateGame builds a started 1-player game in the action phase with the
// colonies pack enabled and the player's turn active. Trade affordability is then
// shaped per-test via fleet availability, colony states, and resources.
func setupTradeStateGame(t *testing.T) (*game.Game, gamecards.CardRegistry, string) {
	t.Helper()

	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	ctx := context.Background()
	testGame.UpdateSettings(ctx, settings)

	playerID := testGame.GetAllPlayers()[0].ID()
	if err := testGame.UpdatePhase(ctx, shared.GamePhaseAction); err != nil {
		t.Fatalf("Failed to set action phase: %v", err)
	}
	if err := testGame.SetTurnOrder(ctx, []string{playerID}); err != nil {
		t.Fatalf("Failed to set turn order: %v", err)
	}
	if err := testGame.SetCurrentTurn(ctx, playerID, 2); err != nil {
		t.Fatalf("Failed to set current turn: %v", err)
	}

	return testGame, cardRegistry, playerID
}

func setTradeableColony(g *game.Game, colonyID string) {
	states := g.Colonies().States()
	states = append(states, &colony.ColonyState{
		DefinitionID:   colonyID,
		MarkerPosition: 3,
		TradedThisGen:  false,
	})
	g.Colonies().SetStates(states)
}

func addRimFreightersDiscount(t *testing.T, g *game.Game, cardRegistry gamecards.CardRegistry, playerID string) {
	t.Helper()
	p, err := g.GetPlayer(playerID)
	testutil.AssertNoError(t, err, "player should exist")
	card, err := cardRegistry.GetByID(testutil.CardID("Rim Freighters"))
	testutil.AssertNoError(t, err, "Rim Freighters card should exist")
	for behaviorIndex, behavior := range card.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        card.ID,
				CardName:      card.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}
}

func TestCalculateColonyTradeState_UnavailableWhenFleetNotAvailable(t *testing.T) {
	g, cardRegistry, playerID := setupTradeStateGame(t)
	p, _ := g.GetPlayer(playerID)

	setTradeableColony(g, "luna")
	if g.Colonies().TradeFleet(playerID).Available() > 0 {
		testutil.AssertNoError(t, g.Colonies().UseTradeFleet(playerID), "Use fleet")
	}
	p.Resources().Set(shared.Resources{Credits: 9, Energy: 3, Titanium: 3})

	state := action.CalculateColonyTradeState(p, g, cardRegistry)
	if state.Available() {
		t.Fatalf("expected trade UNAVAILABLE when fleet not available, errors=%v", state.Errors)
	}
}

func TestCalculateColonyTradeState_UnavailableWhenNoTradeableColonies(t *testing.T) {
	g, cardRegistry, playerID := setupTradeStateGame(t)
	p, _ := g.GetPlayer(playerID)

	// No colony states added -> nothing tradeable.
	g.Colonies().AddTradeFleets(playerID, 1)
	p.Resources().Set(shared.Resources{Credits: 9, Energy: 3, Titanium: 3})

	state := action.CalculateColonyTradeState(p, g, cardRegistry)
	if state.Available() {
		t.Fatalf("expected trade UNAVAILABLE when no tradeable colonies, errors=%v", state.Errors)
	}
}

func TestCalculateColonyTradeState_UnavailableWhenCannotAffordAny(t *testing.T) {
	g, cardRegistry, playerID := setupTradeStateGame(t)
	p, _ := g.GetPlayer(playerID)

	setTradeableColony(g, "luna")
	g.Colonies().AddTradeFleets(playerID, 1)
	// Below every effective base cost (credits 9, energy 3, titanium 3).
	p.Resources().Set(shared.Resources{Credits: 8, Energy: 2, Titanium: 2})

	state := action.CalculateColonyTradeState(p, g, cardRegistry)
	if state.Available() {
		t.Fatalf("expected trade UNAVAILABLE when no payment type affordable, errors=%v", state.Errors)
	}
}

func TestCalculateColonyTradeState_AvailableWhenAffordableAndTradeable(t *testing.T) {
	g, cardRegistry, playerID := setupTradeStateGame(t)
	p, _ := g.GetPlayer(playerID)

	setTradeableColony(g, "luna")
	g.Colonies().AddTradeFleets(playerID, 1)
	// Cannot afford credits/titanium, but can afford energy (3).
	p.Resources().Set(shared.Resources{Credits: 0, Energy: 3, Titanium: 0})

	state := action.CalculateColonyTradeState(p, g, cardRegistry)
	if !state.Available() {
		t.Fatalf("expected trade AVAILABLE when fleet free, colony tradeable, and one payment affordable, errors=%v", state.Errors)
	}
}

func TestCalculateColonyTradeState_DiscountsReduceEffectiveCost(t *testing.T) {
	g, cardRegistry, playerID := setupTradeStateGame(t)
	p, _ := g.GetPlayer(playerID)

	setTradeableColony(g, "luna")
	g.Colonies().AddTradeFleets(playerID, 1)
	addRimFreightersDiscount(t, g, cardRegistry, playerID)

	state := action.CalculateColonyTradeState(p, g, cardRegistry)

	// Rim Freighters gives -1 to each payment resource.
	testutil.AssertEqual(t, action.TradeCreditsCost-1, state.Cost[string(shared.ResourceCredit)], "credit cost should be discounted by 1")
	testutil.AssertEqual(t, action.TradeEnergyCost-1, state.Cost[string(shared.ResourceEnergy)], "energy cost should be discounted by 1")
	testutil.AssertEqual(t, action.TradeTitaniumCost-1, state.Cost[string(shared.ResourceTitanium)], "titanium cost should be discounted by 1")

	// With the discount, exactly the discounted energy cost (2) is affordable.
	p.Resources().Set(shared.Resources{Credits: 0, Energy: action.TradeEnergyCost - 1, Titanium: 0})
	state = action.CalculateColonyTradeState(p, g, cardRegistry)
	if !state.Available() {
		t.Fatalf("expected trade AVAILABLE at discounted energy cost, errors=%v", state.Errors)
	}
}

func TestCalculateEffectiveTradeCosts_NeverNegative(t *testing.T) {
	g, registry, id := setupTradeStateGame(t)
	p, _ := g.GetPlayer(id)
	discount := shared.NewEffectCondition(shared.ResourceDiscount, 20, "self-player")
	discount.Selectors = []shared.Selector{{Actions: []string{shared.ActionColonyTrade}, Resources: []string{"credit", "energy", "titanium"}}}
	p.Effects().AddEffect(shared.CardEffect{CardID: "test-discount", Behavior: shared.CardBehavior{Outputs: []shared.BehaviorCondition{discount}}})
	costs, _ := action.CalculateEffectiveTradeCosts(p, registry)
	for _, resource := range []string{"credit", "energy", "titanium"} {
		testutil.AssertEqual(t, 0, costs[resource], "Cost is floored at zero")
	}
}
