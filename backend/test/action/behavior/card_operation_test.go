package behavior_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"openmars/internal/action"
	cardAction "openmars/internal/action/card"
	confirmAction "openmars/internal/action/confirmation"
	"openmars/internal/events"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// ============================================================================
// Card discard tests
// ============================================================================

// --- Mars University (073) ---
// "Effect: When you play a science tag, including this, you may discard a card from hand to draw a card."

func TestMarsUniversity_CreatesDiscardSelectionOnScienceTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
		Tags: []shared.CardTag{shared.TagBuilding, shared.TagScience},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// Give player some cards in hand to discard
	owner.Hand().AddCard("some-card-1")
	owner.Hand().AddCard("some-card-2")

	// Register Mars University passive effect with card-discard input
	effect := shared.CardEffect{
		CardID:        "card-mars-university",
		CardName:      "Mars University",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type: "tag-played",
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagScience}},
						},
					},
				},
			},
			Inputs: []shared.BehaviorCondition{
				&shared.CardOperationCondition{
					ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardDiscard, Amount: 1, Target: "self-player"},
					Optional:      true,
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)

	// Subscribe the passive effect to events
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	// Verify no pending selection before event
	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Should have no pending discard selection before science tag event")

	// Publish a science tag played event
	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	// Should have a pending card discard selection
	selection := owner.Selection().GetPendingBehaviorResolutions()[0]
	testutil.AssertTrue(t, selection != nil, "Should have a pending discard selection after science tag event")
	testutil.AssertEqual(t, 0, selection.MinCards, "MinCards should be 0 (optional)")
	testutil.AssertEqual(t, 1, selection.MaxCards, "MaxCards should be 1")
	testutil.AssertEqual(t, "Mars University", selection.Source, "Source should be Mars University")
	testutil.AssertEqual(t, "card-mars-university", selection.SourceCardID, "SourceCardID should match")
	testutil.AssertEqual(t, 1, len(selection.PendingOutputs), "Should have 1 pending output")
	testutil.AssertEqual(t, shared.ResourceCardDraw, selection.PendingOutputs[0].GetResourceType(), "Pending output should be card-draw")

	_ = repo // suppress unused warning
}

func TestMarsUniversity_RetainsDecisionWhenNoCardsInHand(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// Do NOT add cards to hand - player has empty hand

	effect := shared.CardEffect{
		CardID:        "card-mars-university",
		CardName:      "Mars University",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type: "tag-played",
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagScience}},
						},
					},
				},
			},
			Inputs: []shared.BehaviorCondition{
				&shared.CardOperationCondition{
					ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardDiscard, Amount: 1, Target: "self-player"},
					Optional:      true,
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	// Publish science tag event
	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	testutil.AssertEqual(t, 1, len(owner.Selection().GetPendingBehaviorResolutions()), "Keep the optional decision: another effect may draw cards first")
}

func TestMarsUniversity_DoesNotTriggerOnNonScienceTag(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Hand().AddCard("some-card-1")

	effect := shared.CardEffect{
		CardID:        "card-mars-university",
		CardName:      "Mars University",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type: "tag-played",
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagScience}},
						},
					},
				},
			},
			Inputs: []shared.BehaviorCondition{
				&shared.CardOperationCondition{
					ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardDiscard, Amount: 1, Target: "self-player"},
					Optional:      true,
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	// Publish a building tag event (not science)
	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "building",
	})

	// Should NOT trigger for building tag
	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Mars University should not trigger on non-science tags")
}

// --- ConfirmCardDiscardAction Tests ---

func TestConfirmCardDiscard_DiscardAndDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// Give player a card to discard
	owner.Hand().AddCard("card-to-discard")
	owner.Hand().AddCard("card-to-keep")

	handBefore := len(owner.Hand().Cards())
	testutil.AssertEqual(t, 2, handBefore, "Should have 2 cards before discard")

	// Set up pending discard selection
	owner.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards:     0,
		MaxCards:     1,
		Source:       "Mars University",
		SourceCardID: "card-mars-university",
		PendingOutputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	})

	// Execute confirm discard
	confirmDiscard := confirmAction.NewConfirmCardDiscardAction(repo, cardRegistry, nil, logger)
	err := confirmDiscard.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, []string{"card-to-discard"})
	testutil.AssertNoError(t, err, "Confirm card discard should succeed")

	// Verify card was removed from hand
	handCards := owner.Hand().Cards()
	found := false
	for _, id := range handCards {
		if id == "card-to-discard" {
			found = true
		}
	}
	testutil.AssertFalse(t, found, "Discarded card should not be in hand")

	// Verify pending selection was cleared
	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Pending discard selection should be cleared after confirmation")
	receipts := owner.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "Discard reward creates one receipt")
	testutil.AssertEqual(t, "Mars University", receipts[0].Source, "Discard receipt preserves source")
	testutil.AssertEqual(t, 1, len(receipts[0].Cards), "One replacement card shown")
}

