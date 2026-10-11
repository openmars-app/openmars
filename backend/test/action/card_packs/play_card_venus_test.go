package card_packs_test

import (
	"context"
	"fmt"
	"openmars/internal/action"
	cardAction "openmars/internal/action/card"
	"openmars/internal/action/confirmation"
	tileAction "openmars/internal/action/tile"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
	"reflect"
	"testing"
)

// =============================================================================
// Card 213: Aerial Mappers (active)
// "Action: Add 1 floater to any card, or spend 1 floater here to draw a card."
// Has floater resource storage. 1 VP fixed.
// =============================================================================
func TestAerialMappers_PlayAndStorageCreated(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Aerial Mappers")
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
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Aerial Mappers should play successfully")
	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 0, storage, "Aerial Mappers should start with 0 floaters")
}
func TestAerialMappers_Action_AddFloater(t *testing.T) {
	testGame, repo, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-aerial-mappers"
	p.PlayedCards().AddCard(cardID, "Aerial Mappers", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 0)
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{
		{ID: cardID, Name: "Aerial Mappers", Type: gamecards.CardTypeActive, Tags: []shared.CardTag{shared.TagVenus}, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater}},
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "any-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Aerial Mappers", BehaviorIndex: 0, Behavior: behavior},
	})
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add floater) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater after adding")
}
func TestAerialMappers_Action_SpendFloaterForCardDraw(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-aerial-mappers"
	p.PlayedCards().AddCard(cardID, "Aerial Mappers", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 2)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "any-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Aerial Mappers", BehaviorIndex: 0, Behavior: behavior},
	})
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 (spend floater for card draw) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater after spending 1 from 2")
}

// =============================================================================
// Card 214: Aerosport Tournament (event)
// "Requires that you have 5 floaters. Gain 1 M€ for each city tile in play."
// 1 VP fixed.
// =============================================================================
func TestAerosportTournament_RequiresFloaters(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Aerosport Tournament")
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
	// Player has no floaters - should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "resource requirement not met", "Aerosport Tournament should fail without 5 floaters")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 216: Atalanta Planitia Lab (automated)
// "Requires 3 science tags. Draw 2 cards." Tags: science, venus. 2 VP.
// =============================================================================
func TestAtalantaPlanitiaLab_RequiresScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Atalanta Planitia Lab")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	// Player has 0 science tags - should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 3 additional tags", "Atalanta Planitia Lab should fail without 3 science tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}
func TestAtalantaPlanitiaLab_SucceedsWithScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Atalanta Planitia Lab")
	sciCard1 := gamecards.Card{ID: "sci-1", Name: "Sci 1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagScience}}
	sciCard2 := gamecards.Card{ID: "sci-2", Name: "Sci 2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagScience}}
	sciCard3 := gamecards.Card{ID: "sci-3", Name: "Sci 3", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagScience}}
	additionalCards := []gamecards.Card{sciCard1, sciCard2, sciCard3}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.PlayedCards().AddCard("sci-1", "Sci 1", "automated", []string{"science"})
	p.PlayedCards().AddCard("sci-2", "Sci 2", "automated", []string{"science"})
	p.PlayedCards().AddCard("sci-3", "Sci 3", "automated", []string{"science"})
	p.Hand().AddCard(card.ID)
	handBefore := p.Hand().CardCount()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Atalanta Planitia Lab should succeed with 3 science tags")
	handAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handBefore-1+2, handAfter, "Should draw 2 cards (hand: -1 played +2 drawn)")
}

// =============================================================================
// Card 219: Corroder Suits (automated)
// "Increase your M€ production 2 steps. Add 1 resource to any venus card."
// Tags: venus.
// =============================================================================
func TestCorroderSuits_CreditProductionIncrease(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Corroder Suits")
	venusTargetCard := gamecards.Card{
		ID:        "venus-target-card",
		Name:      "Venus Target",
		Type:      gamecards.CardTypeActive,
		GameModes: []string{"classic"},
		Cost:      1,
		Tags:      []shared.CardTag{shared.TagVenus},
		ResourceStorage: &gamecards.ResourceStorage{
			Type:     shared.ResourceFloater,
			Starting: 0,
		},
	}
	additionalCards := []gamecards.Card{venusTargetCard}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	p.PlayedCards().AddCard("venus-target-card", "Venus Target", "active", []string{"venus"})
	p.Resources().AddToStorage("venus-target-card", 0)
	prodBefore := p.Resources().Production()
	targetCardID := "venus-target-card"
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Corroder Suits should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits, "Credit production should increase by 2")
}

// =============================================================================
// Card 221: Deuterium Export (active)
// "Action: Add 1 floater to this card, or spend 1 floater here to increase
//
//	your energy production 1 step."
//
// Tags: power, space, venus. Has floater storage.
// =============================================================================
func TestDeuteriumExport_PlayAndStorageCreated(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Deuterium Export")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Deuterium Export should play successfully")
	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 0, storage, "Deuterium Export should start with 0 floaters")
}
func TestDeuteriumExport_Action_AddFloater(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-deuterium-export"
	p.PlayedCards().AddCard(cardID, "Deuterium Export", "active", []string{"power", "space", "venus"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Deuterium Export", BehaviorIndex: 0, Behavior: behavior},
	})
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add floater) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater after adding")
}
func TestDeuteriumExport_Action_SpendFloaterForEnergyProduction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-deuterium-export"
	p.PlayedCards().AddCard(cardID, "Deuterium Export", "active", []string{"power", "space", "venus"})
	p.Resources().AddToStorage(cardID, 3)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Deuterium Export", BehaviorIndex: 0, Behavior: behavior},
	})
	prodBefore := p.Resources().Production()
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 (spend floater for energy production) should succeed")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(cardID), "Card should have 2 floaters after spending 1 from 3")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy, "Energy production should increase by 1")
}

// =============================================================================
// Card 222: Dirigibles (active)
// "Action: Add 1 floater to any card. / Effect: When playing a Venus tag,
//
//	floaters here may be used as payment, and are worth 3 M€ each."
//
// Tags: venus. Has floater storage.
// =============================================================================
func TestDirigibles_StoragePaymentSubstitute_VenusCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 200})

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)

	// Play Dirigibles
	dirigibles := testutil.GetCardByName("Dirigibles")
	p.Hand().AddCard(dirigibles.ID)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), dirigibles.ID, shared.NativePayment(shared.ResourceCredit, 11), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Dirigibles should play successfully")

	// Verify storage payment substitute was registered
	subs := p.Resources().PaymentSubstitutes()
	testutil.AssertTrue(t, len(subs) > 0, "Should have storage payment substitute after playing Dirigibles")
	testutil.AssertEqual(t, dirigibles.ID, subs[0].Source.CardID, "Storage payment substitute should reference Dirigibles")
	testutil.AssertEqual(t, 3, subs[0].ConversionRate, "Floater conversion rate should be 3 M€")

	// Add 3 floaters to Dirigibles
	p.Resources().AddToStorage(dirigibles.ID, 3)
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(dirigibles.ID), "Dirigibles should have 3 floaters")

	// Play Ishtar Mining (venus-tagged, cost 5) using 1 floater (3 M€) + 2 credits
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	ishtarMining := testutil.GetCardByName("Ishtar Mining")
	p.Hand().AddCard(ishtarMining.ID)
	// Set venus to 8% so Ishtar Mining requirement is met
	for i := 0; i < 4; i++ {
		if _, err := testGame.GlobalParameters().IncreaseVenus(ctx, 1, p.ID()); err != nil {
			t.Fatalf("IncreaseVenus failed: %v", err)
		}
	}

	creditsBefore := p.Resources().Get().Credits
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), ishtarMining.ID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-player", Resource: "credit"}, TargetResource: shared.ResourceCredit, Amount: 2}, {Source: shared.PaymentSource{Target: "self-card", Resource: "floater",
		CardID: dirigibles.ID}, TargetResource: shared.ResourceCredit,
		Amount: 1}},
	}, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Should pay for Venus card with Dirigibles floaters")

	// Verify floaters were deducted
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(dirigibles.ID), "Should have 2 floaters remaining")
	// Verify credits were deducted (2 credits for the remaining cost)
	testutil.AssertEqual(t, creditsBefore-2, p.Resources().Get().Credits, "Should deduct 2 credits")
}

func TestDirigibles_StoragePaymentSubstitute_NonVenusCardRejected(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 200})

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)

	// Play Dirigibles first
	dirigibles := testutil.GetCardByName("Dirigibles")
	p.Hand().AddCard(dirigibles.ID)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), dirigibles.ID, shared.NativePayment(shared.ResourceCredit, 11), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Dirigibles should play successfully")

	// Add floaters
	p.Resources().AddToStorage(dirigibles.ID, 3)

	// Try to play a non-Venus card using Dirigibles floaters — should fail
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	asteroid := testutil.GetCardByName("Asteroid")
	p.Hand().AddCard(asteroid.ID)
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), asteroid.ID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-card", Resource: "floater",
		CardID: dirigibles.ID}, TargetResource: shared.ResourceCredit,
		Amount: 3}},
	}, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "ineligible payment source for credit", "Should reject Dirigibles floaters for non-Venus card")

	// Verify floaters were NOT deducted
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(dirigibles.ID), "Floaters should not be deducted on failure")
}

