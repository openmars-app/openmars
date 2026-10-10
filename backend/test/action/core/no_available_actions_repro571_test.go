package core_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
	"openmars/test/testutil"
)

// loadStandardProjectRegistry571 loads the real standard-project registry from the
// JSON asset so HasAvailableActions iterates the same surfaces the game does.
func loadStandardProjectRegistry571(t *testing.T) standardproject.StandardProjectRegistry {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	stdProjPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "assets", "standard_projects.json")
	stdProjData, err := standardproject.LoadStandardProjectsFromJSON(stdProjPath)
	if err != nil {
		t.Fatalf("Failed to load standard projects JSON: %v", err)
	}
	return standardproject.NewInMemoryStandardProjectRegistry(stdProjData)
}

// allStandardProjects is the full set of standard projects a player can perform during
// the action phase, INCLUDING the two resource-conversion "projects"
// (heat->temperature and plants->greenery). The state calculator treats conversions as
// standard projects, so iterating this list also covers the conversion action surface.
var allStandardProjects = []shared.StandardProject{
	shared.StandardProjectSellPatents,
	shared.StandardProjectPowerPlant,
	shared.StandardProjectAsteroid,
	shared.StandardProjectAquifer,
	shared.StandardProjectGreenery,
	shared.StandardProjectCity,
	shared.StandardProjectAirScrapping,
	shared.StandardProjectConvertHeatToTemperature,
	shared.StandardProjectConvertPlantsToGreenery,
}

