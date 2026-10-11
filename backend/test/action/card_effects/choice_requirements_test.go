package card_effects_test

import (
	"context"
	"testing"

	cardAction "openmars/internal/action/card"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// tagPtr returns a pointer to a CardTag

// createChoiceRequirementsTestCard creates a card similar to Io Sulphur Research
// with choice 0 having no requirements and choice 1 requiring 3+ venus tags
func createChoiceRequirementsTestCard() gamecards.Card {
	return gamecards.Card{
		ID:          "card-choice-req-test",
		Name:        "Choice Requirements Test",
		Type:        gamecards.CardTypeAutomated,
		GameModes:   []string{"classic"},
		Cost:        10,
		Tags:        []shared.CardTag{shared.TagScience},
		Description: gamecards.CardDescription{{Type: gamecards.DescriptionSectionGeneric, Text: "Draw 1 card, or draw 3 cards if you have 3+ venus tags."}},
		Behaviors: []shared.CardBehavior{
			{
				Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
				Choices: []shared.Choice{
					{
						// Choice 0: always available, draw 1 card
						Outputs: []shared.BehaviorCondition{
							shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
						},
					},
					{
						// Choice 1: requires 3+ venus tags, draw 3 cards
						Outputs: []shared.BehaviorCondition{
							shared.NewCardOperationCondition(shared.ResourceCardDraw, 3, "self-player"),
						},
						Requirements: &shared.ChoiceRequirements{
							Items: []shared.ChoiceRequirement{
								{Type: "tags", Min: testutil.IntPtr(3), Tag: testutil.TagPtr(shared.TagVenus)},
							},
						},
					},
				},
			},
		},
	}
}

func TestChoiceRequirements_Choice0AlwaysAvailable(t *testing.T) {
	// Setup: Create game with player who has the test card
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	additionalCards := []gamecards.Card{createChoiceRequirementsTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, player.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard("card-choice-req-test")

	// Player has NO venus tags → choice 0 (no requirements) should work
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	choiceIndex := 0
	err = playCardAction.Execute(ctx, testGame.ID(), player.ID(), "card-choice-req-test", payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 with no requirements should succeed")

	// Verify card was played
	testutil.AssertFalse(t, player.Hand().HasCard("card-choice-req-test"), "Card should be removed from hand")
}

func TestChoiceRequirements_Choice1RejectedWithoutEnoughTags(t *testing.T) {
	// Setup: Create game with player who has the test card
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	additionalCards := []gamecards.Card{createChoiceRequirementsTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, player.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard("card-choice-req-test")

	// Player has 0 venus tags → choice 1 (requires 3+ venus tags) should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	choiceIndex := 1
	err = playCardAction.Execute(ctx, testGame.ID(), player.ID(), "card-choice-req-test", payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "choice 1 requirements not met: tag requirements", "Choice 1 should fail without venus tags")

	// Card should still be in hand since the play failed
	testutil.AssertTrue(t, player.Hand().HasCard("card-choice-req-test"), "Card should remain in hand after failed play")
}

func TestChoiceRequirements_Choice1SucceedsWithEnoughTags(t *testing.T) {
	// Setup: Create game with player who has the test card AND 3+ venus tags
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)

	// Create 3 venus-tagged cards for the player to have played
	venusCard1 := gamecards.Card{
		ID: "venus-1", Name: "Venus 1", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}
	venusCard2 := gamecards.Card{
		ID: "venus-2", Name: "Venus 2", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}
	venusCard3 := gamecards.Card{
		ID: "venus-3", Name: "Venus 3", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}

	additionalCards := []gamecards.Card{createChoiceRequirementsTestCard(), venusCard1, venusCard2, venusCard3}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, player.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard("card-choice-req-test")

	// Add 3 venus-tagged cards to played cards
	player.PlayedCards().AddCard("venus-1", "Venus 1", "automated", []string{"venus"})
	player.PlayedCards().AddCard("venus-2", "Venus 2", "automated", []string{"venus"})
	player.PlayedCards().AddCard("venus-3", "Venus 3", "automated", []string{"venus"})

	// Player has 3 venus tags → choice 1 (requires 3+ venus tags) should succeed
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	choiceIndex := 1
	err = playCardAction.Execute(ctx, testGame.ID(), player.ID(), "card-choice-req-test", payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 should succeed with 3 venus tags")

	// Verify card was played
	testutil.AssertFalse(t, player.Hand().HasCard("card-choice-req-test"), "Card should be removed from hand")
}

func TestChoiceRequirements_Choice0DrawsCard(t *testing.T) {
	// Verify that choosing option 0 (draw 1 card) actually adds a card to the player's hand
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	additionalCards := []gamecards.Card{createChoiceRequirementsTestCard()}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, player.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard("card-choice-req-test")

	handBefore := player.Hand().CardCount()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	choiceIndex := 0
	err = playCardAction.Execute(ctx, testGame.ID(), player.ID(), "card-choice-req-test", payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 should succeed")

	// The test card was removed from hand (-1), and choice 0 should draw 1 card (+1)
	// Net effect: hand count should remain the same
	handAfter := player.Hand().CardCount()
	testutil.AssertEqual(t, handBefore, handAfter, "Hand count should be unchanged (played 1, drew 1)")
}

func TestChoiceRequirements_Choice1DrawsThreeCards(t *testing.T) {
	// Verify that choosing option 1 (draw 3 cards) actually adds 3 cards to hand
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)

	venusCard1 := gamecards.Card{
		ID: "venus-1", Name: "Venus 1", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}
	venusCard2 := gamecards.Card{
		ID: "venus-2", Name: "Venus 2", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}
	venusCard3 := gamecards.Card{
		ID: "venus-3", Name: "Venus 3", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}

	additionalCards := []gamecards.Card{createChoiceRequirementsTestCard(), venusCard1, venusCard2, venusCard3}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, player.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard("card-choice-req-test")

	// Add 3 venus-tagged cards to played cards
	player.PlayedCards().AddCard("venus-1", "Venus 1", "automated", []string{"venus"})
	player.PlayedCards().AddCard("venus-2", "Venus 2", "automated", []string{"venus"})
	player.PlayedCards().AddCard("venus-3", "Venus 3", "automated", []string{"venus"})

	handBefore := player.Hand().CardCount()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	choiceIndex := 1
	err = playCardAction.Execute(ctx, testGame.ID(), player.ID(), "card-choice-req-test", payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 should succeed with 3 venus tags")

	// The test card was removed from hand (-1), and choice 1 should draw 3 cards (+3)
	// Net effect: hand count should increase by 2
	handAfter := player.Hand().CardCount()
	testutil.AssertEqual(t, handBefore+2, handAfter, "Hand count should increase by 2 (played 1, drew 3)")
}

func TestChoiceRequirements_Choice1FailsWithTwoTags(t *testing.T) {
	// Setup: Same as above but with only 2 venus tags (below the 3 minimum)
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)

	venusCard1 := gamecards.Card{
		ID: "venus-1", Name: "Venus 1", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}
	venusCard2 := gamecards.Card{
		ID: "venus-2", Name: "Venus 2", Type: gamecards.CardTypeAutomated,
		GameModes: []string{"classic"}, Tags: []shared.CardTag{shared.TagVenus},
	}

	additionalCards := []gamecards.Card{createChoiceRequirementsTestCard(), venusCard1, venusCard2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	logger := testutil.TestLogger()
	ctx := context.Background()

	players := testGame.GetAllPlayers()
	player := players[0]
	player.SetCorporationID(testutil.CardID("Tharsis Republic"))

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "UpdateStatus failed")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseAction)
	testutil.AssertNoError(t, err, "UpdatePhase failed")
	err = testGame.SetCurrentTurn(ctx, player.ID(), 2)
	testutil.AssertNoError(t, err, "SetCurrentTurn failed")

	player.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	player.Hand().AddCard("card-choice-req-test")

	// Add only 2 venus-tagged cards
	player.PlayedCards().AddCard("venus-1", "Venus 1", "automated", []string{"venus"})
	player.PlayedCards().AddCard("venus-2", "Venus 2", "automated", []string{"venus"})

	// Player has 2 venus tags → choice 1 (requires 3+) should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	choiceIndex := 1
	err = playCardAction.Execute(ctx, testGame.ID(), player.ID(), "card-choice-req-test", payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "choice 1 requirements not met: tag requirements", "Choice 1 should fail with only 2 venus tags")
}