// =============================================================================
// Card 223: Extractor Balloons (active)
// "Action: Add 1 floater to this card, or remove 2 floaters here to raise
//
//	Venus 1 step. Add 3 floaters to this card." (on play)
//
// Tags: venus. Has floater storage.
// =============================================================================
func TestExtractorBalloons_PlayAdds3Floaters(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Extractor Balloons")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Extractor Balloons should play successfully")
	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 3, storage, "Extractor Balloons should have 3 floaters after playing")
}

// =============================================================================
// Card 224: Extremophiles (active)
// "Action: Add 1 microbe to any card. Requires 2 science tags."
// Tags: microbe, venus. Has microbe storage. 1 VP per 3 microbes.
// =============================================================================
func TestExtremophiles_RequiresScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Extremophiles")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	// No science tags - should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 3)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 2 additional tags", "Extremophiles should fail without 2 science tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 225: Floating Habs (active)
// "Action: Spend 2 M€ to add 1 floater to any card. Requires 2 science tags."
// Tags: venus. Has floater storage. 1 VP per 2 floaters.
// =============================================================================
func TestFloatingHabs_Action_Spend2CreditsForFloater(t *testing.T) {
	testGame, repo, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-floating-habs"
	p.PlayedCards().AddCard(cardID, "Floating Habs", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 0)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 50})
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{
		{ID: cardID, Name: "Floating Habs", Type: gamecards.CardTypeActive, Tags: []shared.CardTag{shared.TagVenus}, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceFloater}},
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceFloater, 1, "any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Floating Habs", BehaviorIndex: 0, Behavior: behavior},
	})
	creditsBefore := p.Resources().Get().Credits
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Floating Habs action should succeed")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-2, creditsAfter, "Should spend 2 credits")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater")
}

// =============================================================================
// Card 226: Forced Precipitation (active)
// "Action: Spend 2 M€ to add a floater to this card, or spend 2 floaters
//
//	here to increase Venus 1 step."
//
// Tags: venus. Has floater storage.
// =============================================================================
func TestForcedPrecipitation_Action_PayCreditsForFloater(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-forced-precipitation"
	p.PlayedCards().AddCard(cardID, "Forced Precipitation", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 0)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 50})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 2, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Forced Precipitation", BehaviorIndex: 0, Behavior: behavior},
	})
	creditsBefore := p.Resources().Get().Credits
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (spend credits for floater) should succeed")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-2, creditsAfter, "Should spend 2 credits")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater")
}
func TestForcedPrecipitation_Action_FailsWithoutEnoughFloaters(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-forced-precipitation"
	p.PlayedCards().AddCard(cardID, "Forced Precipitation", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 1) // Only 1 floater, need 2
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 2, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Forced Precipitation", BehaviorIndex: 0, Behavior: behavior},
	})
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient resources on card", "Choice 1 should fail with only 1 floater (need 2)")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "floaters unchanged")
}

// =============================================================================
// Card 228: GHG Import From Venus (event)
// "Raise Venus 1 step. Increase your heat production 3 steps."
// Tags: space, venus.
// =============================================================================
func TestGHGImportFromVenus_HeatProductionIncrease(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("GHG Import From Venus")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 23)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "GHG Import From Venus should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+3, prodAfter.Heat, "Heat production should increase by 3")
}

// =============================================================================
// Card 232: Io Sulphur Research (automated)
// "Draw 1 card, or draw 3 cards if you have at least 3 Venus tags."
// Tags: jovian, science. 2 VP.
// =============================================================================
func TestIoSulphurResearch_DrawOneCardWithoutVenusTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Io Sulphur Research")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	handBefore := p.Hand().CardCount()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 17)
	choiceIndex := 0
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Io Sulphur Research choice 0 should succeed")
	handAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handBefore-1+1, handAfter, "Should draw 1 card (hand: -1 played +1 drawn)")
}
func TestIoSulphurResearch_FailsDraw3WithoutVenusTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Io Sulphur Research")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	// No venus tags - choice 1 (draw 3) should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 17)
	choiceIndex := 1
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "choice 1 requirements not met: tag requirements", "Io Sulphur Research choice 1 should fail without 3 venus tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

func TestIoSulphurResearch_FailsDraw3WithOnly2VenusTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 200})

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)

	// Play 2 venus-tagged cards: Dirigibles (cost 11) and Jet Stream Microscrappers (cost 12)
	dirigibles := testutil.GetCardByName("Dirigibles")
	p.Hand().AddCard(dirigibles.ID)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), dirigibles.ID, shared.NativePayment(shared.ResourceCredit, 11), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Dirigibles should play successfully")

	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	jetStream := testutil.GetCardByName("Jet Stream Microscrappers")
	p.Hand().AddCard(jetStream.ID)
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), jetStream.ID, shared.NativePayment(shared.ResourceCredit, 12), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Jet Stream Microscrappers should play successfully")

	// Verify player has exactly 2 venus tags
	venusTagCount := gamecards.CountPlayerTagsByType(p, cardRegistry, shared.TagVenus)
	testutil.AssertEqual(t, 2, venusTagCount, "Player should have exactly 2 venus tags")

	// Now play Io Sulphur Research with choice 1 (draw 3, requires 3+ venus tags) — should fail
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	ioSulphur := testutil.GetCardByName("Io Sulphur Research")
	p.Hand().AddCard(ioSulphur.ID)
	choiceIndex := 1
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), ioSulphur.ID, shared.NativePayment(shared.ResourceCredit, 17), &choiceIndex, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "choice 1 requirements not met: tag requirements", "Io Sulphur Research choice 1 should fail with only 2 venus tags")

	// Verify card is still in hand (play was rejected)
	testutil.AssertTrue(t, p.Hand().HasCard(ioSulphur.ID), "Io Sulphur Research should remain in hand after failed play")

	// Also verify state calculator marks choice 1 as unavailable
	for _, behavior := range ioSulphur.Behaviors {
		if len(behavior.Choices) >= 2 {
			errors := action.CalculateChoiceErrors(behavior.Choices[1], p, testGame, cardRegistry)
			testutil.AssertTrue(t, len(errors) > 0, "State calculator should report errors for choice 1 with only 2 venus tags")
		}
	}
}

// =============================================================================
// Card 233: Ishtar Mining (automated)
// "Requires Venus 8%. Increase your titanium production 1 step."
// Tags: venus.
// =============================================================================
func TestIshtarMining_TitaniumProductionIncrease(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Ishtar Mining")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 8), "SetVenus failed")
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ishtar Mining should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium, "Titanium production should increase by 1")
}

// =============================================================================
// Card 234: Jet Stream Microscrappers (active)
// "Action: Spend 1 titanium to add 2 floaters to this card, or remove 2
//
//	floaters here to raise Venus 1 step."
//
// Tags: venus. Has floater storage.
// =============================================================================
func TestJetStreamMicroscrappers_Action_SpendTitaniumFor2Floaters(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-jet-stream-microscrappers"
	p.PlayedCards().AddCard(cardID, "Jet Stream Microscrappers", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 0)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceTitanium: 5})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourceTitanium, 1, "self-player"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 2, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 2, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Jet Stream Microscrappers", BehaviorIndex: 0, Behavior: behavior},
	})
	titaniumBefore := p.Resources().Get().Titanium
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (spend titanium for floaters) should succeed")
	titaniumAfter := p.Resources().Get().Titanium
	testutil.AssertEqual(t, titaniumBefore-1, titaniumAfter, "Should spend 1 titanium")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(cardID), "Card should have 2 floaters")
}

// =============================================================================
// Card 235: Local Shading (active)
// "Action: Add 1 floater to this card, or spend 1 floater here to raise your
//
//	M€ production 1 step."
//
// Tags: venus. Has floater storage.
// =============================================================================
func TestLocalShading_Action_AddFloater(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-local-shading"
	p.PlayedCards().AddCard(cardID, "Local Shading", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceCreditProduction, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Local Shading", BehaviorIndex: 0, Behavior: behavior},
	})
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add floater) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater")
}
func TestLocalShading_Action_SpendFloaterForCreditProduction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-local-shading"
	p.PlayedCards().AddCard(cardID, "Local Shading", "active", []string{"venus"})
	p.Resources().AddToStorage(cardID, 3)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceCreditProduction, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{CardID: cardID, CardName: "Local Shading", BehaviorIndex: 0, Behavior: behavior},
	})
	prodBefore := p.Resources().Production()
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 (spend floater for credit production) should succeed")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(cardID), "Card should have 2 floaters after spending 1")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits, "Credit production should increase by 1")
}

