package card_packs_test

import (
	"context"
	"fmt"
	"openmars/internal/action/admin"
	"openmars/internal/action/turn_management"
	gamecards "openmars/internal/game/cards"
	"slices"
	"testing"

	cardAction "openmars/internal/action/card"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// =============================================================================
// P01 - Allied Bank (tags: earth)
// "Increase your M€ production 4 steps. Gain 3 M€."
// =============================================================================
func TestPrelude_AlliedBank_ProductionAndCredits(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Allied Bank")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Allied Bank should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Allied Bank should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, creditsBefore+3, creditsAfter, "Should gain 3 M€")
	testutil.AssertEqual(t, prodBefore.Credits+4, prodAfter.Credits, "M€ production should increase by 4")
}

// =============================================================================
// P02 - Aquifer Turbines (tags: power)
// "Place an ocean tile. Increase your energy production 2 steps. Remove 3 M€."
// =============================================================================
func TestPrelude_AquiferTurbines_CreditsEnergyProductionAndOceanTile(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Aquifer Turbines")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Aquifer Turbines should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Aquifer Turbines should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, creditsBefore-3, creditsAfter, "Should lose 3 M€")
	testutil.AssertEqual(t, prodBefore.Energy+2, prodAfter.Energy, "Energy production should increase by 2")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending ocean tile selection")
}

// =============================================================================
// P03 - Biofuels (tags: microbe)
// "Increase your plant production and energy production 1 step each. Gain 2 plants."
// =============================================================================
func TestPrelude_Biofuels_PlantsAndProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Biofuels")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Biofuels should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Biofuels should be in played cards")
	plantsAfter := p.Resources().Get().Plants
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy, "Energy production should increase by 1")
}

// =============================================================================
// P04 - Biolab (tags: science)
// "Increase your plant production 1 step. Draw 3 cards."
// =============================================================================
func TestPrelude_Biolab_PlantProductionAndCardDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Biolab")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	handSizeBefore := p.Hand().CardCount()
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Biolab should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Biolab should be in played cards")
	handSizeAfter := p.Hand().CardCount()
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	testutil.AssertEqual(t, handSizeBefore+3-1, handSizeAfter, "Should draw 3 cards (minus the played prelude)")
}

// =============================================================================
// P05: Biosphere Support (prelude, cost 0, tags: [plant])
// "Decrease your M€ production 1 step. Increase your plant production 2 steps."
// Behavior: auto trigger, outputs: credit-production -1, plant-production +2
// =============================================================================
func TestPrelude_BiosphereSupport_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Biosphere Support")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Biosphere Support should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Biosphere Support should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-1, prodAfter.Credits, "M€ production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants, "Plant production should increase by 2")
}

// =============================================================================
// P06: Business Empire (prelude, cost 0, tags: [earth])
// "Increase your M€ production 6 steps. Remove 6 M€."
// Behavior: auto trigger, outputs: credit -6, credit-production +6
// =============================================================================
func TestPrelude_BusinessEmpire_RemovesCreditsAndIncreasesProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Business Empire")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Business Empire should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Business Empire should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, creditsBefore-6, creditsAfter, "Credits should decrease by 6")
	testutil.AssertEqual(t, prodBefore.Credits+6, prodAfter.Credits, "M€ production should increase by 6")
}

// =============================================================================
// P07: Dome Farming (prelude, cost 0, tags: [building, plant])
// "Increase your plant production 1 step. Increase your M€ production 2 steps."
// Behavior: auto trigger, outputs: credit-production +2, plant-production +1
// =============================================================================
func TestPrelude_DomeFarming_IncreasesProductionBoth(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Dome Farming")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Dome Farming should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Dome Farming should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits, "M€ production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
}

// =============================================================================
// P08: Donation (prelude, cost 0, no tags)
// "Gain 21 M€."
// Behavior: auto trigger, outputs: credit +21
// =============================================================================
func TestPrelude_Donation_GainsCredits(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Donation")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Donation should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Donation should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore+21, creditsAfter, "Credits should increase by 21")
}