func TestConfirmCardDiscard_SkipOptionalDiscard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Hand().AddCard("card-to-keep")

	// Set up pending discard with min=0 (optional)
	owner.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards:     0,
		MaxCards:     1,
		Source:       "Mars University",
		SourceCardID: "card-mars-university",
		PendingOutputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	})

	// Skip discard by sending empty list
	confirmDiscard := confirmAction.NewConfirmCardDiscardAction(repo, cardRegistry, nil, logger)
	err := confirmDiscard.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, []string{})
	testutil.AssertNoError(t, err, "Skipping optional discard should succeed")

	// Hand should still have the card
	testutil.AssertEqual(t, 1, len(owner.Hand().Cards()), "Hand should still have 1 card")

	// Pending selection should be cleared
	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Pending discard selection should be cleared even when skipping")
}

func TestConfirmCardDiscard_RejectsNonHandCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Hand().AddCard("real-card")

	owner.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards:     0,
		MaxCards:     1,
		Source:       "Mars University",
		SourceCardID: "card-mars-university",
		PendingOutputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	})

	// Try to discard a card not in hand
	confirmDiscard := confirmAction.NewConfirmCardDiscardAction(repo, cardRegistry, nil, logger)
	err := confirmDiscard.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, []string{"fake-card"})
	testutil.AssertErrorContains(t, err, "not in player's hand", "Should reject discard of card not in hand")
}

func TestConfirmCardDiscard_RejectsWithoutPendingSelection(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// No pending selection set

	confirmDiscard := confirmAction.NewConfirmCardDiscardAction(repo, cardRegistry, nil, logger)
	err := confirmDiscard.Execute(ctx, testGame.ID(), owner.ID(), "missing", []string{})
	testutil.AssertErrorContains(t, err, "no pending card discard selection", "Should reject confirm when no pending selection exists")
}

func TestConfirmCardDiscard_RejectsTooManyCards(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	marsUniCard := gamecards.Card{
		ID:   "card-mars-university",
		Name: "Mars University",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{marsUniCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Hand().AddCard("card-1")
	owner.Hand().AddCard("card-2")

	owner.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards:     0,
		MaxCards:     1,
		Source:       "Mars University",
		SourceCardID: "card-mars-university",
		PendingOutputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	})

	// Try to discard 2 cards when max is 1
	confirmDiscard := confirmAction.NewConfirmCardDiscardAction(repo, cardRegistry, nil, logger)
	err := confirmDiscard.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, []string{"card-1", "card-2"})
	testutil.AssertErrorContains(t, err, "can discard at most 1", "Should reject discarding too many cards")
}

// ============================================================================
// All opponents card operation tests
// ============================================================================

func createAllOpponentsDrawTestCard() gamecards.Card {
	return gamecards.Card{
		ID:        "card-all-opponents-draw-test",
		Name:      "All Opponents Draw Test",
		Type:      gamecards.CardTypeAutomated,
		GameModes: []string{"classic"},
		Cost:      5,
		Tags:      []shared.CardTag{shared.TagScience},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardOperationCondition(shared.ResourceCardDraw, 2, "self-player"),
					shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "all-opponents"),
				},
			},
		},
	}
}

