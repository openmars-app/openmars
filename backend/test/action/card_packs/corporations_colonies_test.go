package card_packs_test

import (
	"openmars/internal/delivery/dto"

	"context"
	"testing"

	"openmars/internal/action"
	"openmars/internal/action/admin"
	cardAction "openmars/internal/action/card"
	"openmars/internal/action/confirmation"
	resconvaction "openmars/internal/action/resource_conversion"
	tileAction "openmars/internal/action/tile"
	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/events"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// =============================================================================
// Aridor (CC1, corporation, colonies)
// "Effect: Increase your M€ production 1 step when you get a new type of tag
//  in play (event cards do not count). You start with 40 M€."
// =============================================================================

func newAridorEffect() shared.CardEffect {
	return shared.CardEffect{
		CardID:        "CC1",
		CardName:      "Aridor",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:   "tag-played",
						Unique: true,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewProductionCondition(shared.ResourceCreditProduction, 1, "self-player"),
			},
		},
	}
}

func TestAridor_NewTagTriggersProductionIncrease(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	scienceCard := gamecards.Card{
		ID:   "test-science",
		Name: "Test Science Card",
		Type: gamecards.CardTypeAutomated,
		Tags: []shared.CardTag{shared.TagScience},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{scienceCard})

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID("CC1")
	testutil.StartTestGame(t, testGame)

	wild := testutil.GetCardByName("Research Coordination")
	p.PlayedCards().AddCard(wild.ID, wild.Name, string(wild.Type), nil)

	effect := newAridorEffect()
	p.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, p, effect, logger, cardRegistry)

	productionBefore := p.Resources().Production().Credits

	// Play a card with a new tag type
	p.PlayedCards().AddCard(scienceCard.ID, scienceCard.Name, string(scienceCard.Type), []string{string(shared.TagScience)})

	testutil.AssertEqual(t, productionBefore+1, p.Resources().Production().Credits,
		"Aridor should gain 1 credit production when a new tag type is played")
}

func TestAridor_DuplicateTagDoesNotTrigger(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	scienceCard1 := gamecards.Card{
		ID:   "test-science-1",
		Name: "Test Science Card 1",
		Type: gamecards.CardTypeAutomated,
		Tags: []shared.CardTag{shared.TagScience},
	}
	scienceCard2 := gamecards.Card{
		ID:   "test-science-2",
		Name: "Test Science Card 2",
		Type: gamecards.CardTypeAutomated,
		Tags: []shared.CardTag{shared.TagScience},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{scienceCard1, scienceCard2})

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID("CC1")
	testutil.StartTestGame(t, testGame)

	// Play first science card (no effect yet, just establishing the tag)
	p.PlayedCards().AddCard(scienceCard1.ID, scienceCard1.Name, string(scienceCard1.Type), []string{string(shared.TagScience)})

	effect := newAridorEffect()
	p.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, p, effect, logger, cardRegistry)

	productionBefore := p.Resources().Production().Credits

	// Play second science card — tag is not new
	p.PlayedCards().AddCard(scienceCard2.ID, scienceCard2.Name, string(scienceCard2.Type), []string{string(shared.TagScience)})

	testutil.AssertEqual(t, productionBefore, p.Resources().Production().Credits,
		"Aridor should NOT gain production when duplicate tag type is played")
}

func TestAridor_MultipleNewTagsOnOneCard(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	multiTagCard := gamecards.Card{
		ID:   "test-multi-tag",
		Name: "Test Multi Tag Card",
		Type: gamecards.CardTypeAutomated,
		Tags: []shared.CardTag{shared.TagScience, shared.TagSpace},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{multiTagCard})

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID("CC1")
	testutil.StartTestGame(t, testGame)

	effect := newAridorEffect()
	p.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, p, effect, logger, cardRegistry)

	productionBefore := p.Resources().Production().Credits

	// Play a card with two new tag types
	p.PlayedCards().AddCard(multiTagCard.ID, multiTagCard.Name, string(multiTagCard.Type),
		[]string{string(shared.TagScience), string(shared.TagSpace)})

	testutil.AssertEqual(t, productionBefore+2, p.Resources().Production().Credits,
		"Aridor should gain 2 credit production for 2 new tag types on one card")
}

func TestAridor_EventCardDoesNotTrigger(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	eventCard := gamecards.Card{
		ID:   "test-event",
		Name: "Test Event Card",
		Type: gamecards.CardTypeEvent,
		Tags: []shared.CardTag{shared.TagScience},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{eventCard})

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID("CC1")
	testutil.StartTestGame(t, testGame)

	effect := newAridorEffect()
	p.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, p, effect, logger, cardRegistry)

	productionBefore := p.Resources().Production().Credits

	// Play an event card with a new tag — should NOT trigger
	p.PlayedCards().AddCard(eventCard.ID, eventCard.Name, string(eventCard.Type), []string{string(shared.TagScience)})

	testutil.AssertEqual(t, productionBefore, p.Resources().Production().Credits,
		"Aridor should NOT gain production from event card tags")
}