// =============================================================================
// Card 236: Luna Metropolis (automated)
// "Increase your M€ production 1 step for each Earth tag you have, including
//
//	this. Place a city tile on the reserved area."
//
// Tags: city, earth, space.  2 VP.
// =============================================================================
func TestLunaMetropolis_CreditProductionPerEarthTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Luna Metropolis")
	earthCard1 := gamecards.Card{ID: "earth-1", Name: "Earth Card 1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	earthCard2 := gamecards.Card{ID: "earth-2", Name: "Earth Card 2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	additionalCards := []gamecards.Card{earthCard1, earthCard2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	// Add 2 earth-tagged played cards
	p.PlayedCards().AddCard("earth-1", "Earth Card 1", "automated", []string{"earth"})
	p.PlayedCards().AddCard("earth-2", "Earth Card 2", "automated", []string{"earth"})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Luna Metropolis should play successfully")
	prodAfter := p.Resources().Production()
	// 2 existing earth tags + 1 from this card (earth tag) = 3 earth tags
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3 (1 per earth tag: 2 existing + 1 from this card)")
}

func TestLunaMetropolis_PlacesCityTile(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Luna Metropolis")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Luna Metropolis should play successfully")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection after playing Luna Metropolis")
}

// =============================================================================
// Card 237: Luxury Foods (automated)
// "Requires Venus, Earth and Jovian tags." No behaviors, just VP.
// 2 VP fixed.
// =============================================================================
func TestLuxuryFoods_FailsWithoutRequiredTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Luxury Foods")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	// No tags at all - should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 3 additional tags", "Luxury Foods should fail without required tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}
func TestLuxuryFoods_SucceedsWithAllTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Luxury Foods")
	earthCard := gamecards.Card{ID: "earth-tag-card", Name: "Earth Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	jovianCard := gamecards.Card{ID: "jovian-tag-card", Name: "Jovian Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagJovian}}
	venusCard := gamecards.Card{ID: "venus-tag-card", Name: "Venus Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	additionalCards := []gamecards.Card{earthCard, jovianCard, venusCard}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.PlayedCards().AddCard("earth-tag-card", "Earth Card", "automated", []string{"earth"})
	p.PlayedCards().AddCard("jovian-tag-card", "Jovian Card", "automated", []string{"jovian"})
	p.PlayedCards().AddCard("venus-tag-card", "Venus Card", "automated", []string{"venus"})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Luxury Foods should succeed with earth, jovian, and venus tags")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-8, creditsAfter, "Should only pay 8 credits (no other effects)")
}

// =============================================================================
// Card 230: Gyropolis (automated)
// "Decrease your energy production 2 steps. Increase your M€ production 1
//
//	step for each Venus and Earth tag you have. Place a city tile."
//
// Tags: building, city.
// =============================================================================
func TestGyropolis_ProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Gyropolis")
	earthCard := gamecards.Card{ID: "earth-g", Name: "Earth Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	venusCard1 := gamecards.Card{ID: "venus-g1", Name: "Venus Card 1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	venusCard2 := gamecards.Card{ID: "venus-g2", Name: "Venus Card 2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	additionalCards := []gamecards.Card{earthCard, venusCard1, venusCard2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 3,
	})
	p.PlayedCards().AddCard("earth-g", "Earth Card", "automated", []string{"earth"})
	p.PlayedCards().AddCard("venus-g1", "Venus Card 1", "automated", []string{"venus"})
	p.PlayedCards().AddCard("venus-g2", "Venus Card 2", "automated", []string{"venus"})
	wild := testutil.GetCardByName("Research Coordination")
	p.PlayedCards().AddCard(wild.ID, wild.Name, string(wild.Type), nil)

	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 20)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Gyropolis should play successfully")
	prodAfter := p.Resources().Production()
	// 1 earth tag + 2 venus tags + 1 wild = 4 credit production increase
	testutil.AssertEqual(t, prodBefore.Credits+4, prodAfter.Credits,
		"Credit production should increase by 4 (1 earth + 2 venus + 1 wild)")
	testutil.AssertEqual(t, prodBefore.Energy-2, prodAfter.Energy,
		"Energy production should decrease by 2")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// =============================================================================
