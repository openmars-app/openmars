package e2e_test

import (
	"slices"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/test/e2e/harness"
	"openmars/test/testutil"
)

func TestOlympusConferenceDrawBroadcastsReceiptExactlyOnce(t *testing.T) {
	t.Parallel()
	srv := harness.Start(t)
	game := srv.Play(t, func(setup *dto.GameSetupDto) {
		setup.GameModeID = "classic"
		setup.Features = nil
		setup.DevelopmentMode = true
	}, "alice", "bob")
	owner := game.Ready(t)
	other := game.Others(owner)[0]
	owner.Admin(t, dto.AdminCommandTypeSetResources, map[string]any{"playerId": owner.PlayerID(), "resources": map[string]any{"credits": 100}})
	play := func(name string) {
		id := testutil.CardID(name)
		owner.Admin(t, dto.AdminCommandTypeGiveCard, map[string]any{"playerId": owner.PlayerID(), "cardId": id})
		cards := owner.State(t).CurrentPlayer.Cards
		index := slices.IndexFunc(cards, func(card dto.PlayerCardDto) bool { return card.ID == id })
		if index < 0 {
			t.Fatalf("%s was not granted", name)
		}
		owner.Send(t, dto.MessageTypeActionPlayCard, map[string]any{"cardId": id, "payment": harness.Pay(map[string]int{"credit": cards[index].EffectiveCost})})
		owner.AwaitState(t, name+" played", func(state dto.GameDto) bool { return !slices.Contains(handIDs(state), id) })
	}
	olympusID := testutil.CardID("Olympus Conference")
	play("Olympus Conference")
	for _, resolution := range owner.State(t).CurrentPlayer.PendingBehaviorResolutions {
		if resolution.SourceCardID == olympusID {
			owner.Send(t, dto.MessageTypeActionBehaviorChoiceConfirmed, map[string]any{"resolutionId": resolution.ID, "choiceIndex": 0})
			owner.AwaitState(t, "initial science stored", func(state dto.GameDto) bool { return len(state.CurrentPlayer.PendingBehaviorResolutions) == 0 })
		}
	}
	owner.Admin(t, dto.AdminCommandTypeSetCurrentTurn, map[string]any{"playerId": owner.PlayerID()})
	play("Search For Life")
	state := owner.AwaitState(t, "Olympus decision", func(state dto.GameDto) bool {
		return slices.ContainsFunc(state.CurrentPlayer.PendingBehaviorResolutions, func(resolution dto.PendingBehaviorResolutionDto) bool { return resolution.SourceCardID == olympusID })
	})
	index := slices.IndexFunc(state.CurrentPlayer.PendingBehaviorResolutions, func(resolution dto.PendingBehaviorResolutionDto) bool { return resolution.SourceCardID == olympusID })
	resolution := state.CurrentPlayer.PendingBehaviorResolutions[index]
	before := len(state.CurrentPlayer.Cards)
	payload := map[string]any{"resolutionId": resolution.ID, "choiceIndex": 1}
	owner.Send(t, dto.MessageTypeActionBehaviorChoiceConfirmed, payload)
	drawn := owner.AwaitState(t, "Olympus receipt", func(state dto.GameDto) bool { return len(state.CurrentPlayer.CardReceipts) == 1 })
	receipt := drawn.CurrentPlayer.CardReceipts[0]
	if receipt.Source != "Olympus Conference" || receipt.SourceCardID != olympusID || len(receipt.Cards) != 1 || len(drawn.CurrentPlayer.Cards) != before+1 {
		t.Fatalf("Olympus draw missing its exact card receipt: %+v", receipt)
	}
	other.Sync(t)
	if len(other.State(t).CurrentPlayer.CardReceipts) != 0 {
		t.Fatal("draw receipt leaked to opponent")
	}
	owner.Send(t, dto.MessageTypeActionBehaviorChoiceConfirmed, payload)
	owner.AwaitError(t)
	if len(owner.State(t).CurrentPlayer.Cards) != before+1 || len(owner.State(t).CurrentPlayer.CardReceipts) != 1 {
		t.Fatal("replayed decision duplicated the grant or receipt")
	}
	for range 2 {
		owner.Send(t, dto.MessageTypeActionAcknowledgeCardReceipt, map[string]any{"receiptId": receipt.ID})
		owner.Sync(t)
	}
	if len(owner.State(t).CurrentPlayer.CardReceipts) != 0 || len(owner.State(t).CurrentPlayer.Cards) != before+1 {
		t.Fatal("acknowledgement changed card ownership or failed to dismiss")
	}
}