func TestAridor_WildTagDoesNotTrigger(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	wildCard := gamecards.Card{
		ID:   "test-wild",
		Name: "Test Wild Card",
		Type: gamecards.CardTypeAutomated,
		Tags: []shared.CardTag{shared.TagWild},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{wildCard})

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID("CC1")
	testutil.StartTestGame(t, testGame)

	effect := newAridorEffect()
	p.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, p, effect, logger, cardRegistry)

	productionBefore := p.Resources().Production().Credits

	// Play a card with a wild tag — should NOT trigger
	p.PlayedCards().AddCard(wildCard.ID, wildCard.Name, string(wildCard.Type), []string{string(shared.TagWild)})

	testutil.AssertEqual(t, productionBefore, p.Resources().Production().Credits,
		"Aridor should NOT gain production from wild tags")
}

// =============================================================================
// Arklight (CC2, corporation, colonies)
// "Effect: Add 1 animal to this card when you play an animal or plant tag,
//  including this. You start with 45 M€. Increase your M€ production 2 steps.
//  1 VP per 2 animals on this card."
// =============================================================================

func newArklightEffect() shared.CardEffect {
	card := testutil.GetCardByName("Arklight")
	return shared.CardEffect{CardID: card.ID, CardName: card.Name, BehaviorIndex: 1, Behavior: card.Behaviors[1]}
}

func TestArklight_PassiveEffectByTag(t *testing.T) {
	tests := []struct {
		name         string
		tag          shared.CardTag
		expectedGain int
		description  string
	}{
		{"animal tag triggers animal gain", shared.TagAnimal, 1, "Arklight should gain 1 animal when an animal tag is played"},
		{"plant tag triggers animal gain", shared.TagPlant, 1, "Arklight should gain 1 animal when a plant tag is played"},
		{"unrelated tag does not trigger", shared.TagScience, 0, "Arklight should NOT gain an animal when an unrelated tag is played"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			logger := testutil.TestLogger()
			ctx := context.Background()

			testCard := gamecards.Card{
				ID:   "test-" + string(tt.tag),
				Name: "Test " + string(tt.tag) + " Card",
				Type: gamecards.CardTypeAutomated,
				Tags: []shared.CardTag{tt.tag},
			}
			cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{testCard})

			p := testGame.GetAllPlayers()[0]
			p.SetCorporationID("CC2")
			p.Resources().AddToStorage("CC2", 0)
			testutil.StartTestGame(t, testGame)

			effect := newArklightEffect()
			p.Effects().AddEffect(effect)
			action.SubscribePassiveEffectToEvents(ctx, testGame, p, effect, logger, cardRegistry)

			storageBefore := p.Resources().GetCardStorage("CC2")

			events.Publish(testGame.EventBus(), events.TagPlayedEvent{
				GameID:   testGame.ID(),
				PlayerID: p.ID(),
				CardID:   testCard.ID,
				CardName: testCard.Name,
				Tag:      string(tt.tag),
			})

			testutil.AssertEqual(t, storageBefore+tt.expectedGain, p.Resources().GetCardStorage("CC2"), tt.description)
		})
	}
}

func TestArklight_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Arklight"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	production := p.Resources().Production()

	testutil.AssertEqual(t, 45, resources.Credits, "Arklight should start with 45 credits")
	testutil.AssertEqual(t, 2, production.Credits, "Arklight should start with +2 credit production")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(testutil.CardID("Arklight")),
		"Arklight should start with 1 animal (from 'including this')")
}

// =============================================================================
// Polyphemos (CC3, corporation, colonies)
// "Effect: Pay 5 M€ instead of 3 M€ when buying cards, including the starting hand.
//  You start with 50 M€. Increase your M€ production 5 steps. Gain 5 titanium."
// =============================================================================

func TestPolyphemos_NegativeDiscountOnCardBuying(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	p := players[0]

	polyphemosCard, err := cardRegistry.GetByID(testutil.CardID("Polyphemos"))
	testutil.AssertNoError(t, err, "Polyphemos card should exist")

	for behaviorIndex, behavior := range polyphemosCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        polyphemosCard.ID,
				CardName:      polyphemosCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	effects := p.Effects().List()
	testutil.AssertEqual(t, 1, len(effects), "Should have 1 discount effect registered")

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	discounts := calculator.CalculateActionDiscounts(p, shared.ActionCardBuying)
	testutil.AssertEqual(t, -2, discounts[shared.ResourceCredit], "Polyphemos should have -2 card-buying discount (cost increase)")
}

func TestPolyphemos_DoesNotAffectCardPlayCost(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	p := players[0]

	polyphemosCard, err := cardRegistry.GetByID(testutil.CardID("Polyphemos"))
	testutil.AssertNoError(t, err, "Polyphemos card should exist")

	for behaviorIndex, behavior := range polyphemosCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        polyphemosCard.ID,
				CardName:      polyphemosCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	// Polyphemos's -2 card-buying discount should NOT affect the play cost of cards
	anyCard, err := cardRegistry.GetByID(testutil.CardID("Arctic Algae"))
	testutil.AssertNoError(t, err, "Arctic Algae card should exist")

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	playDiscount := calculator.CalculateCardDiscounts(p, anyCard)
	testutil.AssertEqual(t, 0, playDiscount, "Polyphemos should not affect card play discount")
}