// Card 239: Mining Quota
// "Requires Venus, Earth and Jovian tags. Increase your steel production 2 steps."
// =============================================================================
func TestMiningQuota_IncreaseSteelProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Quota")
	earthHelper := gamecards.Card{ID: "earth-1", Name: "Earth Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	jovianHelper := gamecards.Card{ID: "jovian-1", Name: "Jovian Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagJovian}}
	venusHelper := gamecards.Card{ID: "venus-1", Name: "Venus Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	additionalCards := []gamecards.Card{earthHelper, jovianHelper, venusHelper}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.PlayedCards().AddCard("earth-1", "Earth Card", "automated", []string{"earth"})
	p.PlayedCards().AddCard("jovian-1", "Jovian Card", "automated", []string{"jovian"})
	p.PlayedCards().AddCard("venus-1", "Venus Card", "automated", []string{"venus"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mining Quota should play successfully with required tags")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Steel+2, prodAfter.Steel, "Steel production should increase by 2")
}
func TestMiningQuota_FailsWithoutRequiredTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Quota")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	// Only earth tag, missing jovian and venus
	p.PlayedCards().AddCard("earth-1", "Earth Card", "automated", []string{"earth"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 3 additional tags", "Mining Quota should fail without jovian and venus tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 240: Neutralizer Factory
// "Requires Venus 10%. Increase Venus 1 step."
// =============================================================================
func TestNeutralizerFactory_IncreaseVenus(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Neutralizer Factory")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 10), "SetVenus failed")
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Neutralizer Factory should play successfully")
}

// =============================================================================
// Card 241: Omnicourt
// "Requires Venus, Earth, and Jovian tags. Increase your TR 2 steps."
// =============================================================================
func TestOmnicourt_IncreaseTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Omnicourt")
	earthHelper := gamecards.Card{ID: "earth-1", Name: "Earth Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	jovianHelper := gamecards.Card{ID: "jovian-1", Name: "Jovian Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagJovian}}
	venusHelper := gamecards.Card{ID: "venus-1", Name: "Venus Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	additionalCards := []gamecards.Card{earthHelper, jovianHelper, venusHelper}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.PlayedCards().AddCard("earth-1", "Earth Card", "automated", []string{"earth"})
	p.PlayedCards().AddCard("jovian-1", "Jovian Card", "automated", []string{"jovian"})
	p.PlayedCards().AddCard("venus-1", "Venus Card", "automated", []string{"venus"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Omnicourt should play successfully with required tags")
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+2, trAfter, "TR should increase by 2")
}

// =============================================================================
// Card 242: Orbital Reflectors
// "Raise Venus 2 steps. Increase your heat production 2 steps."
// =============================================================================
func TestOrbitalReflectors_HeatProductionAndVenus(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Orbital Reflectors")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 26)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Orbital Reflectors should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat, "Heat production should increase by 2")
}

// =============================================================================
// Card 243: Rotator Impacts
// "Action: Spend 6 M€ to add a floater to this card (titanium may be used), or
// spend 1 floater here to increase Venus 1 step."
// =============================================================================
func TestRotatorImpacts_AddFloater(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-rotator-impacts"
	p.PlayedCards().AddCard(cardID, "Rotator Impacts", "active", []string{"space"})
	p.Resources().AddToStorage(cardID, 0)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Inputs: []shared.BehaviorCondition{
					&shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCredit, Amount: 6, Target: "self-player"}, PaymentAllowed: []shared.ResourceType{shared.ResourceTitanium}},
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceFloater, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Rotator Impacts",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add floater) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 floater")
}

// =============================================================================
// Card 244: Sister Planet Support
// "Requires Venus and Earth tags. Increase your M€ production 3 steps."
// =============================================================================
func TestSisterPlanetSupport_CreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Sister Planet Support")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	// Self-tags satisfy the requirement since the card itself has venus+earth
	p.PlayedCards().AddCard("venus-1", "Venus Card", "automated", []string{"venus"})
	p.PlayedCards().AddCard("earth-1", "Earth Card", "automated", []string{"earth"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Sister Planet Support should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits, "Credit production should increase by 3")
}

// =============================================================================
// Card 245: Solarnet
// "Requires Venus, Earth, and Jovian tags. Draw 2 cards."
// =============================================================================
func TestSolarnet_Draw2Cards(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Solarnet")
	venusHelper := gamecards.Card{ID: "venus-1", Name: "Venus Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	earthHelper := gamecards.Card{ID: "earth-1", Name: "Earth Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEarth}}
	jovianHelper := gamecards.Card{ID: "jovian-1", Name: "Jovian Card", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagJovian}}
	additionalCards := []gamecards.Card{venusHelper, earthHelper, jovianHelper}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.PlayedCards().AddCard("venus-1", "Venus Card", "automated", []string{"venus"})
	p.PlayedCards().AddCard("earth-1", "Earth Card", "automated", []string{"earth"})
	p.PlayedCards().AddCard("jovian-1", "Jovian Card", "automated", []string{"jovian"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	handSizeBefore := len(p.Hand().Cards())
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Solarnet should play successfully")
	handSizeAfter := len(p.Hand().Cards())
	// -1 from playing the card, +2 from drawing = net +1
	testutil.AssertEqual(t, handSizeBefore+1, handSizeAfter,
		"Hand should have net +1 card (played 1, drew 2)")
}

// =============================================================================
// Card 246: Spin-Inducing Asteroid
// "Raise Venus 2 steps."
// =============================================================================
func TestSpinInducingAsteroid_RaiseVenus(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Spin-Inducing Asteroid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Spin-Inducing Asteroid should play successfully")
}

// =============================================================================
// Card 247: Sponsored Academies
// "Discard 1 card from hand and **then** draw 3 cards. All **opponents** draw 1 card."
// =============================================================================
func TestSponsoredAcademies_DiscardDrawAndOpponentDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Sponsored Academies")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	opponent := players[1]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	opponent.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})

	// Player has: Sponsored Academies + 2 fodder cards = 3 cards in hand
	p.Hand().AddCard(card.ID)
	p.Hand().AddCard("card-fodder-1")
	p.Hand().AddCard("card-fodder-2")

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Sponsored Academies should play successfully")

	// Step 1: After playing, a pending card discard selection should exist
	selection := p.Selection().GetPendingBehaviorResolutions()[0]
	if selection == nil {
		t.Fatal("Expected pending card discard selection after playing Sponsored Academies")
	}
	testutil.AssertEqual(t, 1, selection.MinCards, "Should require discarding exactly 1 card")
	testutil.AssertEqual(t, 1, selection.MaxCards, "Should allow discarding exactly 1 card")

	// Player hand after play: 3 - 1 (played SA) = 2 cards remaining (fodder-1, fodder-2)
	testutil.AssertEqual(t, 2, p.Hand().CardCount(), "Player should have 2 cards before discard")
	testutil.AssertEqual(t, 0, opponent.Hand().CardCount(), "Opponent should have 0 cards before discard confirmation")

	// Step 2: Confirm discard — player chooses to discard fodder-1
	confirmAction := confirmation.NewConfirmCardDiscardAction(repo, cardRegistry, nil, logger)
	err = confirmAction.Execute(ctx, testGame.ID(), p.ID(), selection.ID, []string{"card-fodder-1"})
	testutil.AssertNoError(t, err, "Confirm card discard should succeed")

	// Player: 2 - 1 (discarded) + 3 (drew) = 4 cards
	testutil.AssertEqual(t, 4, p.Hand().CardCount(),
		"Player should have 4 cards after discard and draw")

	// Opponent: drew 1 card
	testutil.AssertEqual(t, 1, opponent.Hand().CardCount(),
		"Opponent should have drawn 1 card from Sponsored Academies")
}

// =============================================================================
// Card 249: Stratospheric Birds
// "Action: Add 1 animal to this card. Requires Venus 12%.
// Remove 1 floater from any card. 1 VP per animal on this card."
// =============================================================================
func TestStratosphericBirds_ActionAddAnimal(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-stratospheric-birds"
	p.PlayedCards().AddCard(cardID, "Stratospheric Birds", "active", []string{"animal", "venus"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Stratospheric Birds",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Adding animal via action should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 animal")
}

// =============================================================================
// Card 250: Sulphur Exports
// "Increase your M€ production 1 step per Venus tag you have, including this."
// =============================================================================
func TestSulphurExports_CreditProductionPerVenusTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Sulphur Exports")
	venusHelper1 := gamecards.Card{ID: "venus-1", Name: "Venus Card 1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	venusHelper2 := gamecards.Card{ID: "venus-2", Name: "Venus Card 2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagVenus}}
	additionalCards := []gamecards.Card{venusHelper1, venusHelper2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	// 2 existing venus tags + 1 from card itself = 3 total
	p.PlayedCards().AddCard("venus-1", "Venus Card 1", "automated", []string{"venus"})
	p.PlayedCards().AddCard("venus-2", "Venus Card 2", "automated", []string{"venus"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Sulphur Exports should play successfully")
	prodAfter := p.Resources().Production()
	// 2 existing + 1 from card = 3 venus tags -> +3 credit production
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3 (1 per venus tag, 3 total)")
}

// =============================================================================
// Card 252: Terraforming Contract
// "Requires TR 25 or higher. Increase your M€ production 4 steps."
// =============================================================================
func TestTerraformingContract_CreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Terraforming Contract")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	// Raise TR to 25
	p.Resources().UpdateTerraformRating(5) // starts at 20, add 5 to reach 25
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Terraforming Contract should play successfully at TR 25")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+4, prodAfter.Credits,
		"Credit production should increase by 4")
}
func TestTerraformingContract_FailsBelowTR25(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Terraforming Contract")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	// TR stays at default (20), below 25
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "terraform rating requirement not met", "Terraforming Contract should fail below TR 25")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 253: Thermophiles
// "Action: Add 1 microbe to this card, or spend 2 microbes here to raise Venus 1 step."
// Requires Venus 6%.
// =============================================================================
func TestThermophiles_AddMicrobe(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-thermophiles"
	p.PlayedCards().AddCard(cardID, "Thermophiles", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 2, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Thermophiles",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add microbe) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 microbe")
}
func TestThermophiles_SpendMicrobesForVenus(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-thermophiles"
	p.PlayedCards().AddCard(cardID, "Thermophiles", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 3)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 2, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Thermophiles",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 (spend 2 microbes for Venus) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 microbe after spending 2 from 3")
}

// =============================================================================
// Card 254: Water To Venus
// "Raise Venus 1 step."
// =============================================================================
func TestWaterToVenus_RaiseVenus(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Water To Venus")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Water To Venus should play successfully")
}

// =============================================================================
// Card 255: Venus Governor
// "Requires 2 Venus tags. Increase your M€ production 2 steps."
// =============================================================================
func TestVenusGovernor_CreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Venus Governor")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 2), "SetVenus failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Venus Governor should play with Venus >= 2%")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits,
		"Credit production should increase by 2")
}

// =============================================================================
// Card 256: Venus Magnetizer
// "Action: Decrease your energy production 1 step to raise Venus 1 step."
// Requires Venus 10%.
// =============================================================================
func TestVenusMagnetizer_ActionDecraseEnergyForVenus(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	cardID := "test-venus-magnetizer"
	p.PlayedCards().AddCard(cardID, "Venus Magnetizer", "active", []string{"venus"})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Venus Magnetizer",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	prodBefore := p.Resources().Production()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Venus Magnetizer action should succeed")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
}

// =============================================================================
// Card 257: Venus Soils
// "Raise Venus 1 step. Increase your plant production 1 step. Add 2 microbes to another card."
// =============================================================================
func TestVenusSoils_PlantProductionAndVenus(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Venus Soils")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 20)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Venus Soils should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants,
		"Plant production should increase by 1")
}

// =============================================================================
// Card 258: Venus Waystation
// "Effect: When you play a Venus tag, you pay 2 M€ less for it."
// =============================================================================
func TestVenusWaystation_DiscountEffect(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Venus Waystation")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Venus Waystation should play successfully and register discount effect")
}

// =============================================================================
// Card 260: Venusian Insects
// "Action: Add 1 microbe to this card. Requires Venus 12%.
// 1 VP per 2 microbes on this card."
// =============================================================================
func TestVenusianInsects_ActionAddMicrobe(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-venusian-insects"
	p.PlayedCards().AddCard(cardID, "Venusian Insects", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Venusian Insects",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Adding microbe via action should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 microbe")
}

// =============================================================================
// Card 238: Maxwell Base
// "Decrease your energy production 1 step. Place a city tile on the reserved area."
// "Action: Add 1 resource to another venus card."
// Requires Venus 12%.
// =============================================================================
func TestMaxwellBase_DecreaseEnergyProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Maxwell Base")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 12), "SetVenus failed")
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Maxwell Base should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
}