func TestPlayCard_AllOpponentsDrawCard_TwoPlayers(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	additionalCards := []gamecards.Card{createAllOpponentsDrawTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player1 := players[0]
	player2 := players[1]
	player1.SetCorporationID(testutil.CardID("Tharsis Republic"))
	player2.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player1.ID(), 2), "set current turn")

	player1.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player1.Hand().AddCard("card-all-opponents-draw-test")

	player1HandBefore := player1.Hand().CardCount()
	player2HandBefore := player2.Hand().CardCount()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), player1.ID(), "card-all-opponents-draw-test", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing all-opponents draw card should succeed")

	// Player 1: played 1 card (-1) and drew 2 cards (+2) = net +1
	player1HandAfter := player1.Hand().CardCount()
	testutil.AssertEqual(t, player1HandBefore+1, player1HandAfter,
		"Player 1 hand should increase by 1 (played 1, drew 2)")

	// Player 2 (opponent): drew 1 card (+1)
	player2HandAfter := player2.Hand().CardCount()
	testutil.AssertEqual(t, player2HandBefore+1, player2HandAfter,
		"Player 2 (opponent) should have drawn 1 card")
}

func TestPlayCard_AllOpponentsDrawCard_ThreePlayers(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 3)
	additionalCards := []gamecards.Card{createAllOpponentsDrawTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player1 := players[0]
	player2 := players[1]
	player3 := players[2]
	player1.SetCorporationID(testutil.CardID("Tharsis Republic"))
	player2.SetCorporationID(testutil.CardID("Tharsis Republic"))
	player3.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player1.ID(), 2), "set current turn")

	player1.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player1.Hand().AddCard("card-all-opponents-draw-test")

	player2HandBefore := player2.Hand().CardCount()
	player3HandBefore := player3.Hand().CardCount()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), player1.ID(), "card-all-opponents-draw-test", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing all-opponents draw card should succeed")

	// Both opponents should have drawn 1 card each
	player2HandAfter := player2.Hand().CardCount()
	testutil.AssertEqual(t, player2HandBefore+1, player2HandAfter,
		"Player 2 (opponent) should have drawn 1 card")

	player3HandAfter := player3.Hand().CardCount()
	testutil.AssertEqual(t, player3HandBefore+1, player3HandAfter,
		"Player 3 (opponent) should have drawn 1 card")
}

func TestPlayCard_AllOpponentsDrawCard_SoloMode(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	additionalCards := []gamecards.Card{createAllOpponentsDrawTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player1 := players[0]
	player1.SetCorporationID(testutil.CardID("Tharsis Republic"))

	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, player1.ID(), 2), "set current turn")

	player1.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player1.Hand().AddCard("card-all-opponents-draw-test")

	player1HandBefore := player1.Hand().CardCount()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), player1.ID(), "card-all-opponents-draw-test", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Playing all-opponents draw card in solo mode should succeed")

	// Solo: played 1 card (-1) and drew 2 cards (+2), no opponents = net +1
	player1HandAfter := player1.Hand().CardCount()
	testutil.AssertEqual(t, player1HandBefore+1, player1HandAfter,
		"Player 1 hand should increase by 1 in solo mode (played 1, drew 2, no opponents)")
}

func TestCardOperation_DrawWithSelectors(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)

	p, _ := testGame.GetPlayer(playerID)
	handBefore := p.Hand().CardCount()

	output := &shared.CardOperationCondition{
		ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardDraw, Amount: 1, Target: "self-player"},
		Selectors:     []shared.Selector{{Tags: []shared.CardTag{shared.TagScience}}},
	}

	applyOutputs(t, p, testGame, cardRegistry, output)

	handAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handBefore+1, handAfter, "Player should have drawn 1 card")
	receipts := p.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "Draw-until creates one receipt")
	testutil.AssertEqual(t, 1, len(receipts[0].Cards), "Only the matched card is received")
	matched, err := cardRegistry.GetByID(receipts[0].Cards[0])
	testutil.AssertNoError(t, err, "Matched card")
	testutil.AssertTrue(t, gamecards.MatchesAnySelector(matched, output.Selectors), "Receipt excludes rejected search cards")
}

func TestCardOperation_VariableAmountDiscard(t *testing.T) {
	input := &shared.CardOperationCondition{
		ConditionBase:  shared.ConditionBase{ResourceType: shared.ResourceCardDiscard, Amount: 1, Target: "self-player"},
		VariableAmount: true,
	}

	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
		Inputs:   []shared.BehaviorCondition{input},
	}

	testutil.AssertTrue(t, behavior.Inputs[0] != nil, "Behavior input should be constructed")
	testutil.AssertEqual(t, shared.ResourceCardDiscard, behavior.Inputs[0].GetResourceType(), "Resource type should be card-discard")
	testutil.AssertTrue(t, shared.IsVariableAmount(behavior.Inputs[0]), "VariableAmount should be true")
}