func TestPolyphemos_CardBuyCostFromCorpBehaviors(t *testing.T) {
	cardRegistry := testutil.CreateTestCardRegistry()
	polyphemosCard, err := cardRegistry.GetByID(testutil.CardID("Polyphemos"))
	testutil.AssertNoError(t, err, "Polyphemos card should exist")

	discount := gamecards.CalculateActionDiscountsFromCard(polyphemosCard, shared.ActionCardBuying)
	testutil.AssertEqual(t, -2, discount, "Polyphemos should have -2 card-buying discount from behaviors")

	effectiveCost := max(3-discount, 0)
	testutil.AssertEqual(t, 5, effectiveCost, "Polyphemos effective card buy cost should be 5 MC")
}

func TestPolyphemos_ProductionPhaseCardBuyCost(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)

	// Register Polyphemos discount effect
	polyphemosCard, err := cardRegistry.GetByID(testutil.CardID("Polyphemos"))
	testutil.AssertNoError(t, err, "Polyphemos card should exist")
	p.SetCorporationID(polyphemosCard.ID)

	for behaviorIndex, behavior := range polyphemosCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        polyphemosCard.ID,
				CardName:      polyphemosCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	// Set up production phase with available cards
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw), "update phase")
	drawnCards, err := testGame.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "draw project cards")

	testutil.AssertNoError(t, testGame.SetProductionPhase(ctx, playerID, &shared.ProductionPhase{
		AvailableCards:    drawnCards,
		SelectionComplete: false,
	}), "set production phase")

	// Give player exactly 10 credits (enough for 2 cards at 5 MC each, but not 3)
	testutil.SetPlayerCredits(ctx, p, 10)

	// Buying 2 cards should succeed (2 * 5 = 10 MC)
	action := confirmation.NewConfirmProductionCardsAction(repo, cardRegistry, nil, logger)
	err = action.Execute(ctx, testGame.ID(), playerID, drawnCards[:2], false, shared.NativePayment(shared.ResourceCredit, len(drawnCards[:2])*5))
	testutil.AssertNoError(t, err, "buying 2 cards at 5 MC each should succeed with 10 credits")

	resources := p.Resources().Get()
	testutil.AssertEqual(t, 0, resources.Credits, "should have 0 credits after buying 2 cards at 5 MC")
}

func TestPolyphemos_ProductionPhaseCardBuyCost_InsufficientCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)

	// Register Polyphemos discount effect
	polyphemosCard, err := cardRegistry.GetByID(testutil.CardID("Polyphemos"))
	testutil.AssertNoError(t, err, "Polyphemos card should exist")
	p.SetCorporationID(polyphemosCard.ID)

	for behaviorIndex, behavior := range polyphemosCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        polyphemosCard.ID,
				CardName:      polyphemosCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	// Set up production phase
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseProductionAndCardDraw), "update phase")
	drawnCards, err := testGame.Deck().DrawProjectCards(ctx, 4)
	testutil.AssertNoError(t, err, "draw project cards")

	testutil.AssertNoError(t, testGame.SetProductionPhase(ctx, playerID, &shared.ProductionPhase{
		AvailableCards:    drawnCards,
		SelectionComplete: false,
	}), "set production phase")

	// Give player 9 credits (not enough for 2 cards at 5 MC each)
	testutil.SetPlayerCredits(ctx, p, 9)

	// Buying 2 cards should fail (2 * 5 = 10 MC, only have 9)
	action := confirmation.NewConfirmProductionCardsAction(repo, cardRegistry, nil, logger)
	err = action.Execute(ctx, testGame.ID(), playerID, drawnCards[:2], false, shared.NativePayment(shared.ResourceCredit, len(drawnCards[:2])*5))
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "buying 2 cards at 5 MC each should fail with only 9 credits")
	testutil.AssertEqual(t, 9, p.Resources().Get().Credits, "credits unchanged")
}

// =============================================================================
// Rim Freighters (C35, active, colonies)
// "Effect: Pay 1 less resource when trading."
// =============================================================================

func TestRimFreighters_ColonyTradeDiscount(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	p := players[0]

	rimFreightersCard, err := cardRegistry.GetByID(testutil.CardID("Rim Freighters"))
	testutil.AssertNoError(t, err, "Rim Freighters card should exist")

	for behaviorIndex, behavior := range rimFreightersCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        rimFreightersCard.ID,
				CardName:      rimFreightersCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	discounts := calculator.CalculateActionDiscounts(p, shared.ActionColonyTrade)

	testutil.AssertEqual(t, 1, discounts[shared.ResourceCredit], "Rim Freighters should give 1 credit trade discount")
	testutil.AssertEqual(t, 1, discounts[shared.ResourceEnergy], "Rim Freighters should give 1 energy trade discount")
	testutil.AssertEqual(t, 1, discounts[shared.ResourceTitanium], "Rim Freighters should give 1 titanium trade discount")
}

func TestRimFreighters_DoesNotAffectCardPlayCost(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	p := players[0]

	rimFreightersCard, err := cardRegistry.GetByID(testutil.CardID("Rim Freighters"))
	testutil.AssertNoError(t, err, "Rim Freighters card should exist")

	for behaviorIndex, behavior := range rimFreightersCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        rimFreightersCard.ID,
				CardName:      rimFreightersCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	anyCard, err := cardRegistry.GetByID(testutil.CardID("Arctic Algae"))
	testutil.AssertNoError(t, err, "Arctic Algae card should exist")

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	playDiscount := calculator.CalculateCardDiscounts(p, anyCard)
	testutil.AssertEqual(t, 0, playDiscount, "Rim Freighters should not affect card play discount")
}

