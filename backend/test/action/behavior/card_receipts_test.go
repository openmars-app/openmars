package behavior_test

import (
	"context"
	"strconv"
	"testing"

	confirmation "openmars/internal/action/confirmation"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestCardDrawReceiptDoesNotDependOnSourceClassification(t *testing.T) {
	for _, source := range []shared.SourceType{"", shared.SourceTypeCardPlay, shared.SourceTypeCardAction, shared.SourceTypeCorporationFirstAction, shared.SourceTypePassiveEffect, shared.SourceTypeGlobalBonus, "future-effect"} {
		t.Run(string(source), func(t *testing.T) {
			g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			g.InitDeck([]string{"001", "002"}, nil, nil)
			p, _ := g.GetPlayer(id)
			p.Hand().SetCards(nil)
			applier := gamecards.NewBehaviorApplier(p, g, "Reward", testutil.TestLogger()).WithSourceCardID("185").WithSourceType(source).WithCardRegistry(registry)
			testutil.AssertNoError(t, applier.ApplyOutputs(context.Background(), []shared.BehaviorCondition{shared.NewCardOperationCondition(shared.ResourceCardDraw, 2, "self-player")}), "Draw")
			receipts := p.Selection().CardReceipts()
			testutil.AssertEqual(t, 1, len(receipts), "Exactly one grant receipt")
			testutil.AssertEqual(t, 2, len(receipts[0].Cards), "Both drawn cards shown")
			testutil.AssertEqual(t, "Reward", receipts[0].Source, "Source preserved")
			testutil.AssertEqual(t, "185", receipts[0].SourceCardID, "Source card preserved")
		})
	}
}

func TestOpponentDrawsCreateOnlyRecipientReceipts(t *testing.T) {
	g, _, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	g.InitDeck([]string{"001"}, nil, nil)
	p, _ := g.GetPlayer(id)
	other, _ := g.GetPlayer(otherID)
	other.Hand().SetCards(nil)
	applier := gamecards.NewBehaviorApplier(p, g, "Generous reward", testutil.TestLogger()).WithCardRegistry(registry)
	testutil.AssertNoError(t, applier.ApplyOutputs(context.Background(), []shared.BehaviorCondition{shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "all-opponents")}), "Opponent draw")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "Actor receives no receipt for opponent's cards")
	testutil.AssertEqual(t, 1, len(other.Selection().CardReceipts()), "Opponent gets their own receipt")
	testutil.AssertEqual(t, "001", other.Selection().CardReceipts()[0].Cards[0], "Exact granted card")
}

func TestTemporaryCardsAndReviewedSelectionsDoNotCreateReceipts(t *testing.T) {
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	g.InitDeck([]string{"001", "002"}, nil, nil)
	p, _ := g.GetPlayer(id)
	p.Hand().SetCards(nil)
	applier := gamecards.NewBehaviorApplier(p, g, "Review cards", testutil.TestLogger()).WithCardRegistry(registry)
	created, err := applier.ApplyCardDrawOutputs(context.Background(), []shared.BehaviorCondition{
		shared.NewCardOperationCondition(shared.ResourceCardPeek, 2, "self-player"),
		shared.NewCardOperationCondition(shared.ResourceCardTake, 1, "self-player"),
	})
	testutil.AssertNoError(t, err, "Peek")
	testutil.AssertTrue(t, created, "Review selection created")
	testutil.AssertEqual(t, 0, p.Hand().CardCount(), "Offered cards are not owned")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "Peeking creates no receipt")
	confirm := confirmation.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
	testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, []string{"001"}, nil, shared.Payment{}), "Take reviewed card")
	testutil.AssertEqual(t, 1, p.Hand().CardCount(), "Confirmed card granted")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "Reviewed choice does not show twice")
}

func TestFailedAndZeroCardDrawsDoNotCreateReceipts(t *testing.T) {
	for _, amount := range []int{0, 2} {
		t.Run(strconv.Itoa(amount), func(t *testing.T) {
			g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			g.InitDeck(nil, nil, nil)
			p, _ := g.GetPlayer(id)
			before := p.Hand().CardCount()
			applier := gamecards.NewBehaviorApplier(p, g, "No cards", testutil.TestLogger()).WithCardRegistry(registry)
			testutil.AssertNoError(t, applier.ApplyOutputs(context.Background(), []shared.BehaviorCondition{shared.NewCardOperationCondition(shared.ResourceCardDraw, amount, "self-player")}), "Empty draw is handled")
			testutil.AssertEqual(t, before, p.Hand().CardCount(), "No cards added")
			testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "No receipt for absent cards")
		})
	}
}