// setupStuckPlayer builds a single-player game in the ACTION phase where the player has
// been deliberately driven into a state with no possible moves:
//   - empty hand (no card to play, and sell-patents has nothing to sell)
//   - zero of every resource (cannot afford any standard project or conversion)
//   - no played cards with usable actions
//   - colonies disabled (no colony trade surface)
//
// It returns the game, the stuck player, and a card registry that intentionally contains
// only unaffordable cards (so we can ALSO assert the "non-empty hand of unaffordable cards"
// variant in the test).
func setupStuckPlayer(t *testing.T) (*game.Game, *player.Player, gamecards.CardRegistry) {
	t.Helper()

	settings := shared.GameSettings{
		MaxPlayers:      5,
		DevelopmentMode: true,
		GameModeID:      "classic", Features: []string{},
		// Colonies and Venus left disabled: no colony-trade action surface exists,
		// matching a default base game. Base game IS enabled so the base-game standard
		// projects (PowerPlant, etc.) are real surfaces, exactly as in a live game.
	}
	ds, err := datastore.NewDataStore()
	if err != nil {
		t.Fatalf("Failed to create datastore: %v", err)
	}
	g := game.NewGame(ds, "repro-571-game", "player1", settings, board.GenerateMarsBoard(false))

	ctx := context.Background()
	p, err := g.AddNewPlayer(ctx, "player1", "Stuck Player")
	if err != nil {
		t.Fatalf("Failed to add player: %v", err)
	}

	// Card registry containing ONLY cards the player can never afford with zero credits.
	// One has an unmet requirement on top, to exercise that path too.
	testCards := []gamecards.Card{
		{
			ID:   "unaffordable-cheap",
			Name: "Unaffordable Cheap Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 3,
			Tags: []shared.CardTag{shared.TagBuilding},
		},
		{
			ID:   "unaffordable-requirement",
			Name: "Unaffordable + Requirement Card",
			Type: gamecards.CardTypeAutomated,
			Cost: 20,
			Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{
				{
					Type: gamecards.RequirementOxygen,
					Min:  intPtr571(10),
				},
			}},
		},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry(testCards)

	// Drive the player to zero of every basic resource.
	p.Resources().Set(shared.Resources{
		Credits:  0,
		Steel:    0,
		Titanium: 0,
		Plants:   0,
		Energy:   0,
		Heat:     0,
	})

	// No played cards, hence no played-card actions.
	p.Actions().SetActions([]shared.CardAction{})

	// Empty hand.
	p.Hand().SetCards([]string{})

	// Action phase, player's turn, with an action remaining (so "no actions remaining"
	// is NOT the reason moves are unavailable -- the reason is genuinely "nothing to do").
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

func intPtr571(v int) *int { return &v }

// TestNoAvailableActions_PlayerIsStuck_Repro571 demonstrates the state behind issue #571:
// a player whose turn it is during the action phase, with an action remaining, can reach a
// state where NONE of their possible moves is available, yet the engine offers no aggregate
// helper to detect this. The only recourse today is a manual skip/pass.
//
// This test PASSES today. Its value is a runnable fixture proving the scenario is real and
// documenting the missing detection helper, to seed the later fix.
func TestNoAvailableActions_PlayerIsStuck_Repro571(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayer(t)

	// Sanity: it really is the action phase, the player's turn, and they still have an action.
	if g.CurrentPhase() != shared.GamePhaseAction {
		t.Fatalf("expected action phase, got %s", g.CurrentPhase())
	}
	turn := g.CurrentTurn()
	if turn == nil || turn.PlayerID() != p.ID() {
		t.Fatalf("expected it to be the stuck player's turn")
	}
	if turn.ActionsRemaining() <= 0 {
		t.Fatalf("expected the player to still have an action remaining, got %d", turn.ActionsRemaining())
	}

	// --- Surface 1: cards in hand ------------------------------------------------------
	// (Verified with an empty hand below; here we also prove that even a hand full of
	// unaffordable cards yields zero playable cards.)
	p.Hand().SetCards([]string{"unaffordable-cheap", "unaffordable-requirement"})
	for _, cardID := range p.Hand().Cards() {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			t.Fatalf("failed to get card %s: %v", cardID, err)
		}
		state := action.CalculatePlayerCardState(card, p, g, cardRegistry)
		if state.Available() {
			t.Fatalf("expected card %s to be UNAVAILABLE (player has 0 credits), but it was available", cardID)
		}
	}

	// Now reset to the canonical stuck state: an empty hand.
	p.Hand().SetCards([]string{})
	if p.Hand().CardCount() != 0 {
		t.Fatalf("expected empty hand, got %d cards", p.Hand().CardCount())
	}

	// --- Surface 2: played-card actions -----------------------------------------------
	// The player has no played cards, so there are no card actions to use.
	cardActions := p.Actions().List()
	if len(cardActions) != 0 {
		t.Fatalf("expected no played-card actions, got %d", len(cardActions))
	}
	for _, act := range cardActions {
		state := action.CalculatePlayerCardActionState(
			act.CardID, act.Behavior, act.TimesUsedThisGeneration, p, g, cardRegistry,
		)
		if state.Available() {
			t.Fatalf("expected card action on %s to be UNAVAILABLE, but it was available", act.CardID)
		}
	}

	// --- Surface 3: standard projects (incl. conversions) -----------------------------
	// With zero of every resource and an empty hand, every standard project must be
	// unavailable. This is the load-bearing assertion: it covers credit-cost projects,
	// sell-patents (needs cards in hand), and the two resource conversions.
	for _, projectType := range allStandardProjects {
		state := action.CalculatePlayerStandardProjectState(projectType, p, g, cardRegistry)
		if state.Available() {
			t.Fatalf(
				"expected standard project %q to be UNAVAILABLE (player has 0 of everything, empty hand), but it was available; errors=%v",
				projectType, state.Errors,
			)
		}
	}

	// --- Surface 4: milestones and awards ---------------------------------------------
	// Documented for completeness. Claiming a milestone costs 8 credits and funding an
	// award costs credits; with 0 credits both are unaffordable. We assert this via the
	// per-entity calculators over the real milestone/award databases.
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	for _, def := range milestoneRegistry.GetAll() {
		state := action.CalculateMilestoneState(
			shared.MilestoneType(def.ID), p, g, cardRegistry, milestoneRegistry,
		)
		if state.Available() {
			t.Fatalf("expected milestone %q to be UNAVAILABLE (0 credits), but it was available", def.ID)
		}
	}
	awardRegistry := testutil.CreateTestAwardRegistry()
	for _, def := range awardRegistry.GetAll() {
		state := action.CalculateAwardState(shared.AwardType(def.ID), p, g, awardRegistry, cardRegistry)
		if state.Available() {
			t.Fatalf("expected award %q to be UNAVAILABLE (0 credits), but it was available", def.ID)
		}
	}

	// --- The aggregate signal: action.HasAvailableActions -----------------------------
	//
	// Every individual surface above is unavailable, so the aggregate helper must report
	// the player as stuck. This is the load-bearing assertion for #571: a single
	// authoritative "can this player act at all?" boolean, built from the same per-surface
	// calculators exercised above.
	if turn.ActionsRemaining() == 0 {
		t.Fatalf("guard: player should still have actions-remaining (proving 'stuck' != 'out of actions')")
	}

	stdProjRegistry := loadStandardProjectRegistry571(t)

	if action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be FALSE for the canonical stuck state")
	}
}

