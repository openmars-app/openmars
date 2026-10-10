package action_test

import (
	"context"
	"slices"
	"testing"

	"openmars/internal/action/confirmation"
	"openmars/internal/action/turn_management"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/deck"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestDeckRecycling_UnselectedCardsFromCardDraw(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	log := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)

	// Draw 4 cards to set up a pending card draw selection
	drawnCards, err := testGame.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "draw project cards")

	// Set up pending card draw selection (buy up to 4 cards at 3 MC each)
	testutil.SetPlayerCredits(ctx, p, 100)
	p.Selection().SetPendingCardDrawSelection(&shared.PendingCardDrawSelection{
		AvailableCards: drawnCards,
		FreeTakeCount:  0,
		MaxBuyCount:    4,
		CardBuyCost:    3,
		Source:         "research-phase",
	})

	// Player buys only 1 card, the other 3 should be discarded
	action := confirmation.NewConfirmCardDrawAction(repo, cardRegistry, log)
	err = action.Execute(ctx, testGame.ID(), playerID, []string{}, []string{drawnCards[0]}, shared.NativePayment(shared.ResourceCredit, len([]string{drawnCards[0]})*3))
	testutil.AssertNoError(t, err, "confirm card draw")

	discardPile := testGame.Deck().DiscardPile()
	testutil.AssertEqual(t, 3, len(discardPile), "discard pile should have 3 unselected cards")

	for _, cardID := range drawnCards[1:] {
		testutil.AssertTrue(t, slices.Contains(discardPile, cardID),
			"discard pile should contain unselected card "+cardID)
	}
}

func TestDeckRecycling_DiscardedCardsFromHand(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	log := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)

	// Add cards to player hand
	cardsInHand := []string{"card-power-plant", "card-asteroid", "card-water-import"}
	for _, cardID := range cardsInHand {
		p.Hand().AddCard(cardID)
	}

	// Set up pending card discard selection
	p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard",
		MinCards: 1,
		MaxCards: 2,
		Source:   "test-discard",
	})

	// Player discards 2 cards
	cardsToDiscard := []string{"card-power-plant", "card-asteroid"}
	action := confirmation.NewConfirmCardDiscardAction(repo, cardRegistry, nil, log)
	err := action.Execute(ctx, testGame.ID(), playerID, p.Selection().GetPendingBehaviorResolutions()[0].ID, cardsToDiscard)
	testutil.AssertNoError(t, err, "confirm card discard")

	discardPile := testGame.Deck().DiscardPile()
	testutil.AssertEqual(t, 2, len(discardPile), "discard pile should have 2 discarded cards")

	for _, cardID := range cardsToDiscard {
		testutil.AssertTrue(t, slices.Contains(discardPile, cardID),
			"discard pile should contain discarded card "+cardID)
	}
}

func TestDeckRecycling_SoldPatents(t *testing.T) {
	testGame, repo, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	log := testutil.TestLogger()

	p, _ := testGame.GetPlayer(playerID)

	// Add cards to player hand
	cardsInHand := []string{"card-power-plant", "card-asteroid"}
	for _, cardID := range cardsInHand {
		p.Hand().AddCard(cardID)
	}

	// Set up pending card selection for sell patents
	p.Selection().SetPendingCardSelection(&shared.PendingCardSelection{
		AvailableCards: cardsInHand,
		MinCards:       0,
		MaxCards:       len(cardsInHand),
		CardRewards:    map[string]int{"card-power-plant": 1, "card-asteroid": 1},
		Source:         "sell-patents",
	})

	action := confirmation.NewConfirmSellPatentsAction(repo, nil, log)
	err := action.Execute(ctx, testGame.ID(), playerID, cardsInHand)
	testutil.AssertNoError(t, err, "confirm sell patents")

	discardPile := testGame.Deck().DiscardPile()
	testutil.AssertEqual(t, 2, len(discardPile), "discard pile should have 2 sold cards")

	for _, cardID := range cardsInHand {
		testutil.AssertTrue(t, slices.Contains(discardPile, cardID),
			"discard pile should contain sold card "+cardID)
	}
}

