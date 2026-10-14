package confirmation_test

import (
	"context"
	"testing"

	confirmAction "openmars/internal/action/confirmation"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestConfirmProductionCards_RandomBuy_DisabledRejects(t *testing.T) {
	g, repo, cardRegistry, p1ID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	logger := testutil.TestLogger()

	settings := g.Settings()
	settings.AllowRandomBuy = false
	g.UpdateSettings(ctx, settings)

	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw), "update phase")
	drawn, err := g.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "draw cards")
	testutil.AssertNoError(t, g.SetProductionPhase(ctx, p1ID, &shared.ProductionPhase{
		AvailableCards:    drawn,
		SelectionComplete: false,
	}), "set production phase")

	p1, _ := g.GetPlayer(p1ID)
	p1.Resources().Set(shared.Resources{Credits: 10})

	action := confirmAction.NewConfirmProductionCardsAction(repo, cardRegistry, nil, logger)
	err = action.Execute(ctx, g.ID(), p1ID, []string{}, true, shared.NativePayment(shared.ResourceCredit, 3))
	testutil.AssertErrorContains(t, err, "random buy is not enabled", "randomBuy should reject when setting is off")
	testutil.AssertEqual(t, 10, p1.Resources().Get().Credits, "credits unchanged")
}

func TestConfirmProductionCards_RandomBuy_BuysOneCardForThreeCredits(t *testing.T) {
	g, repo, cardRegistry, p1ID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	logger := testutil.TestLogger()

	settings := g.Settings()
	settings.AllowRandomBuy = true
	g.UpdateSettings(ctx, settings)

	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw), "update phase")
	drawn, err := g.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "draw cards")
	testutil.AssertNoError(t, g.SetProductionPhase(ctx, p1ID, &shared.ProductionPhase{
		AvailableCards:    drawn,
		SelectionComplete: false,
	}), "set production phase")

	p1, _ := g.GetPlayer(p1ID)
	p1.Resources().Set(shared.Resources{Credits: 10})
	handBefore := len(p1.Hand().Cards())

	action := confirmAction.NewConfirmProductionCardsAction(repo, cardRegistry, nil, logger)
	err = action.Execute(ctx, g.ID(), p1ID, []string{}, true, shared.NativePayment(shared.ResourceCredit, 3))
	testutil.AssertNoError(t, err, "randomBuy should succeed when setting is on")

	testutil.AssertEqual(t, handBefore+1, len(p1.Hand().Cards()), "hand should grow by exactly one card")
	testutil.AssertEqual(t, 7, p1.Resources().Get().Credits, "should pay 3 credits for one random buy")

	pickedID := p1.Hand().Cards()[len(p1.Hand().Cards())-1]
	receipts := p1.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "Unseen purchase creates one receipt")
	testutil.AssertEqual(t, "Random research purchase", receipts[0].Source, "Random purchase source")
	testutil.AssertEqual(t, pickedID, receipts[0].Cards[0], "Receipt shows the purchased card")
	for _, id := range drawn {
		testutil.AssertTrue(t, id != pickedID, "random buy must pull a fresh card from the deck, not one of the cards the player already rejected")
	}
}

func TestConfirmProductionCards_RandomBuy_RejectsWithSelection(t *testing.T) {
	g, repo, cardRegistry, p1ID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	logger := testutil.TestLogger()

	settings := g.Settings()
	settings.AllowRandomBuy = true
	g.UpdateSettings(ctx, settings)

	testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw), "update phase")
	drawn, err := g.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "draw cards")
	testutil.AssertNoError(t, g.SetProductionPhase(ctx, p1ID, &shared.ProductionPhase{
		AvailableCards:    drawn,
		SelectionComplete: false,
	}), "set production phase")

	p1, _ := g.GetPlayer(p1ID)
	p1.Resources().Set(shared.Resources{Credits: 10})

	action := confirmAction.NewConfirmProductionCardsAction(repo, cardRegistry, nil, logger)
	err = action.Execute(ctx, g.ID(), p1ID, []string{drawn[0]}, true, shared.NativePayment(shared.ResourceCredit, 3))
	testutil.AssertErrorContains(t, err, "random buy requires an empty selection", "randomBuy with non-empty selection should reject")
	testutil.AssertEqual(t, 10, p1.Resources().Get().Credits, "credits unchanged")
}