// =============================================================================
// Card 248: Stratopolis
// "Requires 2 science tags. Place a city tile.
// Action: Add 1 floater to this card, or add 2 floaters to this card.
// 1 VP per 3 floaters on this card."
// =============================================================================
func TestStratopolis_CityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Stratopolis")
	sciHelper1 := gamecards.Card{ID: "science-1", Name: "Science Card 1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagScience}}
	sciHelper2 := gamecards.Card{ID: "science-2", Name: "Science Card 2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagScience}}
	additionalCards := []gamecards.Card{sciHelper1, sciHelper2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.PlayedCards().AddCard("science-1", "Science Card 1", "automated", []string{"science"})
	p.PlayedCards().AddCard("science-2", "Science Card 2", "automated", []string{"science"})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 22)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Stratopolis should play successfully with 2 science tags")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// =============================================================================
// Card 259: Venusian Animals
// "Effect: When you play a science tag, including this, add 1 animal to this card.
// Requires Venus 18%. 1 VP for each animal on this card."
// =============================================================================
func TestVenusianAnimals_PlaysAndRegistersEffect(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Venusian Animals")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 18), "SetVenus failed")
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 15)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Venusian Animals should play successfully")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "own science tag adds exactly one animal")
	research := testutil.GetCardByName("Research")
	p.Hand().AddCard(research.ID)
	testutil.AssertNoError(t, playCardAction.Execute(ctx, testGame.ID(), p.ID(), research.ID, shared.NativePayment(shared.ResourceCredit, research.Cost), nil, nil, nil, nil, nil), "play Research")
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(card.ID), "Research adds two animals for its two science tags")
	players[1].PlayedCards().AddCard(research.ID, research.Name, string(research.Type), []string{"science", "science"})
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(card.ID), "opponent science tags do not add animals")
	p.PlayedCards().AddCard("unrelated", "Unrelated", "automated", []string{"animal", "venus"})
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(card.ID), "unrelated tags do not add animals")

}

// =============================================================================
// Card 261: Venusian Plants
// "Requires Venus 16%. Raise Venus 1 step. Add 1 microbe or 1 animal to another venus card."
// Uses choices for the microbe/animal selection.
// =============================================================================
func TestVenusianPlants_RaiseVenusWithAnimalChoice(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Venusian Plants")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 16), "SetVenus failed")
	choiceIndex := 0 // choose animal
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Venusian Plants should play with animal choice")
}

// =============================================================================
// Card 251: Sulphur-Eating Bacteria (active)
// "Action: Add 1 microbe to this card, or spend any number of microbes here
//
//	to gain triple that amount of M€."
//
// Requires Venus 6%. Tags: microbe, venus. Has microbe storage.
// =============================================================================
func sulphurEatingBacteriaBehavior() shared.CardBehavior {
	return shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					&shared.CardStorageCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceMicrobe, Amount: 1, Target: "self-card"}, VariableAmount: true},
				},
				Outputs: []shared.BehaviorCondition{
					&shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCredit, Amount: 3, Target: "self-player"}, VariableAmount: true},
				},
			},
		},
	}
}
func TestSulphurEatingBacteria_Choice0_AddMicrobe(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-sulphur-eating-bacteria"
	p.PlayedCards().AddCard(cardID, "Sulphur-Eating Bacteria", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 0)
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Sulphur-Eating Bacteria",
			BehaviorIndex: 0,
			Behavior:      sulphurEatingBacteriaBehavior(),
		},
	})
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add microbe) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 microbe after adding")
}
func TestSulphurEatingBacteria_Choice1_SpendMicrobesForCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-sulphur-eating-bacteria"
	p.PlayedCards().AddCard(cardID, "Sulphur-Eating Bacteria", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 3)
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Sulphur-Eating Bacteria",
			BehaviorIndex: 0,
			Behavior:      sulphurEatingBacteriaBehavior(),
		},
	})
	creditsBefore := p.Resources().Get().Credits
	choiceIndex := 1
	selectedAmount := 2
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, &selectedAmount, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 (spend microbes for credits) should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 microbe after spending 2 from 3")
	testutil.AssertEqual(t, creditsBefore+6, p.Resources().Get().Credits, "Should gain 6 credits (2 microbes * 3)")
}
func TestSulphurEatingBacteria_Choice1_FailsWithoutSelectedAmount(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-sulphur-eating-bacteria"
	p.PlayedCards().AddCard(cardID, "Sulphur-Eating Bacteria", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 3)
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Sulphur-Eating Bacteria",
			BehaviorIndex: 0,
			Behavior:      sulphurEatingBacteriaBehavior(),
		},
	})
	creditsBefore := p.Resources().Get().Credits
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "must select an amount", "Choice 1 without selectedAmount should fail")
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(cardID), "Microbes should be unchanged")
	testutil.AssertEqual(t, creditsBefore, p.Resources().Get().Credits, "Credits should be unchanged")
	actions := p.Actions().List()
	testutil.AssertEqual(t, 0, actions[0].TimesUsedThisGeneration, "Action should not be marked as used")
}
func TestSulphurEatingBacteria_Choice1_SpendAllMicrobes(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-sulphur-eating-bacteria"
	p.PlayedCards().AddCard(cardID, "Sulphur-Eating Bacteria", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 5)
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Sulphur-Eating Bacteria",
			BehaviorIndex: 0,
			Behavior:      sulphurEatingBacteriaBehavior(),
		},
	})
	creditsBefore := p.Resources().Get().Credits
	choiceIndex := 1
	selectedAmount := 5
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, &selectedAmount, nil, nil, nil)
	testutil.AssertNoError(t, err, "Spending all microbes should succeed")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(cardID), "Card should have 0 microbes")
	testutil.AssertEqual(t, creditsBefore+15, p.Resources().Get().Credits, "Should gain 15 credits (5 * 3)")
}
func TestSulphurEatingBacteria_Choice1_FailsWhenInsufficientMicrobes(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-sulphur-eating-bacteria"
	p.PlayedCards().AddCard(cardID, "Sulphur-Eating Bacteria", "active", []string{"microbe", "venus"})
	p.Resources().AddToStorage(cardID, 2)
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Sulphur-Eating Bacteria",
			BehaviorIndex: 0,
			Behavior:      sulphurEatingBacteriaBehavior(),
		},
	})
	choiceIndex := 1
	selectedAmount := 5
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, &choiceIndex, []string{cardID}, nil, nil, &selectedAmount, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient resources on card", "Should fail when trying to spend more microbes than available")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(cardID), "Microbes should be unchanged")
}

// =============================================================================
// Venus Global Parameter Tests
// =============================================================================
func TestVenusRequirement_BlocksWhenTooLow(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := gamecards.Card{
		ID:        "card-venus-req-low-test",
		Name:      "Venus Req Low Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Tags:      []shared.CardTag{shared.TagVenus},
		Requirements: &gamecards.CardRequirements{
			Items: []gamecards.Requirement{
				{Type: gamecards.RequirementVenus, Min: testutil.IntPtr(8)},
			},
		},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceTitaniumProduction, 1, "self-player"),
				},
			},
		},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{card})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard("card-venus-req-low-test")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 6), "SetVenus failed")
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-venus-req-low-test", payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "venus requirement not met", "Should fail when Venus is too low")
	testutil.AssertTrue(t, p.Hand().HasCard("card-venus-req-low-test"), "rejected card stays in hand")
}
func TestVenusRequirement_BlocksWhenTooHigh(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := gamecards.Card{
		ID:        "card-venus-req-high-test",
		Name:      "Venus Req High Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Tags:      []shared.CardTag{shared.TagVenus},
		Requirements: &gamecards.CardRequirements{
			Items: []gamecards.Requirement{
				{Type: gamecards.RequirementVenus, Max: testutil.IntPtr(14)},
			},
		},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceTitaniumProduction, 1, "self-player"),
				},
			},
		},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{card})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard("card-venus-req-high-test")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 16), "SetVenus failed")
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-venus-req-high-test", payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "venus requirement not met", "Should fail when Venus is too high")
	testutil.AssertTrue(t, p.Hand().HasCard("card-venus-req-high-test"), "rejected card stays in hand")
}
func TestVenusIncrease_RaisesGlobalParameter(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := gamecards.Card{
		ID:        "card-venus-raise-test",
		Name:      "Venus Raise Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Tags:      []shared.CardTag{shared.TagVenus},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceVenus, 1, "none"),
				},
			},
		},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{card})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard("card-venus-raise-test")
	venusBefore := testGame.GlobalParameters().Venus()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), "card-venus-raise-test", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing card with Venus output should succeed")
	venusAfter := testGame.GlobalParameters().Venus()
	testutil.AssertEqual(t, venusBefore+2, venusAfter, "Venus should increase by 2 (1 step = 2%)")
}
func TestVenusIncrease_CappedAtMax(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	ctx := context.Background()
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 28), "SetVenus failed")
	actualSteps, err := testGame.GlobalParameters().IncreaseVenus(ctx, 2, "")
	testutil.AssertNoError(t, err, "IncreaseVenus should not error")
	testutil.AssertEqual(t, 1, actualSteps, "Should only increase 1 step (capped at 30)")
	testutil.AssertEqual(t, 30, testGame.GlobalParameters().Venus(), "Venus should be capped at 30")
}
func TestVenusStateCalculator_RequirementValidation(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	ctx := context.Background()
	card := gamecards.Card{
		ID:        "card-venus-state-calc-test",
		Name:      "Venus State Calc Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Tags:      []shared.CardTag{shared.TagVenus},
		Requirements: &gamecards.CardRequirements{
			Items: []gamecards.Requirement{
				{Type: gamecards.RequirementVenus, Min: testutil.IntPtr(10)},
			},
		},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewProductionCondition(shared.ResourceTitaniumProduction, 1, "self-player"),
				},
			},
		},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{card})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard("card-venus-state-calc-test")
	g, _ := repo.Get(ctx, testGame.ID())
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 8), "SetVenus failed")
	state := action.CalculatePlayerCardState(&card, p, g, cardRegistry)
	hasVenusError := false
	for _, e := range state.Errors {
		if e.Code == player.ErrorCodeVenusTooLow {
			hasVenusError = true
		}
	}
	testutil.AssertTrue(t, hasVenusError, "State calculator should report venus-too-low when Venus is 8 and requirement is 10")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 10), "SetVenus failed")
	state = action.CalculatePlayerCardState(&card, p, g, cardRegistry)
	hasVenusError = false
	for _, e := range state.Errors {
		if e.Code == player.ErrorCodeVenusTooLow {
			hasVenusError = true
		}
	}
	testutil.AssertTrue(t, !hasVenusError, "State calculator should NOT report venus-too-low when Venus meets requirement")
}