// =============================================================================
// P09: Early Settlement (prelude, cost 0, tags: [building, city])
// "Place a city tile. Increase your plant production 1 step."
// Behavior: auto trigger, outputs: plant-production +1, city-placement
// =============================================================================
func TestPrelude_EarlySettlement_PlantProductionAndCity(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Early Settlement")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Early Settlement should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Early Settlement should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// =============================================================================
// P10: Ecology Experts (prelude, cost 0, tags: [microbe, plant])
// "Increase your plant production 1 step. Play a card from hand, ignoring global requirements"
// Behavior: auto trigger, outputs: plant-production +1
// =============================================================================
func TestPrelude_EcologyExperts_PlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Ecology Experts")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ecology Experts should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Ecology Experts should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
}

func TestPrelude_EcologyExperts_PlayCardIgnoringRequirements(t *testing.T) {
	// Trees (060) requires temperature >= -4°C, game starts at -30°C
	// After playing Ecology Experts, Trees should succeed despite unmet temperature requirement
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	ecologyExperts := testutil.GetCardByName("Ecology Experts")
	trees := testutil.GetCardByName("Trees")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(ecologyExperts.ID)
	p.Hand().AddCard(trees.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), ecologyExperts.ID, shared.Payment{Allocations: []shared.PaymentAllocation{}}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ecology Experts should play successfully")
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), trees.ID, shared.NativePayment(shared.ResourceCredit, 13), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Trees should play successfully when ignoring global requirements")
	testutil.AssertTrue(t, p.PlayedCards().Contains(trees.ID), "Trees should be in played cards")
	testutil.AssertEqual(t, plantsBefore+1, p.Resources().Get().Plants, "Should gain 1 plant")
	testutil.AssertEqual(t, prodBefore.Plants+3, p.Resources().Production().Plants, "Plant production should increase by 3")
}

// =============================================================================
// P11: Excentric Sponsor (prelude, cost 0, no tags)
// "Play a card from hand, reducing its cost by 25 M€"
// The benefit is tied to a single immediate project play, not spendable credits.
func TestPrelude_ExcentricSponsor_DiscountedPlay(t *testing.T) {
	for _, cost := range []int{10, 25, 40} {
		t.Run(fmt.Sprint(cost), func(t *testing.T) {
			g, repo := testutil.CreateTestGameWithPlayers(t, 1)
			ctx := context.Background()
			p := g.GetAllPlayers()[0]
			project := gamecards.Card{ID: "sponsored-project", Name: "Sponsored project", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: cost}
			registry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{project})
			testutil.AssertNoError(t, g.UpdateStatus(ctx, shared.GameStatusActive), "active")
			testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseInitApplyPrelude), "phase")
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, p.ID(), 2), "turn")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetTurnOrder(ctx, []string{p.ID()}), "turn order")
			p.Hand().AddCard(project.ID)
			testutil.AssertNoError(t, turn_management.ApplyPreludeCard(ctx, g, p, testutil.CardID("Excentric Sponsor"), registry, nil, testutil.TestLogger()), "apply sponsor")
			testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "no cash grant")
			testutil.AssertTrue(t, p.Selection().GetPendingCardPlaySelection() != nil, "pending play")
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			payable := max(0, cost-25)
			if payable > 0 {
				err := play.Execute(ctx, g.ID(), p.ID(), project.ID, shared.NativePayment(shared.ResourceCredit, payable-1), nil, nil, nil, nil, nil)
				testutil.AssertTrue(t, err != nil, "underpayment rejected")
				testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "failed payment spends nothing")
				testutil.AssertTrue(t, p.Selection().GetPendingCardPlaySelection() != nil && p.Hand().HasCard(project.ID), "failed payment preserves selection and hand")
			}
			payment := shared.NativePayment(shared.ResourceCredit, payable)
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), p.ID(), project.ID, payment, nil, nil, nil, nil, nil), "discounted play")
			testutil.AssertEqual(t, 100-payable, p.Resources().Get().Credits, "discount clamps without remainder")
			testutil.AssertTrue(t, p.Selection().GetPendingCardPlaySelection() == nil, "selection consumed")
			testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "forced play does not consume action")
			testutil.AssertTrue(t, play.Execute(ctx, g.ID(), p.ID(), project.ID, payment, nil, nil, nil, nil, nil) != nil, "duplicate rejected")
		})
	}
}