func TestDeckRecycling_UnselectedStartingCards(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()
	ctx := context.Background()
	log := testutil.TestLogger()

	err := testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "update game status")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseStartingSelection)
	testutil.AssertNoError(t, err, "update game phase")

	playerID := "player-1"

	availableCards := []string{
		"001", "002", "003", "004", "005",
		"006", "007", "008", "009", "010",
	}

	err = testGame.SetSelectStartingCardsPhase(ctx, playerID, &shared.SelectStartingCardsPhase{
		AvailableCards: availableCards,
	})
	testutil.AssertNoError(t, err, "set starting cards phase")

	// Also set corporation phase (required by combined action)
	err = testGame.SetSelectCorporationPhase(ctx, playerID, &shared.SelectCorporationPhase{
		AvailableCorporations: []string{"B08", "B06"},
	})
	testutil.AssertNoError(t, err, "set corporation phase")

	p, _ := testGame.GetPlayer(playerID)
	testutil.SetPlayerCredits(ctx, p, 100)

	// Player selects corporation B08 and 3 out of 10 project cards
	selectedCards := []string{"001", "002", "003"}
	action := turn_management.NewSelectStartingChoicesAction(repo, cardRegistry, nil, log)
	err = action.Execute(ctx, testGame.ID(), playerID, "B08", []string{}, selectedCards, shared.NativePayment(shared.ResourceCredit, len(selectedCards)*3))
	testutil.AssertNoError(t, err, "select starting cards")

	discardPile := testGame.Deck().DiscardPile()
	testutil.AssertEqual(t, 7, len(discardPile), "discard pile should have 7 unselected project cards")

	// Verify the selected cards are NOT in the discard pile
	for _, cardID := range selectedCards {
		testutil.AssertFalse(t, slices.Contains(discardPile, cardID),
			"discard pile should not contain selected card "+cardID)
	}

	// Verify unselected cards ARE in the discard pile
	for _, cardID := range availableCards {
		if !slices.Contains(selectedCards, cardID) {
			testutil.AssertTrue(t, slices.Contains(discardPile, cardID),
				"discard pile should contain unselected card "+cardID)
		}
	}

	// Verify corporations are NOT in the discard pile
	testutil.AssertFalse(t, slices.Contains(discardPile, "B08"),
		"discard pile should not contain selected corporation")
	testutil.AssertFalse(t, slices.Contains(discardPile, "B06"),
		"discard pile should not contain unselected corporation")
}

func TestDeckRecycling_AutoShuffleOnEmptyDrawPile(t *testing.T) {
	ctx := context.Background()

	// Create a deck with only 2 project cards in draw pile
	ds := testutil.NewTestDataStoreWithGame(t, "test-game")
	gameDeck := deck.NewDeck(ds, "test-game", 42, []string{"card-a", "card-b"}, nil, nil)

	// Draw all cards to empty the draw pile
	drawn, err := gameDeck.DrawProjectCards(ctx, 2)
	testutil.AssertNoError(t, err, "initial draw")
	testutil.AssertEqual(t, 2, len(drawn), "should draw 2 cards")
	testutil.AssertEqual(t, 0, gameDeck.GetAvailableCardCount(), "draw pile should be empty")

	// Add cards to discard pile
	err = gameDeck.Discard(ctx, []string{"card-c", "card-d", "card-e"})
	testutil.AssertNoError(t, err, "discard cards")
	testutil.AssertEqual(t, 3, len(gameDeck.DiscardPile()), "discard pile should have 3 cards")

	// Drawing should auto-shuffle the discard pile back in
	drawn, err = gameDeck.DrawProjectCards(ctx, 2)
	testutil.AssertNoError(t, err, "draw after auto-shuffle")
	testutil.AssertEqual(t, 2, len(drawn), "should draw 2 cards after auto-shuffle")
	testutil.AssertEqual(t, 1, gameDeck.GetAvailableCardCount(), "draw pile should have 1 remaining card")
	testutil.AssertEqual(t, 0, len(gameDeck.DiscardPile()), "discard pile should be empty after shuffle")
	testutil.AssertEqual(t, 1, gameDeck.ShuffleCount(), "shuffle count should be 1")
}