func TestRimFreighters_DoesNotAffectCardBuyingCost(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	p := players[0]

	rimFreightersCard, err := cardRegistry.GetByID(testutil.CardID("Rim Freighters"))
	testutil.AssertNoError(t, err, "Rim Freighters card should exist")

	for behaviorIndex, behavior := range rimFreightersCard.Behaviors {
		if gamecards.HasAutoTrigger(behavior) && gamecards.HasPersistentEffects(behavior) {
			p.Effects().AddEffect(shared.CardEffect{
				CardID:        rimFreightersCard.ID,
				CardName:      rimFreightersCard.Name,
				BehaviorIndex: behaviorIndex,
				Behavior:      behavior,
			})
		}
	}

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	discounts := calculator.CalculateActionDiscounts(p, shared.ActionCardBuying)
	testutil.AssertEqual(t, 0, discounts[shared.ResourceCredit], "Rim Freighters should not affect card-buying cost")
}

func TestPartialResourceTradeDiscount_OnlyAffectsSpecifiedResources(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	p := players[0]

	// Register a fake effect that discounts only titanium and credit for colony-trade (not energy)
	p.Effects().AddEffect(shared.CardEffect{
		CardID:        "fake-partial-trade-discount",
		CardName:      "Fake Partial Trade Discount",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
			Outputs: []shared.BehaviorCondition{
				&shared.EffectCondition{
					ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceDiscount, Amount: 1, Target: "self-player"},
					Selectors: []shared.Selector{
						{
							Actions:   []string{shared.ActionColonyTrade},
							Resources: []string{"titanium", "credit"},
						},
					},
				},
			},
		},
	})

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	discounts := calculator.CalculateActionDiscounts(p, shared.ActionColonyTrade)

	testutil.AssertEqual(t, 1, discounts[shared.ResourceTitanium], "Should discount titanium trade cost")
	testutil.AssertEqual(t, 1, discounts[shared.ResourceCredit], "Should discount credit trade cost")
	testutil.AssertEqual(t, 0, discounts[shared.ResourceEnergy], "Should NOT discount energy trade cost")
}

// =============================================================================
// Poseidon (CC4, corporation, colonies)
// "Effect: Raise your M€ production 1 step when any colony is placed, including
//  this. You start with 45 M€. As your first action, place a colony."
// =============================================================================

func TestPoseidon_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	addColony(testGame, "luna", 1, nil)
	addColony(testGame, "io", 1, nil)
	addColony(testGame, "ganymede", 1, nil)

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Poseidon"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 45, resources.Credits, "Poseidon should start with 45 credits")
}

func TestPoseidon_ForcedFirstActionSetup(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	addColony(testGame, "luna", 1, nil)
	addColony(testGame, "io", 1, nil)
	addColony(testGame, "ganymede", 1, nil)

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Poseidon"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	forcedAction := testGame.GetForcedFirstAction(playerID)
	testutil.AssertTrue(t, forcedAction != nil, "Poseidon should create a forced first action")
	testutil.AssertEqual(t, "resolving", forcedAction.State, "First action should be resolving")
	testutil.AssertEqual(t, testutil.CardID("Poseidon"), forcedAction.CorporationID, "Forced first action should reference Poseidon")

	p, _ := testGame.GetPlayer(playerID)
	colonySelection := p.Selection().GetPendingColonySelection()
	testutil.AssertTrue(t, colonySelection != nil, "Poseidon should create pending colony selection")
	testutil.AssertTrue(t, len(colonySelection.AvailableColonyIDs) > 0, "Should have available colonies to choose from")
}

func TestPoseidon_GainProductionOnColonyPlacement(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	addColony(testGame, "luna", 1, nil)
	addColony(testGame, "io", 1, nil)
	addColony(testGame, "ganymede", 1, nil)

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Poseidon"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	creditProductionBefore := p.Resources().Production().Credits

	events.Publish(testGame.EventBus(), events.ColonyBuiltEvent{
		GameID:   testGame.ID(),
		PlayerID: playerID,
		ColonyID: "luna",
	})

	testutil.AssertEqual(t, creditProductionBefore+1, p.Resources().Production().Credits,
		"Poseidon should gain 1 credit production on colony placement")
}

// =============================================================================
// Stormcraft Incorporated (CC5, corporation, colonies)
// "Action: Add 1 floater to **any** card. / Effect: Floaters on this card may
//  be used as 2 heat each. You start with 48 M€."
// =============================================================================

func TestStormcraft_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Stormcraft Incorporated"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()

	testutil.AssertEqual(t, 48, resources.Credits, "Stormcraft should start with 48 credits")
}