func TestCardOperation_DrawFromNearlyEmptyDeck(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)

	// Drain the deck to leave very few cards
	available := testGame.Deck().GetAvailableCardCount()
	if available > 2 {
		_, err := testGame.Deck().DrawProjectCards(ctx, available-2)
		testutil.AssertNoError(t, err, "Draining deck should succeed")
	}

	remaining := testGame.Deck().GetAvailableCardCount()
	handBefore := p.Hand().CardCount()

	// Try to draw 5 cards when only a few remain
	output := shared.NewCardOperationCondition(shared.ResourceCardDraw, 5, "self-player")
	applyOutputs(t, p, testGame, cardRegistry, output)

	// The draw may fail (not enough cards even after reshuffle), which is logged
	// as a warning and returns nil. The player gets 0 cards from a failed draw.
	// Either way, no crash should occur.
	handAfter := p.Hand().CardCount()
	testutil.AssertTrue(t, handAfter >= handBefore,
		"Hand should not decrease")
	_ = remaining
}

func TestCardTake_OptionalAndBuySelection(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(fmt.Sprintf("optional-%v", optional), func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			testutil.SetPlayerCredits(context.Background(), p, 20)
			take := shared.NewCardOperationCondition(shared.ResourceCardTake, 1, "self-player")
			take.Optional = optional
			applier := gamecards.NewBehaviorApplier(p, g, "Take and buy", testutil.TestLogger()).WithCardRegistry(registry)
			testutil.AssertNoError(t, applier.ApplyOutputs(context.Background(), []shared.BehaviorCondition{
				shared.NewCardOperationCondition(shared.ResourceCardPeek, 3, "self-player"), take, shared.NewCardOperationCondition(shared.ResourceCardBuy, 1, "self-player"),
			}), "create selection")
			selection := p.Selection().GetPendingCardDrawSelection()
			if selection == nil {
				t.Fatal("missing selection")
			}
			confirm := confirmAction.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
			testutil.AssertErrorContains(t, confirm.Execute(context.Background(), g.ID(), id, []string{selection.AvailableCards[0]}, []string{selection.AvailableCards[0]}, shared.NativePayment(shared.ResourceCredit, len([]string{selection.AvailableCards[0]})*3)), "duplicate selected card", "reject duplicate across free and bought lists")
			testutil.AssertEqual(t, 20, p.Resources().Get().Credits, "invalid selection never charges")
			if optional {
				testutil.AssertEqual(t, 0, selection.MinFreeTakeCount, "optional minimum")
				testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, nil, nil, shared.NativePayment(shared.ResourceCredit, 0*3)), "decline all optional cards")
			} else {
				testutil.AssertErrorContains(t, confirm.Execute(context.Background(), g.ID(), id, nil, []string{selection.AvailableCards[0]}, shared.NativePayment(shared.ResourceCredit, len([]string{selection.AvailableCards[0]})*3)), "must take at least 1", "buy cannot replace mandatory free take")
				testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, []string{selection.AvailableCards[0]}, []string{selection.AvailableCards[1]}, shared.NativePayment(shared.ResourceCredit, len([]string{selection.AvailableCards[1]})*3)), "take and buy")
				testutil.AssertEqual(t, 17, p.Resources().Get().Credits, "charge only bought card")
			}
		})
	}
}