func TestDeckRecycling_PreludeCardsNeverReshuffledIntoProjectDeck(t *testing.T) {
	ctx := context.Background()

	projectCards := []string{"card-a", "card-b"}
	preludeCards := []string{"P01", "P02", "P03", "P04"}
	ds2 := testutil.NewTestDataStoreWithGame(t, "test-game")
	gameDeck := deck.NewDeck(ds2, "test-game", 42, projectCards, nil, preludeCards)

	// Draw all project cards to empty the draw pile
	drawn, err := gameDeck.DrawProjectCards(ctx, 2)
	testutil.AssertNoError(t, err, "initial draw")
	testutil.AssertEqual(t, 2, len(drawn), "should draw 2 cards")

	// Simulate unselected preludes being removed (the fix) and project cards being discarded
	err = gameDeck.Remove(ctx, []string{"P03", "P04"})
	testutil.AssertNoError(t, err, "remove unselected preludes")
	err = gameDeck.Discard(ctx, drawn)
	testutil.AssertNoError(t, err, "discard project cards")

	// Trigger reshuffle by drawing from empty draw pile
	redrawn, err := gameDeck.DrawProjectCards(ctx, 2)
	testutil.AssertNoError(t, err, "draw after reshuffle")
	testutil.AssertEqual(t, 2, len(redrawn), "should draw 2 cards after reshuffle")

	// Verify no prelude cards appear in the drawn cards
	for _, cardID := range redrawn {
		testutil.AssertFalse(t, slices.Contains(preludeCards, cardID),
			"reshuffled draw pile must not contain prelude card "+cardID)
	}

	// Verify prelude cards are in removed pile, not discard pile
	removedCards := gameDeck.RemovedCards()
	testutil.AssertTrue(t, slices.Contains(removedCards, "P03"), "P03 should be in removed cards")
	testutil.AssertTrue(t, slices.Contains(removedCards, "P04"), "P04 should be in removed cards")
}

func TestDeckRecycling_AutoShuffleFailsWhenBothPilesEmpty(t *testing.T) {
	ctx := context.Background()

	// Create a deck with only 1 project card
	ds3 := testutil.NewTestDataStoreWithGame(t, "test-game")
	gameDeck := deck.NewDeck(ds3, "test-game", 42, []string{"card-a"}, nil, nil)

	// Draw the only card
	_, err := gameDeck.DrawProjectCards(ctx, 1)
	testutil.AssertNoError(t, err, "initial draw")

	// Both draw pile and discard pile are empty — draw should fail
	_, err = gameDeck.DrawProjectCards(ctx, 1)
	testutil.AssertErrorContains(t, err, "not enough cards available", "draw from empty deck should fail")
}

func TestDeckRecycling_RealDeckNeverDealsPreludeOrCorporation(t *testing.T) {
	ctx := context.Background()
	registry := testutil.GetCardDB()

	projectCardIDs, corpIDs, preludeIDs := gamecards.GetCardIDs(registry)
	testutil.AssertTrue(t, len(projectCardIDs) > 0, "should have project cards")
	testutil.AssertTrue(t, len(preludeIDs) > 0, "should have prelude cards")
	testutil.AssertTrue(t, len(corpIDs) > 0, "should have corporation cards")

	ds4 := testutil.NewTestDataStoreWithGame(t, "test-game")
	gameDeck := deck.NewDeck(ds4, "test-game", 42, projectCardIDs, corpIDs, preludeIDs)

	// Build lookup sets for non-project card types
	preludeSet := make(map[string]bool, len(preludeIDs))
	for _, id := range preludeIDs {
		preludeSet[id] = true
	}
	corpSet := make(map[string]bool, len(corpIDs))
	for _, id := range corpIDs {
		corpSet[id] = true
	}

	// Draw every project card from the deck
	drawn, err := gameDeck.DrawProjectCards(ctx, len(projectCardIDs))
	testutil.AssertNoError(t, err, "draw all project cards")

	for _, cardID := range drawn {
		card, lookupErr := registry.GetByID(cardID)
		testutil.AssertNoError(t, lookupErr, "card should exist in registry: "+cardID)
		testutil.AssertFalse(t, card.Type == gamecards.CardTypePrelude,
			"drew prelude card from project deck: "+cardID+" ("+card.Name+")")
		testutil.AssertFalse(t, card.Type == gamecards.CardTypeCorporation,
			"drew corporation card from project deck: "+cardID+" ("+card.Name+")")
	}

	// Discard all drawn cards, then reshuffle and draw them all again
	err = gameDeck.Discard(ctx, drawn)
	testutil.AssertNoError(t, err, "discard all cards")

	redrawn, err := gameDeck.DrawProjectCards(ctx, len(projectCardIDs))
	testutil.AssertNoError(t, err, "redraw all project cards after reshuffle")

	for _, cardID := range redrawn {
		testutil.AssertFalse(t, preludeSet[cardID],
			"reshuffled deck contains prelude card: "+cardID)
		testutil.AssertFalse(t, corpSet[cardID],
			"reshuffled deck contains corporation card: "+cardID)
	}
}