func TestPrelude_ExcentricSponsor_ProjectRetainsTileContinuation(t *testing.T) {
	g, repo := testutil.CreateTestGameWithPlayers(t, 1)
	ctx := context.Background()
	p := g.GetAllPlayers()[0]
	registry := testutil.CreateTestCardRegistry()
	testutil.AssertNoError(t, g.UpdateStatus(ctx, shared.GameStatusActive), "active")
	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseInitApplyPrelude), "phase")
	testutil.AssertNoError(t, g.SetTurnOrder(ctx, []string{p.ID()}), "turn order")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, p.ID(), 2), "turn")
	id := testutil.CardID("Ganymede Colony")
	p.Hand().AddCard(id)
	testutil.AssertNoError(t, turn_management.ApplyPreludeCard(ctx, g, p, testutil.CardID("Excentric Sponsor"), registry, nil, testutil.TestLogger()), "apply sponsor")
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), p.ID(), id, shared.Payment{}, nil, nil, nil, nil, nil), "play sponsored city")
	testutil.AssertTrue(t, p.Selection().GetPendingCardPlaySelection() == nil, "sponsor play complete")
	testutil.AssertTrue(t, g.GetPendingTileSelection(p.ID()) != nil, "normal city placement remains pending")
	testutil.AssertEqual(t, shared.GamePhaseInitApplyPrelude, g.CurrentPhase(), "init waits for tile placement")
	testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "no turn action consumed")
}

func TestPrelude_ExcentricSponsor_NoLegalProjectRefund(t *testing.T) {
	g, _ := testutil.CreateTestGameWithPlayers(t, 1)
	p := g.GetAllPlayers()[0]
	before := p.Resources().Get().Credits
	id := testutil.CardID("Excentric Sponsor")
	testutil.AssertNoError(t, turn_management.ApplyPreludeCard(context.Background(), g, p, id, testutil.CreateTestCardRegistry(), nil, testutil.TestLogger()), "apply unplayable prelude")
	testutil.AssertEqual(t, before+15, p.Resources().Get().Credits, "unplayable prelude refund")
	testutil.AssertTrue(t, !p.PlayedCards().Contains(id), "unplayable prelude is not played")
	testutil.AssertTrue(t, p.Selection().GetPendingCardPlaySelection() == nil, "no impossible prompt")
}

// =============================================================================
// P12: Experimental Forest (prelude, cost 0, tags: [plant])
// "Place a greenery tile and increase oxygen 1 step. Reveal cards from the deck until you
// have revealed 2 plant-tag cards. Take these into your hand, and discard the rest."
// Behavior: auto trigger, outputs: greenery-placement, card-draw 2
// =============================================================================
func TestPrelude_ExperimentalForest_GreeneryAndCardDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Experimental Forest")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Experimental Forest should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Experimental Forest should be in played cards")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending greenery tile selection")

	// Should have drawn exactly 2 cards (all with plant tag)
	drawnCards := p.Hand().Cards()
	testutil.AssertEqual(t, 2, len(drawnCards), "Should have 2 cards in hand (drew 2 plant-tag cards, played 1 prelude)")
	for _, cardID := range drawnCards {
		drawnCard, err := cardRegistry.GetByID(cardID)
		testutil.AssertNoError(t, err, "Drawn card should exist in registry")
		testutil.AssertTrue(t, slices.Contains(drawnCard.Tags, shared.TagPlant), "Drawn card "+drawnCard.Name+" should have plant tag")
	}
}

// --- P13: Galilean Mining (tags: jovian) ---
// "Increase your titanium production 2 steps. Remove 5 M€."
func TestPrelude_GalileanMining_TitaniumProductionAndCostRemoval(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Galilean Mining")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Galilean Mining should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Galilean Mining should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, creditsBefore-5, creditsAfter, "Should lose 5 M€")
	testutil.AssertEqual(t, prodBefore.Titanium+2, prodAfter.Titanium, "Titanium production should increase by 2")
}

// --- P14: Great Aquifer (no tags) ---
// "Place 2 ocean tiles."
func TestPrelude_GreatAquifer_PendingOceanTileSelection(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Great Aquifer")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Great Aquifer should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Great Aquifer should be in played cards")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending ocean tile selection after playing Great Aquifer")
}