func TestStormcraft_StoragePaymentSubstituteRegistered(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Stormcraft Incorporated"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	subs := p.Resources().PaymentSubstitutes()

	testutil.AssertEqual(t, 1, len(subs), "Should have 1 storage payment substitute")
	testutil.AssertEqual(t, string(shared.ResourceHeat), string(subs[0].TargetResource),
		"Storage substitute should target heat")
	testutil.AssertEqual(t, 2, subs[0].ConversionRate,
		"Conversion rate should be 2 heat per floater")
	testutil.AssertEqual(t, string(shared.ResourceFloater), string(subs[0].Source.Resource),
		"Storage resource type should be floater")
}

func TestStormcraft_ConvertHeatWithFloatersOnly(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Stormcraft Incorporated"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	stormcraftID := testutil.CardID("Stormcraft Incorporated")

	// Give player 4 floaters on Stormcraft (= 8 heat), 0 actual heat
	p.Resources().AddToStorage(stormcraftID, 4)
	testutil.SetPlayerHeat(ctx, p, 0)

	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID, 2), "set current turn")

	convertAction := resconvaction.NewConvertHeatToTemperatureAction(repo, cardRegistry, nil, logger)
	initialTemp := testGame.GlobalParameters().Temperature()

	err = convertAction.Execute(ctx, testGame.ID(), playerID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-card", Resource: shared.ResourceFloater, CardID: stormcraftID}, TargetResource: shared.ResourceHeat, Amount: 4}}})
	testutil.AssertNoError(t, err, "Should succeed with 4 floaters (= 8 heat)")

	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(stormcraftID),
		"All 4 floaters should be spent")
	testutil.AssertEqual(t, initialTemp+2, testGame.GlobalParameters().Temperature(),
		"Temperature should increase by 2")
}

func TestStormcraft_ConvertHeatWithMixedPayment(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Stormcraft Incorporated"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	stormcraftID := testutil.CardID("Stormcraft Incorporated")

	// 2 floaters (= 4 heat) + 4 actual heat = 8 total
	p.Resources().AddToStorage(stormcraftID, 2)
	testutil.SetPlayerHeat(ctx, p, 4)

	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID, 2), "set current turn")

	convertAction := resconvaction.NewConvertHeatToTemperatureAction(repo, cardRegistry, nil, logger)

	err = convertAction.Execute(ctx, testGame.ID(), playerID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-card", Resource: shared.ResourceFloater, CardID: stormcraftID}, TargetResource: shared.ResourceHeat, Amount: 2}, {Source: shared.PaymentSource{Target: "self-player", Resource: shared.ResourceHeat}, TargetResource: shared.ResourceHeat, Amount: 4}}})
	testutil.AssertNoError(t, err, "Should succeed with 2 floaters + 4 heat")

	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(stormcraftID),
		"Both floaters should be spent")
	testutil.AssertEqual(t, 0, p.Resources().Get().Heat,
		"All heat should be spent")
}

func TestStormcraft_ConvertHeatInsufficientResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Stormcraft Incorporated"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	stormcraftID := testutil.CardID("Stormcraft Incorporated")

	// 1 floater (= 2 heat) + 3 actual heat = 5 total (need 8)
	p.Resources().AddToStorage(stormcraftID, 1)
	testutil.SetPlayerHeat(ctx, p, 3)

	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID, 2), "set current turn")

	convertAction := resconvaction.NewConvertHeatToTemperatureAction(repo, cardRegistry, nil, logger)

	err = convertAction.Execute(ctx, testGame.ID(), playerID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-card", Resource: shared.ResourceFloater, CardID: stormcraftID}, TargetResource: shared.ResourceHeat, Amount: 1}}})
	testutil.AssertErrorContains(t, err, "insufficient heat payment", "Should fail with only 5 heat equivalent (need 8)")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(stormcraftID), "failed payment preserves floaters")
	testutil.AssertEqual(t, 3, p.Resources().Get().Heat, "failed payment preserves heat")
}

func TestStormcraft_StateCalculatorShowsAffordableWithFloaters(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	p.SetCorporationID(testutil.CardID("Stormcraft Incorporated"))

	stormcraftID := testutil.CardID("Stormcraft Incorporated")

	// Register the storage payment substitute manually
	p.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: shared.PaymentSource{Target: "self-card", CardID: stormcraftID, Resource: shared.ResourceFloater},
		ConversionRate: 2,
		TargetResource: shared.ResourceHeat,
	})

	// 2 floaters (= 4 heat) + 4 actual heat = 8 total
	p.Resources().AddToStorage(stormcraftID, 2)
	testutil.SetPlayerHeat(ctx, p, 4)
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID, 2), "set current turn")

	state := action.CalculatePlayerStandardProjectState(
		shared.StandardProjectConvertHeatToTemperature, p, testGame, cardRegistry,
	)

	testutil.AssertEqual(t, 0, len(state.Errors),
		"Convert heat should be affordable with 4 heat + 2 floaters (= 8 heat total)")
}

