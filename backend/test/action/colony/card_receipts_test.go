package colony_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	colonyaction "openmars/internal/action/colony"
	confirmation "openmars/internal/action/confirmation"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestPlutoTradeCardReceipts(t *testing.T) {
	for _, free := range []bool{false, true} {
		for _, ownColony := range []bool{false, true} {
			t.Run(fmt.Sprintf("free=%v/ownColony=%v", free, ownColony), func(t *testing.T) {
				g, repo, colonies, traderID, ownerID := setupColonyGame(t)
				registry := testutil.CreateTestCardRegistry()
				trader, _ := g.GetPlayer(traderID)
				owner, _ := g.GetPlayer(ownerID)
				owners := []string{ownerID}
				if ownColony {
					owners = append(owners, traderID)
				}
				setupColony(g, "pluto", 3, owners)
				g.InitDeck([]string{"001", "002", "003", "004"}, nil, nil)
				beforeTrader, beforeOwner := trader.Hand().Cards(), owner.Hand().Cards()
				ctx := context.Background()
				source := "Trade: Pluto"
				if free {
					source = "Free Trade: Pluto"
					trader.Selection().SetPendingFreeTradeSelection(&shared.PendingFreeTradeSelection{AvailableColonyIDs: []string{"pluto"}})
					action := confirmation.NewConfirmFreeTradeAction(repo, registry, colonies, game.NewInMemoryGameStateRepository())
					testutil.AssertNoError(t, action.Execute(ctx, g.ID(), traderID, "pluto", 0), "Free Pluto trade")
				} else {
					action := colonyaction.NewTradeAction(repo, colonies, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger())
					testutil.AssertNoError(t, action.Execute(ctx, g.ID(), traderID, "pluto", colonyaction.TradePaymentEnergy, 0, testutil.TradePayment(colonyaction.TradePaymentEnergy)), "Paid Pluto trade")
				}
				expectedReceipts, expectedCards := 1, 2
				if ownColony {
					expectedReceipts, expectedCards = 2, 3
				}
				traderView := dto.ToGameDto(g, registry, traderID, colonies).CurrentPlayer
				ownerView := dto.ToGameDto(g, registry, ownerID, colonies).CurrentPlayer
				testutil.AssertEqual(t, expectedReceipts, len(traderView.CardReceipts), "One receipt per trader grant")
				testutil.AssertEqual(t, 1, len(ownerView.CardReceipts), "One private colony bonus receipt")
				testutil.AssertEqual(t, source, traderView.CardReceipts[0].Source, "Trade receipt source")
				testutil.AssertEqual(t, "Colony bonus: Pluto", ownerView.CardReceipts[0].Source, "Owner receipt source")
				testutil.AssertEqual(t, expectedCards, len(trader.Hand().Cards())-len(beforeTrader), "Trader cards already granted")
				testutil.AssertEqual(t, 1, len(owner.Hand().Cards())-len(beforeOwner), "Owner card already granted")
				for _, view := range []dto.PlayerDto{traderView, ownerView} {
					seen := map[string]bool{}
					for _, receipt := range view.CardReceipts {
						for _, card := range receipt.Cards {
							if seen[card.ID] {
								t.Fatalf("card %s has duplicate receipts", card.ID)
							}
							seen[card.ID] = true
						}
					}
				}
				ack := confirmation.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
				traderCards := slices.Clone(trader.Hand().Cards())
				for range 2 {
					testutil.AssertNoError(t, ack.AcknowledgeReceipt(ctx, g.ID(), traderID, traderView.CardReceipts[0].ID), "Acknowledge receipt idempotently")
				}
				testutil.AssertEqual(t, expectedReceipts-1, len(trader.Selection().CardReceipts()), "Only named receipt dismissed")
				testutil.AssertTrue(t, slices.Equal(traderCards, trader.Hand().Cards()), "Acknowledgement does not grant cards again")
				testutil.AssertEqual(t, 1, len(owner.Selection().CardReceipts()), "Other player's receipt retained")
			})
		}
	}
}

func TestPlutoNoCardsCreatesNoReceipt(t *testing.T) {
	for _, emptyDeck := range []bool{false, true} {
		t.Run(fmt.Sprintf("emptyDeck=%v", emptyDeck), func(t *testing.T) {
			g, repo, colonies, playerID, _ := setupColonyGame(t)
			position := 0
			if emptyDeck {
				position = 3
				g.InitDeck(nil, nil, nil)
			}
			setupColony(g, "pluto", position, nil)
			action := colonyaction.NewTradeAction(repo, colonies, testutil.CreateTestCardRegistry(), game.NewInMemoryGameStateRepository(), testutil.TestLogger())
			testutil.AssertNoError(t, action.Execute(context.Background(), g.ID(), playerID, "pluto", colonyaction.TradePaymentEnergy, 0, testutil.TradePayment(colonyaction.TradePaymentEnergy)), "Pluto trade")
			p, _ := g.GetPlayer(playerID)
			testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "No empty receipt")
		})
	}
}

func TestPlutoCardPlacementCreatesReceipt(t *testing.T) {
	g, _, colonies, playerID, _ := setupColonyGame(t)
	setupColony(g, "pluto", 3, nil)
	g.InitDeck([]string{"001", "002"}, nil, nil)
	p, _ := g.GetPlayer(playerID)
	definition, err := colonies.GetByID("pluto")
	testutil.AssertNoError(t, err, "Pluto definition")
	before := len(p.Hand().Cards())
	err = colonyaction.PlaceColonyOnTile(context.Background(), g, p, definition, g.Colonies().GetState("pluto"), testutil.CreateTestCardRegistry(), "Mining Colony", testutil.TestLogger())
	testutil.AssertNoError(t, err, "Card colony placement")
	receipts := p.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "One receipt for the placement reward")
	testutil.AssertEqual(t, "Mining Colony: Pluto", receipts[0].Source, "Placement source preserved")
	testutil.AssertEqual(t, 2, len(receipts[0].Cards), "Both reward cards shown")
	testutil.AssertEqual(t, before+2, len(p.Hand().Cards()), "Both reward cards already granted")
}