func TestTriggeringCardStorage_ContextAndValidation(t *testing.T) {
	g, _, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	opponent, _ := g.GetPlayer(opponentID)
	card := testutil.GetCardByName("Fish")
	opponent.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), []string{"animal"})
	output := shared.NewCardStorageCondition(shared.ResourceCardResource, 2, "triggering-card")
	apply := func(cardID, ownerID string, condition shared.BehaviorCondition) error {
		return gamecards.NewBehaviorApplier(p, g, "Generic effect", testutil.TestLogger()).WithCardRegistry(registry).WithSourceCardID(testutil.CardID("Viral Enhancers")).WithTriggeringCard(cardID, ownerID).ApplyOutputs(context.Background(), []shared.BehaviorCondition{condition})
	}
	testutil.AssertErrorContains(t, apply("", "", output), "missing triggering card context", "missing context cannot fall back to source")
	testutil.AssertErrorContains(t, apply(card.ID, id, output), "triggering card is not in play", "wrong owner rejected")
	testutil.AssertErrorContains(t, apply(testutil.CardID("Tardigrades"), id, output), "triggering card is not in play", "unplayed target rejected")
	testutil.AssertErrorContains(t, apply(card.ID, opponentID, shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "triggering-card")), "cannot store microbe", "incompatible storage rejected")
	restricted := shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "triggering-card")
	restricted.Selectors = []shared.Selector{{Tags: []shared.CardTag{shared.TagScience}}}
	testutil.AssertErrorContains(t, apply(card.ID, opponentID, restricted), "does not match selectors", "selectors enforced")
	testutil.AssertEqual(t, 0, opponent.Resources().GetCardStorage(card.ID), "invalid outputs do not mutate storage")
	g.ClearTriggeredEffects()
	testutil.AssertNoError(t, apply(card.ID, opponentID, output), "generic resource uses retained owner and storage type")
	testutil.AssertEqual(t, 2, opponent.Resources().GetCardStorage(card.ID), "reward goes to triggering owner")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(card.ID), "effect owner does not receive storage")
	effects := g.GetTriggeredEffects()
	testutil.AssertEqual(t, 1, len(effects), "one reward notification")
	testutil.AssertEqual(t, "animal", effects[0].CalculatedOutputs[0].ResourceType, "notification uses concrete storage type")
}

func TestPreludeDrawConfirmationPreservesNestedCardSelection(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	nested := gamecards.Card{ID: "nested-prelude", Name: "Nested Prelude", Type: gamecards.CardTypePrelude,
		Behaviors: []shared.CardBehavior{{Triggers: []shared.Trigger{{Type: "auto"}}, Outputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardPeek, 2, "self-player"),
			shared.NewCardOperationCondition(shared.ResourceCardTake, 1, "self-player"),
		}}}}
	cards := registry.GetAll()
	cards = append(cards, nested)
	registry = gamecards.NewInMemoryCardRegistry(cards)
	g.InitDeck([]string{"001", "002"}, nil, nil)
	p, _ := g.GetPlayer(id)
	p.Selection().SetPendingCardDrawSelection(&shared.PendingCardDrawSelection{AvailableCards: []string{nested.ID}, FreeTakeCount: 1, MinFreeTakeCount: 1, PlayAsPrelude: true})
	testutil.AssertNoError(t, g.SetForcedFirstAction(ctx, id, &shared.ForcedFirstAction{State: "resolving", CorporationID: "test"}), "resolving action")
	confirm := confirmAction.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, []string{nested.ID}, nil, shared.Payment{}), "play selected prelude")
	pending := p.Selection().GetPendingCardDrawSelection()
	testutil.AssertTrue(t, pending != nil && !pending.PlayAsPrelude, "nested selection survives")
	testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "not complete yet")
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, []string{pending.AvailableCards[0]}, nil, shared.Payment{}), "take nested card")
	testutil.AssertTrue(t, g.GetForcedFirstAction(id) == nil, "first action resolved")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "consume exactly once")
}

func TestCardPlayCondition_RoundTripAndValidation(t *testing.T) {
	original := &shared.CardOperationCondition{
		ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardPlay, Amount: 1, Target: "self-player"},
		CostReduction: 25, Selectors: []shared.Selector{{Tags: []shared.CardTag{shared.TagScience}}},
	}
	data, err := json.Marshal(original)
	testutil.AssertNoError(t, err, "marshal")
	var behavior shared.CardBehavior
	testutil.AssertNoError(t, json.Unmarshal([]byte(`{"outputs":[`+string(data)+`]}`), &behavior), "unmarshal categorized output")
	got, ok := behavior.Outputs[0].(*shared.CardOperationCondition)
	testutil.AssertTrue(t, ok, "card operation category")
	testutil.AssertEqual(t, 25, got.CostReduction, "discount survives round trip")
	testutil.AssertEqual(t, 0, len(shared.ValidateResourceCondition(got, false)), "valid output")
	got.CostReduction = -1
	testutil.AssertTrue(t, len(shared.ValidateResourceCondition(got, false)) > 0, "negative discount rejected")
	got.CostReduction = 25
	got.Amount = 2
	testutil.AssertTrue(t, len(shared.ValidateResourceCondition(got, false)) > 0, "single play contract")
}
