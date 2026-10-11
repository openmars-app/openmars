package behavior_test

import (
	"context"
	"testing"

	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestMisc_ExtraActionsGranted(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := testGame.GetPlayer(playerID)

	actionsBefore := testGame.CurrentTurn().ActionsRemaining()
	output := shared.NewMiscCondition(shared.ResourceExtraActions, 2, "none")
	applyOutputs(t, p, testGame, cardRegistry, output)

	actionsAfter := testGame.CurrentTurn().ActionsRemaining()
	testutil.AssertEqual(t, actionsBefore+2, actionsAfter, "remaining actions should increase by 2")
}

func TestMisc_BonusTagsWithPer(t *testing.T) {
	testGame, _, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := testGame.GetPlayer(playerID)

	jovianTag := shared.TagJovian

	// Add 3 played cards with jovian tag using synthetic cards
	syntheticCards := []gamecards.Card{
		{ID: "jov-a", Name: "JovianA", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagJovian}},
		{ID: "jov-b", Name: "JovianB", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagJovian}},
		{ID: "jov-c", Name: "JovianC", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagJovian}},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(syntheticCards)

	p.PlayedCards().AddCard("jov-a", "JovianA", "automated", []string{"jovian"})
	p.PlayedCards().AddCard("jov-b", "JovianB", "automated", []string{"jovian"})
	p.PlayedCards().AddCard("jov-c", "JovianC", "automated", []string{"jovian"})

	// One bonus jovian tag per jovian tag the player has, as on Home Schooled.
	output := &shared.MiscCondition{
		ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceBonusTags, Amount: 1, Target: "none"},
		Per: &shared.PerCondition{
			ResourceType: shared.ResourceType("tag"),
			Amount:       1,
			Tag:          &jovianTag,
		},
		Selectors: []shared.Selector{
			{Tags: []shared.CardTag{shared.TagJovian}},
		},
	}
	applyOutputs(t, p, testGame, cardRegistry, output)

	testutil.AssertEqual(t, 3, p.BonusTags()[shared.TagJovian], "one bonus tag per jovian tag")
}

func TestMisc_FreeTrade(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := testGame.GetPlayer(playerID)
	settings := testGame.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	testGame.UpdateSettings(context.Background(), settings)
	testGame.Colonies().SetStates([]*colony.ColonyState{
		{DefinitionID: "luna", MarkerPosition: 1},
		{DefinitionID: "ganymede", MarkerPosition: 1, TradedThisGen: true},
	})
	freeTrade := shared.NewMiscCondition(shared.ResourceFreeTrade, 1, "self-player")

	applyOutputs(t, p, testGame, cardRegistry, freeTrade)
	testutil.AssertTrue(t, p.Selection().GetPendingFreeTradeSelection() == nil, "without a trade fleet there is no free trade")

	testGame.Colonies().AddTradeFleets(playerID, 1)
	applyOutputs(t, p, testGame, cardRegistry, freeTrade)
	pending := p.Selection().GetPendingFreeTradeSelection()
	testutil.AssertTrue(t, pending != nil, "a player with a trade fleet should be offered a free trade")
	testutil.AssertEqual(t, 1, len(pending.AvailableColonyIDs), "only colonies not traded with this generation are offered")
	testutil.AssertEqual(t, "luna", pending.AvailableColonyIDs[0], "luna is the colony still open for trade")
}