// --- P15: Huge Asteroid (no tags) ---
// "Raise temperature 3 steps. Remove 5 M€."
func TestPrelude_HugeAsteroid_TemperatureAndCostRemoval(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Huge Asteroid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Huge Asteroid should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Huge Asteroid should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, creditsBefore-5, creditsAfter, "Should lose 5 M€")
	testutil.AssertEqual(t, tempBefore+6, tempAfter, "Temperature should increase by 3 steps (6 degrees)")
}

// --- P16: Io Research Outpost (tags: jovian, science) ---
// "Increase your titanium production 1 step. Draw 1 card."
func TestPrelude_IoResearchOutpost_TitaniumProductionAndCardDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Io Research Outpost")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	handSizeBefore := len(p.Hand().Cards())
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Io Research Outpost should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Io Research Outpost should be in played cards")
	handSizeAfter := len(p.Hand().Cards())
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium, "Titanium production should increase by 1")
	// Playing the card removes it from hand (-1), drawing 1 card adds 1, net change = 0
	testutil.AssertEqual(t, handSizeBefore, handSizeAfter, "Hand size should be unchanged (played 1, drew 1)")
}

// --- P17: Loan (no tags) ---
// "Decrease your M€ production 2 steps. Gain 30 M€."
func TestPrelude_Loan_ProductionDecreaseAndCredits(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Loan")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Loan should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Loan should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, creditsBefore+30, creditsAfter, "Should gain 30 M€")
	testutil.AssertEqual(t, prodBefore.Credits-2, prodAfter.Credits, "M€ production should decrease by 2")
}

// --- P18: Martian Industries (tags: building) ---
// "Increase your energy production and steel production 1 step each. Gain 6 M€."
func TestPrelude_MartianIndustries_CreditsAndProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Martian Industries")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Martian Industries should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Martian Industries should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, creditsBefore+6, creditsAfter, "Should gain 6 M€")
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy, "Energy production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Steel+1, prodAfter.Steel, "Steel production should increase by 1")
}

// --- P19: Metal-Rich Asteroid (no tags) ---
// "Raise temperature 1 step. Gain 4 titanium, and 4 steel."
func TestPrelude_MetalRichAsteroid_TemperatureAndResources(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Metal-Rich Asteroid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	resourcesBefore := p.Resources().Get()
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Metal-Rich Asteroid should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Metal-Rich Asteroid should be in played cards")
	resourcesAfter := p.Resources().Get()
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+2, tempAfter, "Temperature should increase by 1 step (2 degrees)")
	testutil.AssertEqual(t, resourcesBefore.Titanium+4, resourcesAfter.Titanium, "Should gain 4 titanium")
	testutil.AssertEqual(t, resourcesBefore.Steel+4, resourcesAfter.Steel, "Should gain 4 steel")
}

// --- P20: Metals Company (no tags) ---
// "Increase your M€ production, steel production, and titanium production 1 step each."
func TestPrelude_MetalsCompany_AllProductionIncrease(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Metals Company")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Metals Company should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Metals Company should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits, "M€ production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Steel+1, prodAfter.Steel, "Steel production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium, "Titanium production should increase by 1")
}

// =============================================================================
// P21: Mining Operations (prelude, cost 0, tags: [building])
// "Increase your steel production 2 steps. Gain 4 steel."
// Behavior: auto trigger, outputs: steel +4, steel-production +2
// =============================================================================
func TestPrelude_MiningOperations_SteelProductionAndGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Operations")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	steelBefore := p.Resources().Get().Steel
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mining Operations should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Mining Operations should be in played cards")
	steelAfter := p.Resources().Get().Steel
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, steelBefore+4, steelAfter, "Should gain 4 steel")
	testutil.AssertEqual(t, prodBefore.Steel+2, prodAfter.Steel, "Steel production should increase by 2")
}