// TestHasAvailableActions_TrueWhenAffordsStandardProject proves the aggregate flips to
// true the moment the player can reach any single surface -- here, by giving them enough
// credits to afford the PowerPlant standard project.
func TestHasAvailableActions_TrueWhenAffordsStandardProject(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayer(t)
	stdProjRegistry := loadStandardProjectRegistry571(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	if action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("precondition: player should still be stuck before being given credits")
	}

	powerPlantCost := stdProjCreditCost(t, stdProjRegistry, string(shared.StandardProjectPowerPlant))
	p.Resources().Set(shared.Resources{Credits: powerPlantCost})

	if !action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be TRUE once PowerPlant is affordable")
	}
}

// TestHasAvailableActions_TrueWhenPendingSelection proves a player with a pending selection
// is never reported as stuck, even though every other surface is unaffordable.
func TestHasAvailableActions_TrueWhenPendingSelection(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayer(t)
	stdProjRegistry := loadStandardProjectRegistry571(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	if action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("precondition: player should be stuck before a pending selection exists")
	}

	p.Selection().SetPendingCardSelection(&shared.PendingCardSelection{
		AvailableCards: []string{"unaffordable-cheap"},
		MinCards:       1,
		MaxCards:       1,
	})

	if !action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be TRUE while a pending selection is active")
	}
}

// TestHasAvailableActions_TrueWhenForcedFirstActionPending proves an incomplete forced first
// action keeps the player out of the stuck state.
func TestHasAvailableActions_TrueWhenForcedFirstActionPending(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayer(t)
	stdProjRegistry := loadStandardProjectRegistry571(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	if action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("precondition: player should be stuck before a forced first action exists")
	}

	ctx := context.Background()
	if err := g.SetForcedFirstAction(ctx, p.ID(), &shared.ForcedFirstAction{
		State: "resolving",
	}); err != nil {
		t.Fatalf("failed to set forced first action: %v", err)
	}

	if !action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be TRUE while a forced first action is pending")
	}
}

// TestHasAvailableActions_TrueViaColonyTrade proves the colony-trade surface contributes
// availability when colonies are enabled, a fleet is free, a colony is tradeable, and one
// payment type is affordable.
func TestHasAvailableActions_TrueViaColonyTrade(t *testing.T) {
	g, cardRegistry, playerID := setupTradeStateGame(t)
	p, _ := g.GetPlayer(playerID)
	stdProjRegistry := loadStandardProjectRegistry571(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	// Drive every non-trade surface to unavailable: empty hand, no played actions,
	// only enough energy to pay exactly one trade.
	p.Hand().SetCards([]string{})
	p.Actions().SetActions([]shared.CardAction{})
	p.Resources().Set(shared.Resources{Credits: 0, Energy: action.TradeEnergyCost, Titanium: 0})

	if action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("precondition: player should be stuck before a tradeable colony + free fleet exist")
	}

	setTradeableColony(g, "luna")
	g.Colonies().AddTradeFleets(playerID, 1)

	if !action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be TRUE via the colony-trade surface")
	}
}