func TestStormcraft_StateCalculatorShowsUnaffordableWithoutEnoughFloaters(t *testing.T) {
	testGame, _, cardRegistry, playerID, _ := setupCorporationColoniesGame(t)
	ctx := context.Background()

	p, _ := testGame.GetPlayer(playerID)
	p.SetCorporationID(testutil.CardID("Stormcraft Incorporated"))

	stormcraftID := testutil.CardID("Stormcraft Incorporated")

	p.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: shared.PaymentSource{Target: "self-card", CardID: stormcraftID, Resource: shared.ResourceFloater},
		ConversionRate: 2,
		TargetResource: shared.ResourceHeat,
	})

	// 1 floater (= 2 heat) + 3 actual heat = 5 total (need 8)
	p.Resources().AddToStorage(stormcraftID, 1)
	testutil.SetPlayerHeat(ctx, p, 3)
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID, 2), "set current turn")

	state := action.CalculatePlayerStandardProjectState(
		shared.StandardProjectConvertHeatToTemperature, p, testGame, cardRegistry,
	)

	testutil.AssertTrue(t, len(state.Errors) > 0,
		"Convert heat should be unaffordable with only 5 heat equivalent")
}

func TestPoseidon_ForcedColonyDoesNotBlockInitAdvance_Repro568(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	ctx := context.Background()

	colonyDefs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	testutil.AssertNoError(t, err, "Failed to load colonies")
	colonyRegistry := colony.NewInMemoryColonyRegistry(colonyDefs)

	testGame.UpdateSettings(ctx, shared.GameSettings{
		MaxPlayers: 4,
		GameModeID: "classic", Features: []string{}, Systems: []string{"colonies"},
	})

	err = testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "Failed to set active status")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseStartingSelection)
	testutil.AssertNoError(t, err, "Failed to set starting selection phase")

	addColony(testGame, "luna", 1, nil)
	addColony(testGame, "io", 1, nil)
	addColony(testGame, "ganymede", 1, nil)

	players := testGame.GetAllPlayers()
	playerID1, playerID2 := players[0].ID(), players[1].ID()
	err = testGame.SetTurnOrder(ctx, []string{playerID1, playerID2})
	testutil.AssertNoError(t, err, "Failed to set turn order")

	for _, p := range players {
		testutil.SetPlayerCredits(ctx, p, 100)
	}

	err = testGame.SetSelectCorporationPhase(ctx, playerID1, &shared.SelectCorporationPhase{
		AvailableCorporations: []string{"B01"},
	})
	testutil.AssertNoError(t, err, "Failed to set corp phase for player 1")

	poseidonID := testutil.CardID("Poseidon")
	err = testGame.SetSelectCorporationPhase(ctx, playerID2, &shared.SelectCorporationPhase{
		AvailableCorporations: []string{poseidonID},
	})
	testutil.AssertNoError(t, err, "Failed to set corp phase for player 2 (Poseidon)")

	deck := testGame.Deck()
	for _, p := range players {
		projectCards, drawErr := deck.DrawProjectCards(ctx, 10)
		testutil.AssertNoError(t, drawErr, "Failed to draw project cards")
		err = testGame.SetSelectStartingCardsPhase(ctx, p.ID(), &shared.SelectStartingCardsPhase{
			AvailableCards: projectCards,
		})
		testutil.AssertNoError(t, err, "Failed to set starting cards phase")
	}

	selectAction := turnAction.NewSelectStartingChoicesAction(repo, cardRegistry, nil, logger)

	err = selectAction.Execute(ctx, testGame.ID(), playerID1, "B01", []string{}, []string{}, shared.NativePayment(shared.ResourceCredit, len([]string{})*3))
	testutil.AssertNoError(t, err, "Player 1 selection")

	err = selectAction.Execute(ctx, testGame.ID(), playerID2, poseidonID, []string{}, []string{}, shared.NativePayment(shared.ResourceCredit, len([]string{})*3))
	testutil.AssertNoError(t, err, "Player 2 (Poseidon) selection")

	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Should be in init_apply_corp")

	confirmAction := turnAction.NewConfirmInitAdvanceAction(repo, cardRegistry, nil, nil, logger)
	confirmColony := confirmation.NewConfirmColonyPlacementAction(repo, cardRegistry, colonyRegistry, logger)

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 1 corp")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Advance to player 2")

	err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
	testutil.AssertNoError(t, err, "Apply player 2 corp (Poseidon)")

	forcedAction := testGame.GetForcedFirstAction(playerID2)
	testutil.AssertTrue(t, forcedAction != nil, "Poseidon first action is queued")
	testutil.AssertEqual(t, "queued", forcedAction.State, "setup does not run first action")
	testutil.AssertTrue(t, players[1].Selection().GetPendingColonySelection() == nil, "no colony selection during setup")
	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "finish setup with first action queued")
	testutil.AssertTrue(t, testGame.InitPhaseRoster(), "setup shows roster")
	testutil.AssertNoError(t, confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID()), "finish roster")
	testutil.AssertEqual(t, shared.GamePhaseAction, testGame.CurrentPhase(), "setup can advance")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, playerID2, 2), "start Poseidon's turn")
	selection := players[1].Selection().GetPendingColonySelection()
	testutil.AssertTrue(t, selection != nil, "first turn creates colony selection")
	testutil.AssertNoError(t, confirmColony.Execute(ctx, testGame.ID(), playerID2, selection.AvailableColonyIDs[0]), "build colony")
	testutil.AssertTrue(t, testGame.GetForcedFirstAction(playerID2) == nil, "first action completes")
	testutil.AssertEqual(t, 1, testGame.CurrentTurn().ActionsRemaining(), "first action uses one action")

}