// =============================================================================
// P22: Mohole (prelude, cost 0, tags: [building])
// "Increase your heat production 3 steps. Gain 3 heat."
// Behavior: auto trigger, outputs: heat +3, heat-production +3
// =============================================================================
func TestPrelude_Mohole_HeatProductionAndGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mohole")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	heatBefore := p.Resources().Get().Heat
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mohole should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Mohole should be in played cards")
	heatAfter := p.Resources().Get().Heat
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, heatBefore+3, heatAfter, "Should gain 3 heat")
	testutil.AssertEqual(t, prodBefore.Heat+3, prodAfter.Heat, "Heat production should increase by 3")
}

// =============================================================================
// P23: Mohole Excavation (prelude, cost 0, tags: [building])
// "Increase your steel production 1 step, and your heat production 2 steps. Gain 2 heat."
// Behavior: auto trigger, outputs: heat +2, steel-production +1, heat-production +2
// =============================================================================
func TestPrelude_MoholeExcavation_SteelAndHeatProductionAndHeatGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mohole Excavation")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	heatBefore := p.Resources().Get().Heat
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mohole Excavation should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Mohole Excavation should be in played cards")
	heatAfter := p.Resources().Get().Heat
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, heatBefore+2, heatAfter, "Should gain 2 heat")
	testutil.AssertEqual(t, prodBefore.Steel+1, prodAfter.Steel, "Steel production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat, "Heat production should increase by 2")
}

// =============================================================================
// P24: Nitrogen Shipment (prelude, cost 0, no tags)
// "Raise your terraform rating 1 step. Increase your plant production 1 step. Gain 5 M€."
// Behavior: auto trigger, outputs: credit +5, plant-production +1, tr +1
// =============================================================================
func TestPrelude_NitrogenShipment_TRAndPlantProductionAndCredits(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Nitrogen Shipment")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Nitrogen Shipment should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Nitrogen Shipment should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	prodAfter := p.Resources().Production()
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, creditsBefore+5, creditsAfter, "Credits should increase by 5")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	testutil.AssertEqual(t, trBefore+1, trAfter, "Terraform rating should increase by 1")
}

// =============================================================================
// P25 - Orbital Construction Yard (tags: space)
// "Increase your titanium production 1 step. Gain 4 titanium."
// Behavior: auto trigger, outputs: titanium +4, titanium-production +1
// =============================================================================
func TestPrelude_OrbitalConstructionYard_TitaniumAndProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Orbital Construction Yard")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	titaniumBefore := p.Resources().Get().Titanium
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Orbital Construction Yard should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Orbital Construction Yard should be in played cards")
	titaniumAfter := p.Resources().Get().Titanium
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, titaniumBefore+4, titaniumAfter, "Should gain 4 titanium")
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium, "Titanium production should increase by 1")
}

// =============================================================================
// P26 - Polar Industries (tags: building)
// "Place 1 ocean tile. Increase your heat production 2 steps."
// Behavior: auto trigger, outputs: heat-production +2, ocean-placement
// =============================================================================
func TestPrelude_PolarIndustries_HeatProductionAndOceanTile(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Polar Industries")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Polar Industries should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Polar Industries should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat, "Heat production should increase by 2")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending ocean tile selection")
}

// =============================================================================
// P27 - Power Generation (tags: power)
// "Increase your energy production 3 steps."
// Behavior: auto trigger, outputs: energy-production +3
// =============================================================================
func TestPrelude_PowerGeneration_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Power Generation")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Generation should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Power Generation should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+3, prodAfter.Energy, "Energy production should increase by 3")
}

// =============================================================================
// P28 - Research Network (tags: wild)
// "Draw 3 cards, and increase your M€ production 1 step."
// Behavior: auto trigger, outputs: card-draw 3, credit-production +1
// =============================================================================
func TestPrelude_ResearchNetwork_CreditProductionAndCardDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Research Network")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	handSizeBefore := p.Hand().CardCount()
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Research Network should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Research Network should be in played cards")
	handSizeAfter := p.Hand().CardCount()
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits, "M€ production should increase by 1")
	testutil.AssertEqual(t, handSizeBefore+3-1, handSizeAfter, "Should draw 3 cards (minus the played prelude)")
}