func TestCometForVenus_OptionalRestrictedRemoval(t *testing.T) {
	for _, tc := range []struct {
		name, card      string
		self, corp      bool
		credits, remove int
		eligible        bool
	}{
		{name: "Venus project", card: "Dirigibles", credits: 10, remove: 4, eligible: true},
		{name: "partial amount", card: "Dirigibles", credits: 10, remove: 2, eligible: true},
		{name: "limited resources", card: "Dirigibles", credits: 2, remove: 2, eligible: true},
		{name: "skip eligible opponent", card: "Dirigibles", credits: 10, eligible: true},
		{name: "self", card: "Dirigibles", self: true, credits: 100, remove: 3, eligible: true},
		{name: "corporation", corp: true, credits: 10, remove: 4, eligible: true},
		{name: "no Venus tag", credits: 10},
		{name: "opponent wild tag", card: "Research Network", credits: 10},
		{name: "event tag", card: "test-venus-event", credits: 10},
		{name: "empty resources", card: "Dirigibles"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g, repo, _, id, otherID := testutil.SetupTwoPlayerGame(t)
			registry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{{ID: "test-venus-event", Name: "Venus event", Type: gamecards.CardTypeEvent, Tags: []shared.CardTag{shared.TagVenus}}})
			p, _ := g.GetPlayer(id)
			targetID := otherID
			if tc.self {
				targetID = id
			}
			target, _ := g.GetPlayer(targetID)
			p.Resources().Set(shared.Resources{Credits: 100})
			target.Resources().Set(shared.Resources{Credits: tc.credits})
			if tc.card != "" {
				cardID := tc.card
				if cardID != "test-venus-event" {
					cardID = testutil.CardID(tc.card)
				}
				card, _ := registry.GetByID(cardID)
				tags := make([]string, len(card.Tags))
				for i, tag := range card.Tags {
					tags[i] = string(tag)
				}
				target.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), tags)
			}
			if tc.corp {
				target.SetCorporationID(testutil.CardID("Morning Star Inc."))
			}
			c := testutil.GetCardByName("Comet For Venus")
			p.Hand().AddCard(c.ID)
			tr := p.Resources().TerraformRating()
			testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "Comet remains playable")
			testutil.AssertEqual(t, 2, g.GlobalParameters().Venus(), "Venus rises once")
			testutil.AssertEqual(t, tr+1, p.Resources().TerraformRating(), "one TR")
			pending := p.Selection().GetPendingResourceRemovalSelection()
			if !tc.eligible {
				testutil.AssertTrue(t, pending == nil, "no removal prompt without a target holding resources")
				return
			}
			testutil.AssertTrue(t, pending != nil, "eligible removal is pending")
			testutil.AssertTrue(t, testutil.ContainsHex(pending.EligiblePlayerIDs, targetID), "target is eligible")
			before := target.Resources().Get().Credits
			ownBefore := p.Resources().Get().Credits
			chosen := targetID
			if tc.remove == 0 {
				chosen = ""
			}
			confirm := confirmation.NewConfirmResourceRemovalAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, pending.ID, chosen, tc.remove), "confirm removal or skip")
			testutil.AssertEqual(t, before-tc.remove, target.Resources().Get().Credits, "exact chosen removal")
			if !tc.self {
				testutil.AssertEqual(t, ownBefore, p.Resources().Get().Credits, "attacker gains nothing")
			}
			testutil.AssertEqual(t, 2, g.GlobalParameters().Venus(), "confirmation does not raise Venus again")
			testutil.AssertEqual(t, tr+1, p.Resources().TerraformRating(), "confirmation does not award more TR")
			testutil.AssertTrue(t, p.Selection().GetPendingResourceRemovalSelection() == nil, "cleared after success")
			testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, pending.ID, chosen, tc.remove), "resource removal selection is no longer pending", "duplicate confirmation rejected")
		})
	}
}

func TestCometForVenus_InvalidRemovalPreservesSelection(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	target, _ := g.GetPlayer(otherID)
	p.Resources().Set(shared.Resources{Credits: 100})
	target.Resources().Set(shared.Resources{Credits: 10})
	c := testutil.GetCardByName("Dirigibles")
	target.PlayedCards().AddCard(c.ID, c.Name, string(c.Type), []string{"venus"})
	comet := testutil.GetCardByName("Comet For Venus")
	p.Hand().AddCard(comet.ID)
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 1), "last action")
	testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, comet.ID, shared.NativePayment(shared.ResourceCredit, comet.Cost), nil, nil, nil, nil, nil), "play")
	pending := p.Selection().GetPendingResourceRemovalSelection()
	testutil.AssertTrue(t, pending != nil, "pending after last action")
	testutil.AssertEqual(t, id, g.CurrentTurn().PlayerID(), "turn waits for removal")
	confirm := confirmation.NewConfirmResourceRemovalAction(repo, registry, nil, testutil.TestLogger())
	for _, tc := range []struct {
		selection, target string
		amount            int
		wantErr           string
	}{
		{pending.ID, id, 4, "not eligible for resource removal"},
		{pending.ID, "missing", 4, "not eligible for resource removal"},
		{"stale", otherID, 4, "resource removal selection is no longer pending"},
		{pending.ID, otherID, 5, "not eligible for resource removal"},
		{pending.ID, otherID, -1, "invalid resource removal amount or target"},
		{pending.ID, "", 1, "invalid resource removal amount or target"},
	} {
		testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, tc.selection, tc.target, tc.amount), tc.wantErr, "invalid selection rejected")
		testutil.AssertEqual(t, 10, target.Resources().Get().Credits, "invalid submission cannot mutate resources")
		testutil.AssertTrue(t, p.Selection().GetPendingResourceRemovalSelection() == pending, "invalid submission preserves selection")
	}
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, otherID, 2), "other turn")
	testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, pending.ID, otherID, 4), "not your turn", "wrong turn rejected")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 0), "restore spent turn")
	target.Resources().Set(shared.Resources{Credits: 1})
	testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, pending.ID, otherID, 4), "not eligible for resource removal", "resource amount is revalidated")
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, pending.ID, "", 0), "skip still possible")
	testutil.AssertEqual(t, otherID, g.CurrentTurn().PlayerID(), "skip releases turn")
}

func TestCometForVenus_MaxVenusWithoutTarget(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	p.Resources().Set(shared.Resources{Credits: 100})
	testutil.AssertNoError(t, g.GlobalParameters().SetVenus(ctx, 30), "max Venus")
	c := testutil.GetCardByName("Comet For Venus")
	p.Hand().AddCard(c.ID)
	tr := p.Resources().TerraformRating()
	testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "playable with no effective outputs")
	testutil.AssertEqual(t, tr, p.Resources().TerraformRating(), "no extra TR at max Venus")
	testutil.AssertEqual(t, 100-c.Cost, p.Resources().Get().Credits, "pay once")
	testutil.AssertTrue(t, p.Selection().GetPendingResourceRemovalSelection() == nil, "no forced removal")
}

func TestVenusCards_WildTagRequirements(t *testing.T) {
	for _, name := range []string{"Luxury Foods", "Mining Quota", "Omnicourt", "Solarnet"} {
		t.Run(name, func(t *testing.T) {
			assertWildTagRequirements(t, name, []shared.CardTag{shared.TagEarth, shared.TagJovian, shared.TagVenus})
		})
	}
}