func TestPoseidon_ForcedColonyInitAdvance_FourPlayerPrelude_Repro568(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 4)
	logger := testutil.TestLogger()
	cardRegistry := testutil.CreateTestCardRegistry()
	ctx := context.Background()

	colonyDefs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	testutil.AssertNoError(t, err, "Failed to load colonies")
	colonyRegistry := colony.NewInMemoryColonyRegistry(colonyDefs)

	testGame.UpdateSettings(ctx, shared.GameSettings{
		MaxPlayers: 5,
		GameModeID: "classic", Features: []string{"prelude"}, Systems: []string{"colonies"},
	})

	err = testGame.UpdateStatus(ctx, shared.GameStatusActive)
	testutil.AssertNoError(t, err, "Failed to set active status")
	err = testGame.UpdatePhase(ctx, shared.GamePhaseStartingSelection)
	testutil.AssertNoError(t, err, "Failed to set starting selection phase")

	addColony(testGame, "luna", 1, nil)
	addColony(testGame, "io", 1, nil)
	addColony(testGame, "ganymede", 1, nil)
	addColony(testGame, "europa", 1, nil)

	players := testGame.GetAllPlayers()
	ids := make([]string, len(players))
	for i, p := range players {
		ids[i] = p.ID()
		testutil.SetPlayerCredits(ctx, p, 100)
	}
	err = testGame.SetTurnOrder(ctx, ids)
	testutil.AssertNoError(t, err, "Failed to set turn order")

	poseidonID := testutil.CardID("Poseidon")
	corpForPlayer := []string{"B01", poseidonID, "B02", "B03"}
	preludePool := []string{"P01", "P03", "P04", "P07"}

	deck := testGame.Deck()
	for i, p := range players {
		err = testGame.SetSelectCorporationPhase(ctx, p.ID(), &shared.SelectCorporationPhase{
			AvailableCorporations: []string{corpForPlayer[i]},
		})
		testutil.AssertNoError(t, err, "Failed to set corp phase")

		err = testGame.SetSelectPreludeCardsPhase(ctx, p.ID(), &shared.SelectPreludeCardsPhase{
			AvailablePreludes: preludePool,
			MaxSelectable:     2,
		})
		testutil.AssertNoError(t, err, "Failed to set prelude phase")

		projectCards, drawErr := deck.DrawProjectCards(ctx, 10)
		testutil.AssertNoError(t, drawErr, "Failed to draw project cards")
		err = testGame.SetSelectStartingCardsPhase(ctx, p.ID(), &shared.SelectStartingCardsPhase{
			AvailableCards: projectCards,
		})
		testutil.AssertNoError(t, err, "Failed to set starting cards phase")
	}

	selectAction := turnAction.NewSelectStartingChoicesAction(repo, cardRegistry, nil, logger)
	for i, p := range players {
		err = selectAction.Execute(ctx, testGame.ID(), p.ID(), corpForPlayer[i], []string{"P01", "P03"}, []string{}, shared.NativePayment(shared.ResourceCredit, len([]string{})*3))
		testutil.AssertNoError(t, err, "selection for player "+p.ID())
	}

	testutil.AssertEqual(t, shared.GamePhaseInitApplyCorp, testGame.CurrentPhase(), "Should be in init_apply_corp")

	confirmAction := turnAction.NewConfirmInitAdvanceAction(repo, cardRegistry, nil, nil, logger)
	confirmColony := confirmation.NewConfirmColonyPlacementAction(repo, cardRegistry, colonyRegistry, logger)

	turnOrder := testGame.TurnOrder()
	for _, pid := range turnOrder {
		err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
		testutil.AssertNoError(t, err, "apply corp for "+pid)

		if sel := players[indexOf(ids, pid)].Selection().GetPendingColonySelection(); sel != nil {
			err = confirmColony.Execute(ctx, testGame.ID(), pid, sel.AvailableColonyIDs[0])
			testutil.AssertNoError(t, err, "place forced colony for "+pid)
			continue
		}

		err = confirmAction.Execute(ctx, testGame.ID(), testGame.HostPlayerID())
		testutil.AssertNoError(t, err, "advance past "+pid)
	}

	testutil.AssertEqual(t, shared.GamePhaseInitApplyPrelude, testGame.CurrentPhase(),
		"After all corps applied, should advance to init_apply_prelude (NOT stuck in init_apply_corp)")
}

func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func TestStormcraft_PaysHeatActionWithoutPayingCreditCosts(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := setupCorporationColoniesGame(t)
	p, _ := g.GetPlayer(id)
	log := testutil.TestLogger()
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, "CC5"), "Stormcraft")
	p.Resources().Set(shared.Resources{})
	p.Resources().AddToStorage("CC5", 4)
	source := shared.PaymentSource{Target: "self-card", Resource: shared.ResourceFloater, CardID: "CC5"}
	creditCard := testutil.GetCardByName("Imported GHG")
	p.Hand().AddCard(creditCard.ID)
	bad := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: source, TargetResource: shared.ResourceCredit, Amount: 4}}}
	testutil.AssertErrorContains(t, cardAction.NewPlayCardAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, creditCard.ID, bad, nil, nil, nil, nil, nil), "ineligible payment source for credit", "floaters cannot pay credits")
	testutil.AssertEqual(t, 4, p.Resources().GetCardStorage("CC5"), "rejected credit payment preserves floaters")
	card := testutil.GetCardByName("Caretaker Contract")
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	var index int
	for i, b := range card.Behaviors {
		if gamecards.HasManualTrigger(b) {
			index = i
			p.Actions().AddAction(shared.CardAction{CardID: card.ID, CardName: card.Name, BehaviorIndex: i, Behavior: b})
		}
	}
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: source, TargetResource: shared.ResourceHeat, Amount: 4}}}
	before := p.Resources().TerraformRating()
	testutil.AssertNoError(t, cardAction.NewUseCardActionAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, card.ID, index, nil, nil, nil, nil, nil, &payment, nil, nil), "pay heat input with floaters")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage("CC5"), "four floaters spent")
	testutil.AssertEqual(t, before+1, p.Resources().TerraformRating(), "action reward")
}

