package behavior_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"openmars/internal/action"
	"openmars/internal/delivery/dto"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestCardReveal_RoundTripCloneAndValidation(t *testing.T) {
	raw := `{"outputs":[{"type":"card-reveal","amount":2,"target":"self-player","destination":"discard","onMatch":{"selectors":[{"tags":["science"]}],"outputs":[{"type":"credit","amount":2,"target":"self-player"}]}}]}`
	var original shared.CardBehavior
	testutil.AssertNoError(t, json.Unmarshal([]byte(raw), &original), "decode reveal")
	encoded, err := json.Marshal(original)
	testutil.AssertNoError(t, err, "encode reveal")
	var decoded shared.CardBehavior
	testutil.AssertNoError(t, json.Unmarshal(encoded, &decoded), "decode round trip")
	if !reflect.DeepEqual(original, decoded) {
		t.Fatal("reveal did not round trip")
	}
	clone := original.DeepCopy()
	cloned := clone.Outputs[0].(*shared.CardRevealCondition)
	cloned.OnMatch.Selectors[0].Tags[0] = shared.TagEarth
	cloned.OnMatch.Outputs[0].SetAmount(99)
	source := original.Outputs[0].(*shared.CardRevealCondition)
	testutil.AssertEqual(t, shared.TagScience, source.OnMatch.Selectors[0].Tags[0], "clone selectors")
	testutil.AssertEqual(t, 2, source.OnMatch.Outputs[0].GetAmount(), "clone rewards")
	converted := dto.ToCardDto(gamecards.Card{Behaviors: []shared.CardBehavior{original}})
	data, err := json.Marshal(converted.Behaviors[0])
	testutil.AssertNoError(t, err, "serialize DTO")
	var dtoBehavior shared.CardBehavior
	testutil.AssertNoError(t, json.Unmarshal(data, &dtoBehavior), "DTO contains reveal and match outputs")
	if !reflect.DeepEqual(original.Outputs, dtoBehavior.Outputs) {
		t.Fatal("DTO dropped reveal fields")
	}
	testutil.AssertEqual(t, 0, len(shared.ValidateResourceCondition(source, false)), "valid reveal")
	for _, mutate := range []func(*shared.CardRevealCondition){
		func(c *shared.CardRevealCondition) { c.Destination = "hand" },
		func(c *shared.CardRevealCondition) { c.Amount = 0 },
		func(c *shared.CardRevealCondition) { c.OnMatch.Selectors = nil },
		func(c *shared.CardRevealCondition) {
			c.OnMatch.Outputs = []shared.BehaviorCondition{shared.NewCardOperationCondition(shared.ResourceCardPeek, 1, "self-player")}
		},
		func(c *shared.CardRevealCondition) {
			c.OnMatch.Outputs = []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "any-card")}
		},
	} {
		c := shared.CloneCondition(source).(*shared.CardRevealCondition)
		mutate(c)
		if len(shared.ValidateResourceCondition(c, false)) == 0 {
			t.Fatal("accepted invalid reveal")
		}
	}
	testutil.AssertErrorContains(t, json.Unmarshal([]byte(`{"outputs":[{"type":"credit","amount":1,"target":"self-player","destination":"discard"}]}`), &decoded), "only valid on card-reveal", "reject reveal fields on other types")
	testutil.AssertErrorContains(t, json.Unmarshal([]byte(`{"outputs":[{"type":"card-reveal","amount":1,"target":"self-player","destination":"discard","optional":true}]}`), &decoded), "field optional is not valid on card-reveal", "reject unsupported reveal fields")
}

func TestCardReveal_GenericBatchAndMatchingOnce(t *testing.T) {
	g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	// Research has two science tags and matches both selector alternatives, but pays once.
	ids := []string{testutil.CardID("Research"), testutil.CardID("Mine"), testutil.CardID("Invention Contest")}
	g.InitDeck(ids, nil, nil)
	expectedOrder := g.Deck().ProjectCards()
	reveal := &shared.CardRevealCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCardReveal, Amount: 3, Target: "self-player"}, Destination: "discard", OnMatch: &shared.RevealMatch{
		Selectors: []shared.Selector{{Tags: []shared.CardTag{shared.TagScience}}, {CardTypes: []string{"event"}}},
		Outputs:   []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"), shared.NewCardStorageCondition(shared.ResourceScience, 1, "self-card")},
	}}
	before, hand := p.Resources().Get().Credits, p.Hand().CardCount()
	source := testutil.CardID("Search For Life")
	applier := gamecards.NewBehaviorApplier(p, g, "Generic reveal", testutil.TestLogger()).WithSourceCardID(source).WithCardRegistry(registry)
	outputs, err := applier.ApplyOutputsAndGetCalculated(context.Background(), []shared.BehaviorCondition{reveal})
	testutil.AssertNoError(t, err, "reveal batch")
	testutil.AssertEqual(t, before+4, p.Resources().Get().Credits, "once per matching card")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(source), "retain source card context")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(testutil.CardID("Research")), "do not store reward on revealed card")
	testutil.AssertEqual(t, hand, p.Hand().CardCount(), "do not draw revealed cards to hand")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "temporary revealed cards are not acquisitions")
	if !reflect.DeepEqual(expectedOrder, g.Deck().DiscardPile()) {
		t.Fatal("reveal discarded out of deck order")
	}
	pending := p.Selection().GetPendingCardReveal()
	for i, entry := range pending.Cards {
		testutil.AssertEqual(t, expectedOrder[i], entry.CardID, "public reveal order")
	}
	testutil.AssertEqual(t, 4, len(outputs), "report actual rewards, not conditional promises")
}

func TestCardReveal_RecyclesDiscardAndDoesNotPreviewReward(t *testing.T) {
	g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Search For Life")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
	g.InitDeck(nil, nil, nil)
	testutil.AssertNoError(t, g.Deck().Discard(context.Background(), []string{testutil.CardID("Tardigrades")}), "seed discard pile")
	state := action.CalculatePlayerCardActionState(card.ID, card.Behaviors[0], 0, p, g, registry)
	testutil.AssertTrue(t, state.Available(), "discard pile can supply reveal")
	testutil.AssertEqual(t, 0, len(state.ComputedValues), "never preview a hidden reward")
	applier := gamecards.NewBehaviorApplier(p, g, card.Name, testutil.TestLogger()).WithSourceCardID(card.ID).WithCardRegistry(registry)
	testutil.AssertNoError(t, applier.ApplyOutputs(context.Background(), card.Behaviors[0].Outputs), "recycle and reveal")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "reward recycled matching card")
	testutil.AssertEqual(t, 1, len(g.Deck().DiscardPile()), "revealed card ends back in discard")
	testutil.AssertEqual(t, 1, g.Deck().ShuffleCount(), "normal reshuffle")
}
