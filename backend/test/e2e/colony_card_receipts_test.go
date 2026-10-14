package e2e_test

import (
	"slices"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/test/e2e/harness"
	"openmars/test/testutil"
)

func TestColonyDrawsBroadcastPrivateCardReceipts(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Play(t, func(setup *dto.GameSetupDto) {
		setup.GameModeID = "classic"
		setup.Features = []string{"colonies"}
		setup.DevelopmentMode = true
	}, "alice", "bob")
	owner := game.Ready(t)
	hasPluto := func(state dto.GameDto) bool {
		return slices.ContainsFunc(state.Colonies, func(colony dto.ColonyDto) bool { return colony.ID == "pluto" })
	}
	// Aridor can bring Pluto into play if the initial random colony selection omitted it.
	if !hasPluto(owner.State(t)) {
		owner.Admin(t, dto.AdminCommandTypeSetCorporation, map[string]any{"playerId": owner.PlayerID(), "corporationId": testutil.CardID("Aridor")})
		owner.AwaitState(t, "additional colony selection", func(state dto.GameDto) bool {
			pending := state.CurrentPlayer.PendingColonySelection
			return pending != nil && slices.Contains(pending.AvailableColonyIDs, "pluto")
		})
		owner.Send(t, dto.MessageTypeActionConfirmColonyPlacement, map[string]any{"colonyId": "pluto"})
		owner.AwaitState(t, "Pluto in play", hasPluto)
	}
	owner.Admin(t, dto.AdminCommandTypeSetCurrentTurn, map[string]any{"playerId": owner.PlayerID()})
	owner.Admin(t, dto.AdminCommandTypeSetResources, map[string]any{"playerId": owner.PlayerID(), "resources": map[string]any{"credits": 100}})
	beforeBuild := len(owner.State(t).CurrentPlayer.Cards)
	owner.Send(t, dto.MessageTypeActionColonyBuild, map[string]any{"colonyId": "pluto", "payment": harness.Pay(map[string]int{"credit": 17})})
	built := owner.AwaitState(t, "colony build card receipt", func(state dto.GameDto) bool { return len(state.CurrentPlayer.CardReceipts) == 1 })
	receipt := built.CurrentPlayer.CardReceipts[0]
	if receipt.Source != "Build Colony: Pluto" || len(receipt.Cards) != 2 || len(built.CurrentPlayer.Cards) != beforeBuild+2 {
		t.Fatalf("colony build receipt does not match cards granted: %+v", receipt)
	}
	owner.Send(t, dto.MessageTypeActionAcknowledgeCardReceipt, map[string]any{"receiptId": receipt.ID})
	owner.AwaitState(t, "build receipt dismissed", func(state dto.GameDto) bool { return len(state.CurrentPlayer.CardReceipts) == 0 })

	trader := game.Others(owner)[0]
	owner.Admin(t, dto.AdminCommandTypeSetCurrentTurn, map[string]any{"playerId": trader.PlayerID()})
	trader = game.Ready(t)
	trader.Admin(t, dto.AdminCommandTypeSetResources, map[string]any{"playerId": trader.PlayerID(), "resources": map[string]any{"energy": 10}})
	trader.AwaitState(t, "trader funded", func(state dto.GameDto) bool { return state.CurrentPlayer.Resources.Energy == 10 })
	beforeTrade := trader.State(t)
	beforeBonus := len(owner.State(t).CurrentPlayer.Cards)
	pluto := beforeTrade.Colonies[slices.IndexFunc(beforeTrade.Colonies, func(colony dto.ColonyDto) bool { return colony.ID == "pluto" })]
	expectedDraw := pluto.Steps[pluto.MarkerPosition].Outputs[0].Amount
	trader.Send(t, dto.MessageTypeActionColonyTrade, map[string]any{"colonyId": "pluto", "paymentType": "energy", "trackSteps": 0, "payment": harness.Pay(map[string]int{"energy": 3})})
	traded := trader.AwaitState(t, "trade card receipt", func(state dto.GameDto) bool { return len(state.CurrentPlayer.CardReceipts) == 1 })
	bonus := owner.AwaitState(t, "colony owner card receipt", func(state dto.GameDto) bool { return len(state.CurrentPlayer.CardReceipts) == 1 })
	tradeReceipt, bonusReceipt := traded.CurrentPlayer.CardReceipts[0], bonus.CurrentPlayer.CardReceipts[0]
	if tradeReceipt.Source != "Trade: Pluto" || len(tradeReceipt.Cards) != expectedDraw || len(traded.CurrentPlayer.Cards) != len(beforeTrade.CurrentPlayer.Cards)+expectedDraw {
		t.Fatalf("trade receipt does not match cards granted: %+v", tradeReceipt)
	}
	if bonusReceipt.Source != "Colony bonus: Pluto" || len(bonusReceipt.Cards) != 1 || len(bonus.CurrentPlayer.Cards) != beforeBonus+1 {
		t.Fatalf("colony bonus receipt does not match cards granted: %+v", bonusReceipt)
	}
	for _, card := range tradeReceipt.Cards {
		if card.ID == bonusReceipt.Cards[0].ID {
			t.Fatal("private card grants leaked between players")
		}
	}
	trader.Send(t, dto.MessageTypeActionAcknowledgeCardReceipt, map[string]any{"receiptId": tradeReceipt.ID})
	acknowledged := trader.AwaitState(t, "trade receipt dismissed", func(state dto.GameDto) bool { return len(state.CurrentPlayer.CardReceipts) == 0 })
	if len(acknowledged.CurrentPlayer.Cards) != len(traded.CurrentPlayer.Cards) || acknowledged.CurrentPlayer.Resources.Energy != traded.CurrentPlayer.Resources.Energy {
		t.Fatal("receipt acknowledgement changed the completed trade")
	}
	owner.Sync(t)
	if len(owner.State(t).CurrentPlayer.CardReceipts) != 1 {
		t.Fatal("trader acknowledgement dismissed the owner's private receipt")
	}
}