func setupCorporationColoniesGame(t *testing.T) (*game.Game, game.GameRepository, gamecards.CardRegistry, string, string) {
	g, repo, registry, id, other := testutil.SetupTwoPlayerGame(t)
	settings := g.Settings()
	settings.Systems = append(settings.Systems, "colonies")
	g.UpdateSettings(context.Background(), settings)
	defs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	testutil.AssertNoError(t, err, "load colony catalog")
	g.Colonies().SetDefinitions(defs)
	return g, repo, registry, id, other
}

func TestAridor_AddsUnownedTileAndActivatesOnResourceHolder(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, other := setupCorporationColoniesGame(t)
	addColony(g, "luna", 1, nil)
	p, _ := g.GetPlayer(id)
	eventsSeen := 0
	events.Subscribe(g.EventBus(), func(events.ColonyBuiltEvent) { eventsSeen++ })
	defs, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	testutil.AssertNoError(t, err, "load colonies")
	colonies := colony.NewInMemoryColonyRegistry(defs)
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, testutil.CardID("Aridor")), "start Aridor action")
	selection := p.Selection().GetPendingColonySelection()
	testutil.AssertTrue(t, selection != nil && selection.AddTile, "choose unused tile")
	view := dto.ToGameDto(g, registry, id, colonies)
	testutil.AssertEqual(t, len(defs)-1, len(view.CurrentPlayer.PendingColonySelection.TileOptions), "unused tile previews sent")
	skip := turnAction.NewSkipActionAction(repo, nil, testutil.TestLogger())
	testutil.AssertErrorContains(t, skip.Execute(ctx, g.ID(), id), "corporation first action must finish first", "cannot pass first action")
	confirm := confirmation.NewConfirmColonyPlacementAction(repo, registry, colonies, testutil.TestLogger())
	testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), other, "titan"), "not your turn", "other player cannot confirm")
	testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, "luna"), "colony luna is not available for selection", "existing tile rejected")
	before := p.Resources().Get()
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, "titan"), "add Titan")
	titan := g.Colonies().GetState("titan")
	testutil.AssertEqual(t, 0, len(titan.PlayerColonies), "no player colony granted")
	testutil.AssertEqual(t, "floater", titan.AwaitingResource, "Titan starts inactive")
	testutil.AssertEqual(t, -1, titan.MarkerPosition, "inactive marker off track")
	testutil.AssertEqual(t, 0, eventsSeen, "adding tile does not trigger colony building")
	testutil.AssertEqual(t, before, p.Resources().Get(), "no placement reward")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "one action spent")
	testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, "titan"), "no pending colony selection", "duplicate confirmation rejected")
	testutil.AssertEqual(t, 2, len(g.Colonies().States()), "no duplicate tile")
	p.Hand().AddCard("213")
	p.Resources().Set(shared.Resources{Credits: 100})
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, "213", shared.NativePayment(shared.ResourceCredit, 11), nil, nil, nil, nil, nil), "play floater holder")
	testutil.AssertEqual(t, "", g.Colonies().GetState("titan").AwaitingResource, "holder activates Titan")
	testutil.AssertEqual(t, 1, g.Colonies().GetState("titan").MarkerPosition, "activation sets normal starting marker")
}

func TestPoseidon_FirstActionWaitsForOceanReward(t *testing.T) {
	g, repo, registry, id, _ := setupCorporationColoniesGame(t)
	ctx := context.Background()
	addColony(g, "europa", 1, nil)
	definitions, err := colony.LoadColoniesFromJSON("../../../assets/colonies.json")
	testutil.AssertNoError(t, err, "load colonies")
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, testutil.CardID("Poseidon")), "start Poseidon action")
	confirm := confirmation.NewConfirmColonyPlacementAction(repo, registry, colony.NewInMemoryColonyRegistry(definitions), testutil.TestLogger())
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, "europa"), "build Europa colony")
	testutil.AssertTrue(t, g.GetForcedFirstAction(id) != nil, "ocean reward keeps action open")
	testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "not consumed before reward")
	pending := g.GetPendingTileSelection(id)
	testutil.AssertTrue(t, pending != nil, "ocean selection exists")
	_, err = tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, pending.AvailableHexes[0])
	testutil.AssertNoError(t, err, "place ocean")
	testutil.AssertTrue(t, g.GetForcedFirstAction(id) == nil, "first action completes after reward")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "only one action consumed")
}