func TestAerosportTournament_GainCreditsForAllCities(t *testing.T) {
	for _, populated := range []bool{false, true} {
		name := "no cities"
		if populated {
			name = "mixed owners and locations"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			card := testutil.GetCardByName("Aerosport Tournament")
			floaterCard := testutil.GetCardByName("Dirigibles")
			p.PlayedCards().AddCard(floaterCard.ID, floaterCard.Name, string(floaterCard.Type), nil)
			p.Resources().AddToStorage(floaterCard.ID, 5)
			p.Hand().AddCard(card.ID)
			testutil.SetPlayerCredits(ctx, p, 100)
			want := 0
			if populated {
				placeCityCountTestTiles(t, g, id, opponentID)
				want = 3
			}
			before := p.Resources().Get().Credits
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID,
				shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
			testutil.AssertNoError(t, err, "play Aerosport Tournament with five floaters")
			testutil.AssertEqual(t, before-card.Cost+want, p.Resources().Get().Credits, "gain credits for all cities after purchase")
			testutil.AssertEqual(t, 5, p.Resources().GetCardStorage(floaterCard.ID), "floater requirement does not spend floaters")
		})
	}
}

func TestAerosportTournament_StoredFloaterRequirement(t *testing.T) {
	for _, tc := range []struct {
		name     string
		floaters int
		valid    bool
	}{
		{"four", 4, false}, {"five", 5, true}, {"six", 6, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			other, _ := g.GetPlayer(otherID)
			card := testutil.GetCardByName("Aerosport Tournament")
			storageCard := testutil.GetCardByName("Dirigibles")
			animalCard := testutil.GetCardByName("Venusian Animals")
			p.SetCorporationID(testutil.CardID("Celestic"))
			p.Resources().AddToStorage(testutil.CardID("Celestic"), 2)
			p.PlayedCards().AddCard(storageCard.ID, storageCard.Name, string(storageCard.Type), nil)
			p.Resources().AddToStorage(storageCard.ID, tc.floaters-2)
			p.PlayedCards().AddCard(animalCard.ID, animalCard.Name, string(animalCard.Type), nil)
			p.Resources().AddToStorage(animalCard.ID, 10)
			other.PlayedCards().AddCard(storageCard.ID, storageCard.Name, string(storageCard.Type), nil)
			other.Resources().AddToStorage(storageCard.ID, 10)
			p.Hand().AddCard(card.ID)
			testutil.SetPlayerCredits(ctx, p, 100)
			state := action.CalculatePlayerCardState(&card, p, g, registry)
			testutil.AssertEqual(t, tc.valid, state.Available(), "preview counts own floaters across cards and corporation only")
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID,
				shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
			if tc.valid {
				testutil.AssertNoError(t, err, "enough stored floaters")
				testutil.AssertEqual(t, 100-card.Cost, p.Resources().Get().Credits, "pay card cost")
			} else {
				testutil.AssertErrorContains(t, err, "resource requirement not met", "four own floaters cannot meet requirement")
				testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "rejection does not spend credits")
				testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejection keeps card in hand")
			}
			testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(testutil.CardID("Celestic")), "corporation floaters not spent")
			testutil.AssertEqual(t, tc.floaters-2, p.Resources().GetCardStorage(storageCard.ID), "project floaters not spent")
		})
	}
}

func addRegistryPlayedCard(p *player.Player, name string) gamecards.Card {
	c := testutil.GetCardByName(name)
	tags := make([]string, len(c.Tags))
	for i, tag := range c.Tags {
		tags[i] = string(tag)
	}
	p.PlayedCards().AddCard(c.ID, c.Name, string(c.Type), tags)
	return c
}

func TestVenusStorageCards_RealDefinitions(t *testing.T) {
	for _, tc := range []struct {
		name, target                 string
		choice                       *int
		gain, venus, temperature, vp int
	}{
		{"Air-Scrapping Expedition", "Dirigibles", nil, 3, 2, 0, 0},
		{"Atmoscoop", "Atmo Collectors", testutil.IntPtr(0), 2, 0, 4, 1},
		{"Atmoscoop", "Dirigibles", testutil.IntPtr(1), 2, 4, 0, 1},
		{"Freyja Biodomes", "Stratospheric Birds", testutil.IntPtr(0), 2, 0, 0, 2},
		{"Freyja Biodomes", "Extremophiles", testutil.IntPtr(1), 2, 0, 0, 2},
		{"Hydrogen To Venus", "Dirigibles", nil, 2, 2, 0, 0},
	} {
		for _, mode := range []string{"normal", "no storage", "capped", "partial cap", "corporation"} {
			if mode == "corporation" && tc.name != "Atmoscoop" {
				continue
			}
			t.Run(tc.name+"/"+tc.target+"/"+mode, func(t *testing.T) {
				ctx := context.Background()
				g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
				p, _ := g.GetPlayer(id)
				other, _ := g.GetPlayer(otherID)
				p.SetCorporationID("")
				p.Resources().Set(shared.Resources{Credits: 100})
				p.Resources().SetProduction(shared.Production{Energy: 1})
				for _, name := range []string{"Research", "Development Center"} {
					addRegistryPlayedCard(p, name)
				}
				if tc.name == "Hydrogen To Venus" {
					addRegistryPlayedCard(p, "Ganymede Colony")
					addRegistryPlayedCard(p, "Io Mining Industries")
				}
				addRegistryPlayedCard(other, "Titan Shuttles")
				venus, temp := 10, -20
				if mode == "capped" {
					venus, temp = 30, 8
				}
				if mode == "partial cap" {
					venus, temp = 28, 6
				}
				testutil.AssertNoError(t, g.GlobalParameters().SetVenus(ctx, venus), "set Venus")
				testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, temp), "set temperature")
				var targets []string
				if mode != "no storage" {
					target := testutil.GetCardByName(tc.target)
					if mode == "corporation" {
						target = testutil.GetCardByName("Celestic")
						p.SetCorporationID(target.ID)
					} else {
						addRegistryPlayedCard(p, tc.target)
					}
					targets = []string{target.ID}
				}
				c := testutil.GetCardByName(tc.name)
				p.Hand().AddCard(c.ID)
				tr := p.Resources().TerraformRating()
				beforeVP := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil).CardVP
				err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), tc.choice, targets, nil, nil, nil)
				testutil.AssertNoError(t, err, "play actual card")
				wantVenus, wantTemp := min(30, venus+tc.venus), min(8, temp+tc.temperature)
				testutil.AssertEqual(t, wantVenus, g.GlobalParameters().Venus(), "Venus steps")
				testutil.AssertEqual(t, wantTemp, g.GlobalParameters().Temperature(), "temperature steps")
				testutil.AssertEqual(t, tr+(wantVenus-venus+wantTemp-temp)/2, p.Resources().TerraformRating(), "TR only for actual steps")
				testutil.AssertEqual(t, 100-c.Cost, p.Resources().Get().Credits, "cost paid once")
				if len(targets) > 0 {
					testutil.AssertEqual(t, tc.gain, p.Resources().GetCardStorage(targets[0]), "exact storage gain")
				}
				wantProduction := shared.Production{Energy: 1}
				if tc.name == "Freyja Biodomes" {
					wantProduction = shared.Production{Credits: 2}
				}
				testutil.AssertEqual(t, wantProduction, p.Resources().Production(), "shared production applied once")
				if len(targets) > 0 {
					p.Resources().AddToStorage(targets[0], -tc.gain)
				}
				testutil.AssertEqual(t, beforeVP+tc.vp, gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil).CardVP, "card VP excludes host resource points")
			})
		}
	}
}