// =============================================================================
// P29 - Self-Sufficient Settlement (tags: building, city)
// "Increase your M€ production 2 steps. Place a city tile."
// Behavior: credit-production +2, place city tile
// =============================================================================
func TestPrelude_SelfSufficientSettlement_ProductionAndCity(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Self-Sufficient Settlement")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Self-Sufficient Settlement should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Self-Sufficient Settlement should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits, "M€ production should increase by 2")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// =============================================================================
// P30 - Smelting Plant (tags: building)
// "Gain 5 steel. Raise oxygen 2 steps."
// Behavior: steel +5, oxygen +2
// =============================================================================
func TestPrelude_SmeltingPlant_SteelAndOxygen(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Smelting Plant")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	steelBefore := p.Resources().Get().Steel
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Smelting Plant should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Smelting Plant should be in played cards")
	steelAfter := p.Resources().Get().Steel
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, steelBefore+5, steelAfter, "Steel should increase by 5")
	testutil.AssertEqual(t, oxygenBefore+2, oxygenAfter, "Oxygen should increase by 2")
}

// =============================================================================
// P31 - Society Support (no tags)
// "Decrease your M€ production 1 step. Increase your plant production 1 step,
//
//	energy production 1 step, and heat production 1 step."
//
// Behavior: credit-production -1, plant-production +1, energy-production +1, heat-production +1
// =============================================================================
func TestPrelude_SocietySupport_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Society Support")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Society Support should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Society Support should be in played cards")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-1, prodAfter.Credits, "M€ production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy, "Energy production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Heat+1, prodAfter.Heat, "Heat production should increase by 1")
}

// =============================================================================
// P32 - Supplier (tags: power)
// "Gain 4 steel. Increase your energy production 2 steps."
// Behavior: steel +4, energy-production +2
// =============================================================================
func TestPrelude_Supplier_SteelAndEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Supplier")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	steelBefore := p.Resources().Get().Steel
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Supplier should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Supplier should be in played cards")
	steelAfter := p.Resources().Get().Steel
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, steelBefore+4, steelAfter, "Steel should increase by 4")
	testutil.AssertEqual(t, prodBefore.Energy+2, prodAfter.Energy, "Energy production should increase by 2")
}

// =============================================================================
// P33 - Supply Drop (no tags)
// "Gain 3 titanium, 8 steel, and 3 plants."
// Behavior: auto trigger, outputs: steel +8, titanium +3, plant +3
// =============================================================================
func TestPrelude_SupplyDrop_ResourceGains(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Supply Drop")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	resBefore := p.Resources().Get()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Supply Drop should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Supply Drop should be in played cards")
	resAfter := p.Resources().Get()
	testutil.AssertEqual(t, resBefore.Steel+8, resAfter.Steel, "Should gain 8 steel")
	testutil.AssertEqual(t, resBefore.Titanium+3, resAfter.Titanium, "Should gain 3 titanium")
	testutil.AssertEqual(t, resBefore.Plants+3, resAfter.Plants, "Should gain 3 plants")
}

// =============================================================================
// P34 - UNMI Contractor (tags: earth)
// "Raise your terraform rating 3 steps. Draw 1 card."
// Behavior: auto trigger, outputs: card-draw +1, tr +3
// =============================================================================
func TestPrelude_UNMIContractor_DrawCardAndRaiseTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("UNMI Contractor")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	handSizeBefore := p.Hand().CardCount()
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "UNMI Contractor should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "UNMI Contractor should be in played cards")
	handSizeAfter := p.Hand().CardCount()
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, handSizeBefore+1-1, handSizeAfter, "Should draw 1 card (net 0 change after playing)")
	testutil.AssertEqual(t, trBefore+3, trAfter, "Terraform rating should increase by 3")
}

// =============================================================================
// P35 - Acquired Space Agency (no tags)
// "Gain 6 titanium. Reveal cards from the deck until you have revealed 2 space
// cards. Take those into hand, and discard the rest."
// Behavior: auto trigger, outputs: titanium +6, card-draw +2 (selectors: space)
// =============================================================================
func TestPrelude_AcquiredSpaceAgency_TitaniumAndCardDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Acquired Space Agency")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	titaniumBefore := p.Resources().Get().Titanium
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Acquired Space Agency should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Acquired Space Agency should be in played cards")
	titaniumAfter := p.Resources().Get().Titanium
	testutil.AssertEqual(t, titaniumBefore+6, titaniumAfter, "Should gain 6 titanium")

	drawnCards := p.Hand().Cards()
	testutil.AssertEqual(t, 2, len(drawnCards), "Should have 2 cards in hand (drew 2 space cards, played 1 prelude)")
	for _, cardID := range drawnCards {
		drawnCard, err := cardRegistry.GetByID(cardID)
		testutil.AssertNoError(t, err, "Drawn card should exist in registry")
		testutil.AssertTrue(t, slices.Contains(drawnCard.Tags, shared.TagSpace), "Drawn card "+drawnCard.Name+" should have space tag")
	}
}