// TestHasAvailableActions_IgnoresUnselectedMilestone is the regression guard for the
// milestone/award selection bug: every game exposes only the selected subset of
// milestones (start_game selects 5 of the full set), and the per-milestone calculator
// does NOT itself check selection. A player who qualifies for and can afford a milestone
// that is NOT in the selected set must therefore still be reported as stuck -- the
// aggregator must filter by selection exactly like the DTO mapper does, otherwise the
// #571 indicator shows "actions available" while the UI shows no claimable milestone.
func TestHasAvailableActions_IgnoresUnselectedMilestone(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayer(t)
	stdProjRegistry := loadStandardProjectRegistry571(t)
	milestoneRegistry := testutil.CreateTestMilestoneRegistry()
	awardRegistry := testutil.CreateTestAwardRegistry()

	// Awards have no qualification requirement -- funding one only needs credits -- so a
	// player with credits could always fund an award. To isolate the milestone surface,
	// fund the maximum number of awards up front, closing the award surface entirely.
	closeAwardSurface(t, g, awardRegistry)

	// Make the Terraformer milestone genuinely claimable: TR >= 35 satisfies the
	// requirement, and 8 credits covers the claim cost. The per-milestone calculator
	// will now report it as Available. Energy is set just below the PowerPlant cost so
	// no standard project is affordable either.
	p.Resources().SetTerraformRating(35)
	p.Resources().Set(shared.Resources{Credits: 8})

	// Sanity: the milestone really is available at the per-entity level.
	if !action.CalculateMilestoneState(shared.MilestoneType("terraformer"), p, g, cardRegistry, milestoneRegistry).Available() {
		t.Fatalf("precondition: Terraformer should be claimable with TR 35 and 8 credits")
	}

	// Selection EXCLUDES Terraformer. The player can claim it at the calculator level,
	// but it is not in play this game, so the aggregator must still report stuck.
	g.SetSelectedMilestones([]string{"mayor", "gardener", "builder", "planner", "tactician"})

	if action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be FALSE: Terraformer is claimable but NOT selected for this game")
	}

	// Now select Terraformer: the same claimable milestone becomes a real action surface,
	// and the aggregator must flip to true.
	g.SetSelectedMilestones([]string{"terraformer", "mayor", "gardener", "builder", "planner"})

	if !action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry) {
		t.Fatalf("expected HasAvailableActions to be TRUE once Terraformer is in the selected set")
	}
}

// closeAwardSurface funds the maximum number of awards so the award action surface is
// unavailable regardless of the player's credits, letting a test isolate other surfaces.
func closeAwardSurface(t *testing.T, g *game.Game, awardRegistry award.AwardRegistry) {
	t.Helper()
	ctx := context.Background()
	defs := awardRegistry.GetAll()
	if len(defs) < game.MaxFundedAwards {
		t.Fatalf("award registry has fewer than %d awards", game.MaxFundedAwards)
	}
	for i := 0; i < game.MaxFundedAwards; i++ {
		def := defs[i]
		cost := def.GetCostForFundedCount(g.Awards().FundedCount())
		if err := g.Awards().FundAward(ctx, shared.AwardType(def.ID), "filler-player", cost); err != nil {
			t.Fatalf("failed to fund award %s: %v", def.ID, err)
		}
	}
}

// stdProjCreditCost returns the credit cost of a registry-backed standard project.
func stdProjCreditCost(t *testing.T, registry standardproject.StandardProjectRegistry, projectID string) int {
	t.Helper()
	def, err := registry.GetByID(projectID)
	if err != nil {
		t.Fatalf("failed to look up standard project %q: %v", projectID, err)
	}
	return def.CreditCost()
}