func TestVenusStorageCards_InvalidTargetsAreAtomic(t *testing.T) {
	for _, name := range []string{"Air-Scrapping Expedition", "Atmoscoop", "Freyja Biodomes", "Hydrogen To Venus"} {
		for _, invalid := range []string{"wrong storage", "opponent", "not in play", "unknown", "missing", "wrong tag"} {
			if name == "Atmoscoop" && invalid == "wrong tag" {
				continue
			}
			t.Run(name+"/"+invalid, func(t *testing.T) {
				ctx := context.Background()
				g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
				p, _ := g.GetPlayer(id)
				other, _ := g.GetPlayer(otherID)
				p.SetCorporationID("")
				p.Resources().Set(shared.Resources{Credits: 100})
				p.Resources().SetProduction(shared.Production{Energy: 1})
				for _, n := range []string{"Research", "Development Center", "Ganymede Colony"} {
					addRegistryPlayedCard(p, n)
				}
				testutil.AssertNoError(t, g.GlobalParameters().SetVenus(ctx, 10), "Venus")
				var choice *int
				if name == "Atmoscoop" || name == "Freyja Biodomes" {
					choice = testutil.IntPtr(0)
				}
				targetName := "Dirigibles"
				if name == "Freyja Biodomes" {
					targetName = "Stratospheric Birds"
				}
				if invalid == "wrong storage" {
					targetName = "Venusian Insects"
				}
				if invalid == "wrong tag" {
					targetName = "Atmo Collectors"
					if name == "Freyja Biodomes" {
						targetName = "Pets"
					}
				}
				target := testutil.GetCardByName(targetName)
				targetID := target.ID
				switch invalid {
				case "opponent":
					addRegistryPlayedCard(other, targetName)
				case "not in play":
					p.Hand().AddCard(target.ID)
				case "unknown":
					targetID = "not-a-card"
				default:
					addRegistryPlayedCard(p, targetName)
				}
				if invalid == "missing" {
					targetID = ""
				}
				c := testutil.GetCardByName(name)
				p.Hand().AddCard(c.ID)
				beforeResources, beforeProduction := p.Resources().Get(), p.Resources().Production()
				beforeStorage := p.Resources().Storage()
				tr, temp, actions := p.Resources().TerraformRating(), g.GlobalParameters().Temperature(), g.CurrentTurn().ActionsRemaining()
				err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), choice, []string{targetID}, nil, nil, nil)
				wantErr := map[string]string{
					"missing":       "select a card for resource storage",
					"not in play":   "target card is not yours or is not in play",
					"opponent":      "target card is not yours or is not in play",
					"unknown":       "target card is not yours or is not in play",
					"wrong storage": "card cannot store that resource",
					"wrong tag":     "target card does not match selectors",
				}[invalid]
				testutil.AssertErrorContains(t, err, wantErr, "invalid target rejected")
				testutil.AssertEqual(t, beforeResources, p.Resources().Get(), "resources unchanged")
				testutil.AssertEqual(t, beforeProduction, p.Resources().Production(), "production unchanged")
				if !reflect.DeepEqual(beforeStorage, p.Resources().Storage()) {
					t.Fatal("storage changed after rejection")
				}
				testutil.AssertEqual(t, 10, g.GlobalParameters().Venus(), "Venus unchanged")
				testutil.AssertEqual(t, temp, g.GlobalParameters().Temperature(), "temperature unchanged")
				testutil.AssertEqual(t, tr, p.Resources().TerraformRating(), "TR unchanged")
				testutil.AssertEqual(t, actions, g.CurrentTurn().ActionsRemaining(), "action unchanged")
				testutil.AssertTrue(t, p.Hand().HasCard(c.ID) && !p.PlayedCards().Contains(c.ID), "card stays in hand")
			})
		}
	}
}

func TestVenusStorageCards_RequirementsAndChoices(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		science, venus, energy int
		choice                 *int
		wantErr                string
	}{
		{"Atmoscoop", 2, 10, 1, testutil.IntPtr(0), "tag requirements"}, {"Atmoscoop", 3, 10, 1, testutil.IntPtr(0), ""},
		{"Atmoscoop", 3, 10, 1, nil, "select a valid card choice"}, {"Atmoscoop", 3, 10, 1, testutil.IntPtr(-1), "select a valid card choice"}, {"Atmoscoop", 3, 10, 1, testutil.IntPtr(2), "select a valid card choice"},
		{"Freyja Biodomes", 0, 8, 1, testutil.IntPtr(0), "venus requirement not met"}, {"Freyja Biodomes", 0, 10, 1, testutil.IntPtr(0), ""},
		{"Freyja Biodomes", 0, 10, 0, testutil.IntPtr(0), "insufficient energy-production"}, {"Freyja Biodomes", 0, 10, 1, nil, "select a valid card choice"}, {"Freyja Biodomes", 0, 10, 1, testutil.IntPtr(2), "select a valid card choice"},
	} {
		choiceName := "missing"
		if tc.choice != nil {
			choiceName = fmt.Sprint(*tc.choice)
		}
		t.Run(fmt.Sprintf("%s/%d/%d/%d/%s", tc.name, tc.science, tc.venus, tc.energy, choiceName), func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			p.Resources().Set(shared.Resources{Credits: 100})
			p.Resources().SetProduction(shared.Production{Energy: tc.energy})
			if tc.science >= 2 {
				addRegistryPlayedCard(p, "Research")
			}
			if tc.science >= 3 {
				addRegistryPlayedCard(p, "Development Center")
			}
			testutil.AssertNoError(t, g.GlobalParameters().SetVenus(ctx, tc.venus), "Venus")
			c := testutil.GetCardByName(tc.name)
			p.Hand().AddCard(c.ID)
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), tc.choice, nil, nil, nil, nil)
			if tc.wantErr == "" {
				testutil.AssertNoError(t, err, "valid boundary")
			} else {
				testutil.AssertErrorContains(t, err, tc.wantErr, "invalid requirement or choice")
				testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "no payment")
				testutil.AssertEqual(t, shared.Production{Energy: tc.energy}, p.Resources().Production(), "no production change")
				testutil.AssertEqual(t, tc.venus, g.GlobalParameters().Venus(), "no parameter change")
				testutil.AssertTrue(t, p.Hand().HasCard(c.ID), "card remains")
			}
		})
	}
}

func TestHydrogenToVenus_JovianScaling(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			other, _ := g.GetPlayer(otherID)
			p.SetCorporationID("")
			p.Resources().Set(shared.Resources{Credits: 100})
			for _, name := range []string{"Ganymede Colony", "Io Mining Industries"}[:count] {
				addRegistryPlayedCard(p, name)
			}
			addRegistryPlayedCard(other, "Titan Shuttles")
			host := addRegistryPlayedCard(p, "Dirigibles")
			c := testutil.GetCardByName("Hydrogen To Venus")
			p.Hand().AddCard(c.ID)
			var targets []string
			if count > 0 {
				targets = []string{host.ID}
			}
			testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, targets, nil, nil, nil), "play")
			testutil.AssertEqual(t, count, p.Resources().GetCardStorage(host.ID), "only own Jovian tags")
			testutil.AssertEqual(t, 2, g.GlobalParameters().Venus(), "Venus raised even with zero tags")
		})
	}
}

func TestDawnCity_RealPlacementAndRequirements(t *testing.T) {
	for _, mode := range []string{"valid", "three science", "no energy", "occupied", "claimed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			p.Resources().Set(shared.Resources{Credits: 100})
			p.Resources().SetProduction(shared.Production{Energy: 1})
			addRegistryPlayedCard(p, "Research")
			addRegistryPlayedCard(p, "Development Center")
			if mode != "three science" {
				addRegistryPlayedCard(p, "Physics Complex")
			}
			if mode == "no energy" {
				p.Resources().SetProduction(shared.Production{})
			}
			var dawn shared.HexPosition
			found := false
			for _, tile := range g.Board().Tiles() {
				for _, tag := range tile.Tags {
					if tag == "dawn-city" {
						dawn = tile.Coordinates
						found = true
					}
				}
			}
			if !found {
				t.Fatal("Dawn City space missing")
			}
			if mode == "occupied" {
				testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, dawn, board.TileOccupant{Type: shared.ResourceCityTile}, otherID), "occupy reserved space")
			}
			if mode == "claimed" {
				testutil.AssertNoError(t, g.Board().ReserveTile(ctx, dawn, otherID), "reserve Dawn City")
			}
			c := testutil.GetCardByName("Dawn City")
			p.Hand().AddCard(c.ID)
			testutil.AssertEqual(t, mode == "valid", action.CalculatePlayerCardState(&c, p, g, registry).Available(), "preview matches playability")
			before := p.Resources().Production()
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil)
			if mode != "valid" {
				wantErr := map[string]string{
					"three science": "tag requirements",
					"no energy":     "insufficient energy-production",
					"occupied":      "No valid city placements",
					"claimed":       "No valid city placements",
				}[mode]
				testutil.AssertErrorContains(t, err, wantErr, "reject invalid play")
				testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "no payment")
				testutil.AssertEqual(t, before, p.Resources().Production(), "no production change")
				testutil.AssertTrue(t, p.Hand().HasCard(c.ID), "card remains")
				return
			}
			testutil.AssertNoError(t, err, "play Dawn City")
			testutil.AssertEqual(t, shared.Production{Titanium: 1}, p.Resources().Production(), "production changes")
			selection := g.GetPendingTileSelection(id)
			if selection == nil {
				t.Fatal("missing placement")
			}
			testutil.AssertEqual(t, 1, len(selection.AvailableHexes), "only named space")
			testutil.AssertEqual(t, dawn.String(), selection.AvailableHexes[0], "Dawn City, not Ganymede")
			_, err = tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, dawn.String())
			testutil.AssertNoError(t, err, "complete placement")
			tile, err := g.Board().GetTile(dawn)
			testutil.AssertNoError(t, err, "tile")
			testutil.AssertEqual(t, shared.ResourceCityTile, tile.OccupiedBy.Type, "city occupant")
			scored := false
			for _, detail := range gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil).CardVPDetails {
				if detail.CardID == c.ID {
					scored = true
					testutil.AssertEqual(t, 3, detail.TotalVP, "Dawn City 3 VP")
				}
			}
			testutil.AssertTrue(t, scored, "Dawn City has a scoring entry")
		})
	}
}