func TestPsychrophiles_PaymentAndManualMicrobe(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	log := testutil.TestLogger()
	card := testutil.GetCardByName("Psychrophiles")
	p.Resources().Set(shared.Resources{Credits: 20})
	p.Hand().AddCard(card.ID)
	play := cardAction.NewPlayCardAction(repo, registry, nil, log)
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play Psychrophiles")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(card.ID), "no free microbe on play")
	for i, b := range card.Behaviors {
		if gamecards.HasManualTrigger(b) {
			testutil.AssertNoError(t, cardAction.NewUseCardActionAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, card.ID, i, nil, nil, nil, nil, nil, nil, nil, nil), "add microbe")
		}
	}
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "manual action adds one microbe")
	p.Resources().AddToStorage(card.ID, 1)
	source := shared.PaymentSource{Target: "self-card", Resource: shared.ResourceMicrobe, CardID: card.ID}
	for _, tc := range []struct {
		name, action, card string
		allowed            bool
	}{
		{"plant card", shared.ActionCardPlaying, "Adapted Lichen", true},
		{"other card", shared.ActionCardPlaying, "Power Plant", false},
		{"purchase", shared.ActionCardBuying, "Adapted Lichen", false},
		{"action", "card-action", "Adapted Lichen", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := testutil.GetCardByName(tc.card)
			quote, err := gamecards.QuotePayment(p, g, registry, gamecards.PaymentContext{Costs: map[shared.ResourceType]int{shared.ResourceCredit: 4}, Action: tc.action, Card: &target})
			testutil.AssertNoError(t, err, "quote")
			_, err = gamecards.ValidatePayment(quote, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: source, TargetResource: shared.ResourceCredit, Amount: 2}}})
			if tc.allowed {
				testutil.AssertNoError(t, err, "plant payment")
			} else {
				testutil.AssertErrorContains(t, err, "ineligible payment source for credit", "restricted payment")
			}
		})
	}
	target := testutil.GetCardByName("Adapted Lichen")
	p.Hand().AddCard(target.ID)
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "turn")
	payment := shared.NativePayment(shared.ResourceCredit, target.Cost-4)
	payment.Allocations = append(payment.Allocations, shared.PaymentAllocation{Source: source, TargetResource: shared.ResourceCredit, Amount: 2})
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, target.ID, payment, nil, nil, nil, nil, nil), "spend microbes on plant card")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(card.ID), "both microbes spent")
}

func TestResearchCoordination_WildTagIsNotAnActionOrTrigger(t *testing.T) {
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	setCorp := admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, setCorp.Execute(context.Background(), g.ID(), id, testutil.CardID("Point Luna")), "set Point Luna")
	card := testutil.GetCardByName("Research Coordination")
	p.Hand().AddCard(card.ID)
	g.InitDeck([]string{testutil.CardID("Sponsors")}, nil, nil)
	beforeHand, beforeActions := p.Hand().CardCount(), len(p.Actions().List())
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play wild")
	testutil.AssertEqual(t, beforeHand-1, p.Hand().CardCount(), "wild does not trigger Point Luna's Earth draw")
	testutil.AssertEqual(t, beforeActions, len(p.Actions().List()), "wild adds no manual action")
	for _, tag := range []shared.CardTag{shared.TagScience, shared.TagPlant} {
		testutil.AssertEqual(t, 1, gamecards.CountPlayerTags(p, registry, []shared.CardTag{tag}, gamecards.TagCountContext{ActorID: id}), "wild can be used in later independent actions")
	}
	testutil.AssertEqual(t, 0, gamecards.CountPlayerTagsByType(p, registry, shared.TagScience), "wild never changes actual tags")
}
