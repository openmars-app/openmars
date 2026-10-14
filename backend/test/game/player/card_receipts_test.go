package player_test

import (
	"slices"
	"testing"

	"openmars/internal/events"
	"openmars/test/testutil"
)

func TestReceiveCardsCommitsPrivateReceiptBeforeHandEvents(t *testing.T) {
	g, _ := testutil.CreateTestGameWithPlayers(t, 2)
	p, other := g.GetAllPlayers()[0], g.GetAllPlayers()[1]
	addedEvents, handEvents := 0, 0
	events.Subscribe(g.EventBus(), func(event events.CardAddedToHandEvent) {
		if event.PlayerID != p.ID() {
			return
		}
		addedEvents++
		receipts := p.Selection().CardReceipts()
		if len(receipts) != 1 || !slices.Equal(receipts[0].Cards, []string{"001", "002"}) || p.Hand().CardCount() != 2 {
			t.Errorf("grant and receipt must already be complete when events fire: %+v", receipts)
		}
	})
	events.Subscribe(g.EventBus(), func(event events.CardHandUpdatedEvent) { handEvents++ })
	cards := []string{"001", "002", "001", ""}
	p.Hand().ReceiveCards(cards, "Olympus Conference", "185")
	cards[0] = "changed"
	receipts := p.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "One receipt for the grant")
	testutil.AssertEqual(t, "Olympus Conference", receipts[0].Source, "Source name")
	testutil.AssertEqual(t, "185", receipts[0].SourceCardID, "Source card")
	testutil.AssertTrue(t, receipts[0].ID != "", "Durable receipt identity")
	testutil.AssertEqual(t, 2, addedEvents, "One event per new card")
	testutil.AssertEqual(t, 1, handEvents, "One complete hand update")
	testutil.AssertEqual(t, 0, len(other.Selection().CardReceipts()), "Receipt is private")

	p.Hand().ReceiveCards([]string{"001", "002"}, "Olympus Conference", "185")
	p.Hand().ReceiveCards(nil, "empty", "")
	testutil.AssertEqual(t, 1, len(p.Selection().CardReceipts()), "Repeated and empty grants create no receipt")
	testutil.AssertEqual(t, 2, addedEvents, "Repeated grant creates no event")
	p.Selection().AcknowledgeCardReceipt(receipts[0].ID)
	p.Selection().AcknowledgeCardReceipt(receipts[0].ID)
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "Acknowledgement is idempotent")
	testutil.AssertEqual(t, 2, p.Hand().CardCount(), "Acknowledgement never grants or removes cards")
}

func TestReviewedCardAcquisitionDoesNotCreateReceipt(t *testing.T) {
	g, _ := testutil.CreateTestGameWithPlayers(t, 1)
	p := g.GetAllPlayers()[0]
	p.Hand().AddCard("001")
	testutil.AssertEqual(t, 1, p.Hand().CardCount(), "Reviewed card is owned")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "Already reviewed card needs no duplicate popup")
}
