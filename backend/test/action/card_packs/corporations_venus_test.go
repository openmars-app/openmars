package card_packs_test

import (
	"context"
	"fmt"
	"testing"

	baseaction "openmars/internal/action"
	"openmars/internal/action/admin"
	cardAction "openmars/internal/action/card"
	"openmars/internal/action/confirmation"
	tileaction "openmars/internal/action/tile"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func TestAphrodite_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Aphrodite"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 47, resources.Credits, "Aphrodite should have 47 starting credits")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 1, production.Plants, "Aphrodite should start with 1 plant production")
}

func TestAphrodite_Gain2MCWhenVenusRaised(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Aphrodite"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 47, resources.Credits, "Aphrodite should have 47 credits before Venus increase")

	_, err = testGame.GlobalParameters().IncreaseVenus(ctx, 1, "")
	testutil.AssertNoError(t, err, "IncreaseVenus failed")

	resources = p.Resources().Get()
	testutil.AssertEqual(t, 49, resources.Credits, "Aphrodite should have 49 credits after Venus increase (gained 2 M€)")
}

func TestCelestic_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Celestic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 42, resources.Credits, "Celestic should start with 42 credits")
}

func TestCelestic_FloaterStorageRegistered(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Celestic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	corpCardID := testutil.CardID("Celestic")
	storageMap := p.Resources().Storage()
	_, hasStorage := storageMap[corpCardID]
	testutil.AssertTrue(t, hasStorage, "Celestic should have floater storage initialized on the corp card")
}

func TestCelestic_FirstActionDrawsFloaterCards(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	// Set up a controlled deck: 3 non-floater cards followed by 2 floater cards
	// The draw-until-matching logic should skip non-floater cards and find the floater ones
	floaterCardIDs := []string{"213", "222"} // Aerial Mappers (floater storage), Dirigibles (floater storage+output)
	nonFloaterCardIDs := []string{"001", "002", "003"}
	allProjectCards := append(nonFloaterCardIDs, floaterCardIDs...)
	testGame.InitDeck(allProjectCards, nil, nil)

	// Clear hand before setting corporation
	p, _ := testGame.GetPlayer(playerID)
	for _, c := range p.Hand().Cards() {
		p.Hand().RemoveCard(c)
	}

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Celestic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	handCards := p.Hand().Cards()
	testutil.AssertEqual(t, 2, len(handCards),
		"Celestic first action should draw exactly 2 cards")

	// Verify both drawn cards are floater cards
	for _, cID := range handCards {
		card, err := cardRegistry.GetByID(cID)
		testutil.AssertNoError(t, err, "Card should exist in registry")
		hasFloater := false
		if card.ResourceStorage != nil && card.ResourceStorage.Type == shared.ResourceFloater {
			hasFloater = true
		}
		for _, b := range card.Behaviors {
			for _, o := range b.Outputs {
				if o.GetResourceType() == shared.ResourceFloater {
					hasFloater = true
				}
			}
			for _, i := range b.Inputs {
				if i.GetResourceType() == shared.ResourceFloater {
					hasFloater = true
				}
			}
		}
		testutil.AssertTrue(t, hasFloater,
			"Card "+card.Name+" ("+cID+") should have floater resource")
	}

	// Verify non-floater cards were NOT drawn into hand
	for _, nonFloaterID := range nonFloaterCardIDs {
		found := false
		for _, handCard := range handCards {
			if handCard == nonFloaterID {
				found = true
			}
		}
		testutil.AssertTrue(t, !found,
			"Non-floater card "+nonFloaterID+" should not be in hand")
	}
}

func TestCelestic_AddFloaterToAnyCard(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Celestic"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)

	corpCardID := testutil.CardID("Celestic")
	p.PlayedCards().AddCard(corpCardID, "Celestic", "corporation", []string{"venus"})
	p.Resources().AddToStorage(corpCardID, 0)

	otherCardID := testutil.CardID("Aerial Mappers")
	p.PlayedCards().AddCard(otherCardID, "Aerial Mappers", "active", []string{"venus", "science"})
	p.Resources().AddToStorage(otherCardID, 0)

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), playerID, corpCardID, 2, nil, []string{otherCardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Celestic add floater to another card should succeed")

	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(corpCardID), "Celestic corp card should still have 0 floaters")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(otherCardID), "Target card should have 1 floater after action")
}

func TestManutech_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Manutech"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 35, resources.Credits, "Manutech should start with 35 credits")
	testutil.AssertEqual(t, 1, resources.Steel, "Manutech should start with 1 steel from production-increased trigger")
	testutil.AssertEqual(t, 0, resources.Titanium, "Manutech should start with 0 titanium")
	testutil.AssertEqual(t, 0, resources.Plants, "Manutech should start with 0 plants")
	testutil.AssertEqual(t, 0, resources.Energy, "Manutech should start with 0 energy")
	testutil.AssertEqual(t, 0, resources.Heat, "Manutech should start with 0 heat")

	production := p.Resources().Production()
	testutil.AssertEqual(t, 1, production.Steel, "Manutech should start with 1 steel production")
}

func TestManutech_GainResourceWhenProductionIncreased(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Manutech"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	steelCount := p.Resources().Get().Steel
	testutil.AssertEqual(t, 1, steelCount, "Manutech should start with 1 steel from production-increased trigger")

	energyBefore := p.Resources().Get().Energy

	deepWellHeatingID := testutil.CardID("Deep Well Heating")
	p.Hand().AddCard(deepWellHeatingID)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})

	playCard := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 13)
	err = playCard.Execute(ctx, testGame.ID(), playerID, deepWellHeatingID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Deep Well Heating should play successfully")

	energyAfter := p.Resources().Get().Energy
	testutil.AssertTrue(t, energyAfter >= energyBefore+1, "Manutech should gain energy when energy production is increased")
}

func TestMorningStarInc_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Morning Star Inc."))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 50, resources.Credits, "Morning Star Inc. should start with 50 credits")
}

func TestMorningStarInc_VenusLenienceRegistered(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Morning Star Inc."))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	effects := p.Effects().List()
	found := false
	for _, effect := range effects {
		if effect.CardName == "Morning Star Inc." && effect.BehaviorIndex == 2 {
			found = true
			testutil.AssertEqual(t, shared.ResourceGlobalParameterLenience, effect.Behavior.Outputs[0].GetResourceType(),
				"Morning Star Inc. effect output should be global parameter lenience")
			testutil.AssertEqual(t, 2, effect.Behavior.Outputs[0].GetAmount(),
				"Morning Star Inc. venus lenience amount should be 2")
			break
		}
	}
	testutil.AssertTrue(t, found, "Morning Star Inc. should have registered its venus lenience effect at behavior index 2")

	calculator := gamecards.NewRequirementModifierCalculator(cardRegistry)
	venusLenience := calculator.CalculateGlobalParameterRequirementOffset(p, "venus")
	testutil.AssertEqual(t, 4, venusLenience, "Morning Star Inc. should allow 4 percentage points on Venus")
	tempLenience := calculator.CalculateGlobalParameterRequirementOffset(p, "temperature")
	testutil.AssertEqual(t, 0, tempLenience, "Morning Star Inc. should NOT provide temperature lenience")
}

func TestViron_StartingResources(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 48, resources.Credits, "Viron should start with 48 credits")
}

func TestViron_HasActionReuseAction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	vironCardID := testutil.CardID("Viron")
	actions := p.Actions().List()
	hasVironAction := false
	for _, a := range actions {
		if a.CardID == vironCardID {
			for _, output := range a.Behavior.Outputs {
				if output.GetResourceType() == shared.ResourceActionReuse {
					hasVironAction = true
					break
				}
			}
		}
	}
	testutil.AssertTrue(t, hasVironAction, "Viron should have a manual action-reuse action")
}

func TestViron_ReuseBlueCardAction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	vironCardID := testutil.CardID("Viron")

	p.PlayedCards().AddCard("test-blue-card", "Test Blue Card", "active", []string{"microbe"})
	p.Resources().AddToStorage("test-blue-card", 0)

	testBlueAction := shared.CardAction{
		CardID:        "test-blue-card",
		CardName:      "Test Blue Card",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
			},
		},
		TimesUsedThisGeneration: 1,
	}

	existingActions := p.Actions().List()
	existingActions = append(existingActions, testBlueAction)
	p.Actions().SetActions(existingActions)

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)

	err = useAction.Execute(ctx, testGame.ID(), playerID, "test-blue-card", 0, nil, []string{"test-blue-card"}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "action already played this generation", "Blue card action should fail because already used this generation")

	reuseSource := vironCardID
	err = useAction.Execute(ctx, testGame.ID(), playerID, "test-blue-card", 0, nil, []string{"test-blue-card"}, nil, nil, nil, nil, &reuseSource, nil)
	testutil.AssertNoError(t, err, "Viron should allow reusing already-used blue card action")

	storage := p.Resources().GetCardStorage("test-blue-card")
	testutil.AssertEqual(t, 1, storage, "Test blue card should have gained 1 microbe from reuse")

	actions := p.Actions().List()
	for _, a := range actions {
		if a.CardID == vironCardID {
			for _, output := range a.Behavior.Outputs {
				if output.GetResourceType() == shared.ResourceActionReuse {
					testutil.AssertEqual(t, 1, a.TimesUsedThisGeneration, "Viron action should be marked as used")
				}
			}
		}
	}
}

func TestViron_CannotReuseUnusedAction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	vironCardID := testutil.CardID("Viron")

	p.PlayedCards().AddCard("test-blue-card", "Test Blue Card", "active", []string{"microbe"})
	p.Resources().AddToStorage("test-blue-card", 0)

	testBlueAction := shared.CardAction{
		CardID:        "test-blue-card",
		CardName:      "Test Blue Card",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
			},
		},
		TimesUsedThisGeneration: 0,
	}

	existingActions := p.Actions().List()
	existingActions = append(existingActions, testBlueAction)
	p.Actions().SetActions(existingActions)

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	reuseSource := vironCardID
	err = useAction.Execute(ctx, testGame.ID(), playerID, "test-blue-card", 0, nil, []string{"test-blue-card"}, nil, nil, nil, nil, &reuseSource, nil)
	testutil.AssertErrorContains(t, err, "target must be a manual action already used this generation", "Should not be able to reuse an action that has not been used this generation")
}

func TestViron_CannotReuseSelf(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	vironCardID := testutil.CardID("Viron")

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	reuseSource := vironCardID
	err = useAction.Execute(ctx, testGame.ID(), playerID, vironCardID, 1, nil, nil, nil, nil, nil, nil, &reuseSource, nil)
	testutil.AssertErrorContains(t, err, "cannot reuse an action-reuse ability", "Should not be able to reuse own action-reuse ability")
}

func TestViron_CannotReuseAfterAlreadyUsedThisGen(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	vironCardID := testutil.CardID("Viron")

	p.PlayedCards().AddCard("test-blue-card", "Test Blue Card", "active", []string{"microbe"})
	p.Resources().AddToStorage("test-blue-card", 0)

	testBlueAction := shared.CardAction{
		CardID:        "test-blue-card",
		CardName:      "Test Blue Card",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
			},
		},
		TimesUsedThisGeneration: 1,
	}

	existingActions := p.Actions().List()
	existingActions = append(existingActions, testBlueAction)
	p.Actions().SetActions(existingActions)

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	reuseSource := vironCardID

	err = useAction.Execute(ctx, testGame.ID(), playerID, "test-blue-card", 0, nil, []string{"test-blue-card"}, nil, nil, nil, nil, &reuseSource, nil)
	testutil.AssertNoError(t, err, "First Viron reuse should succeed")

	p.Resources().AddToStorage("test-blue-card", 0)

	testBlueAction2 := shared.CardAction{
		CardID:        "test-blue-card-2",
		CardName:      "Test Blue Card 2",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
			},
		},
		TimesUsedThisGeneration: 1,
	}
	currentActions := p.Actions().List()
	currentActions = append(currentActions, testBlueAction2)
	p.Actions().SetActions(currentActions)

	err = useAction.Execute(ctx, testGame.ID(), playerID, "test-blue-card-2", 0, nil, nil, nil, nil, nil, nil, &reuseSource, nil)
	testutil.AssertErrorContains(t, err, "reuse action already played this generation", "Viron should not be able to reuse again this generation")
}

func TestViron_ReuseStratopolis_AddFloatersToSelf(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Viron"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	vironCardID := testutil.CardID("Viron")
	stratopolisCardID := testutil.CardID("Stratopolis")

	p.PlayedCards().AddCard(stratopolisCardID, "Stratopolis", "active", []string{"city", "venus"})
	p.Resources().AddToStorage(stratopolisCardID, 0)

	stratopolisAction := shared.CardAction{
		CardID:        stratopolisCardID,
		CardName:      "Stratopolis",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceFloater, 2, "any-card"),
			},
		},
		TimesUsedThisGeneration: 0,
	}

	existingActions := p.Actions().List()
	existingActions = append(existingActions, stratopolisAction)
	p.Actions().SetActions(existingActions)

	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)

	// Step 1: Use Stratopolis action normally to add 2 floaters to itself
	err = useAction.Execute(ctx, testGame.ID(), playerID, stratopolisCardID, 1, nil, []string{stratopolisCardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Stratopolis action should succeed")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(stratopolisCardID), "Stratopolis should have 2 floaters after first use")

	// Step 2: Use Viron to reuse Stratopolis action, adding 2 more floaters to itself
	reuseSource := vironCardID
	err = useAction.Execute(ctx, testGame.ID(), playerID, stratopolisCardID, 1, nil, []string{stratopolisCardID}, nil, nil, nil, nil, &reuseSource, nil)
	testutil.AssertNoError(t, err, "Viron reuse of Stratopolis should succeed")
	testutil.AssertEqual(t, 4, p.Resources().GetCardStorage(stratopolisCardID), "Stratopolis should have 4 floaters after Viron reuse")

	// Verify Viron's action is marked as used
	actions := p.Actions().List()
	for _, a := range actions {
		if a.CardID == vironCardID {
			for _, output := range a.Behavior.Outputs {
				if output.GetResourceType() == shared.ResourceActionReuse {
					testutil.AssertEqual(t, 1, a.TimesUsedThisGeneration, "Viron action should be marked as used")
				}
			}
		}
	}
}

func TestValleyTrust_FirstActionCreatesPreludeSelection(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	// Set up controlled prelude deck with exactly 5 preludes
	preludeIDs := []string{"P01", "P02", "P03", "P04", "P05"}
	testGame.InitDeck(nil, nil, preludeIDs)

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Valley Trust"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	selection := p.Selection().GetPendingCardDrawSelection()
	testutil.AssertTrue(t, selection != nil, "Valley Trust should create a pending card draw selection")
	testutil.AssertEqual(t, 3, len(selection.AvailableCards), "Should have 3 prelude cards available")
	testutil.AssertEqual(t, 1, selection.FreeTakeCount, "Should be able to take 1 card for free")
	testutil.AssertEqual(t, 0, selection.MaxBuyCount, "Should not be able to buy cards")
	testutil.AssertTrue(t, selection.PlayAsPrelude, "Selection should be marked as prelude")

	// Verify all available cards are prelude cards
	for _, cardID := range selection.AvailableCards {
		card, err := cardRegistry.GetByID(cardID)
		testutil.AssertNoError(t, err, "Card should exist in registry")
		testutil.AssertEqual(t, string(gamecards.CardTypePrelude), string(card.Type),
			"Available card "+card.Name+" should be a prelude card")
	}

	// Verify forced first action was set
	forcedAction := testGame.GetForcedFirstAction(playerID)
	testutil.AssertTrue(t, forcedAction != nil, "Should create forced first action")
	testutil.AssertEqual(t, "resolving", forcedAction.State, "First action should be resolving")
}

func TestValleyTrust_ConfirmPreludeSelection(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()

	// Set up controlled prelude deck
	preludeIDs := []string{"P01", "P02", "P03"}
	testGame.InitDeck(nil, nil, preludeIDs)

	setCorp := admin.NewSetCorporationAction(repo, cardRegistry, nil, logger)
	err := setCorp.Execute(ctx, testGame.ID(), playerID, testutil.CardID("Valley Trust"))
	testutil.AssertNoError(t, err, "SetCorporation should succeed")

	p, _ := testGame.GetPlayer(playerID)
	selection := p.Selection().GetPendingCardDrawSelection()
	testutil.AssertTrue(t, selection != nil, "Should have pending selection")

	// Record credits before confirming (Valley Trust starts with 37)
	creditsBefore := p.Resources().Get().Credits

	// Take the first available prelude card (Allied Bank - P01 gives 3 credits + 4 credit production)
	selectedPreludeID := "P01"

	confirmAction := confirmation.NewConfirmCardDrawAction(repo, cardRegistry, logger)
	err = confirmAction.Execute(ctx, testGame.ID(), playerID, []string{selectedPreludeID}, nil, shared.NativePayment(shared.ResourceCredit, 0*3))
	testutil.AssertNoError(t, err, "Confirm card draw should succeed")

	// Verify prelude was played (added to played cards)
	playedCards := p.PlayedCards().Cards()
	found := false
	for _, pc := range playedCards {
		if pc == selectedPreludeID {
			found = true
			break
		}
	}
	testutil.AssertTrue(t, found, "Selected prelude should be in played cards")

	// Verify prelude effects were applied (Allied Bank gives 3 credits)
	if selectedPreludeID == "P01" {
		creditsAfter := p.Resources().Get().Credits
		testutil.AssertEqual(t, creditsBefore+3, creditsAfter,
			"Allied Bank prelude should have given 3 credits")

		production := p.Resources().Production()
		testutil.AssertEqual(t, 4, production.Credits,
			"Allied Bank prelude should have given 4 credit production")
	}

	// Verify selection was cleared
	testutil.AssertTrue(t, p.Selection().GetPendingCardDrawSelection() == nil,
		"Pending card draw selection should be cleared")

	// Verify forced first action was cleared
	testutil.AssertTrue(t, testGame.GetForcedFirstAction(playerID) == nil,
		"Forced first action should be cleared")

	// Verify unselected preludes were removed permanently (not discarded)
	removedCards := testGame.Deck().RemovedCards()
	testutil.AssertEqual(t, 2, len(removedCards), "2 unselected preludes should be permanently removed")
}

func TestMorningStarInc_RequirementStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Morning Star Inc."}, [4]int{0, 0, 0, 2})
}

func TestMorningStarInc_StacksWithSpecialDesignAtStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Morning Star Inc.", "Special Design"}, [4]int{2, 2, 2, 4})
}

func TestCelestic_DeferredDrawIncludesAllFloaterReferences(t *testing.T) {
	assertCorporationDrawReceipt(t, "Celestic", []string{"001", "214", "C10"}, 2, 1)
}

func TestMorningStar_DeferredDrawReceipt(t *testing.T) {
	assertCorporationDrawReceipt(t, "Morning Star Inc.", []string{"001", "213", "222", "248"}, 3, 1)
}

func TestCelestic_ExhaustedSearchDoesNotCreateEmptyReceipt(t *testing.T) {
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	g.InitDeck([]string{"001", "002"}, nil, nil)
	p, _ := g.GetPlayer(id)
	before := len(p.Hand().Cards())
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, testutil.CardID("Celestic")), "search exhausted")
	testutil.AssertEqual(t, before, len(p.Hand().Cards()), "no matching cards")
	testutil.AssertEqual(t, 0, len(p.Selection().CardReceipts()), "no empty receipt")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "search still consumes action")
	testutil.AssertEqual(t, 2, len(g.Deck().DiscardPile()), "skipped cards discarded")
}

func TestViron_DeferredPurchaseCompletesReuse(t *testing.T) {
	for _, name := range []string{"Inventors' Guild", "Business Network"} {
		for _, buy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/buy=%t", name, buy), func(t *testing.T) {
				g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
				ctx := context.Background()
				testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, "V05"), "set Viron")
				p, _ := g.GetPlayer(id)
				target := testutil.GetCardByName(name)
				behaviorIndex := 0
				for i, behavior := range target.Behaviors {
					for _, trigger := range behavior.Triggers {
						if trigger.Type == shared.TriggerTypeManual {
							behaviorIndex = i
						}
					}
				}
				p.PlayedCards().AddCard(target.ID, target.Name, "active", nil)
				p.Actions().SetActions(append(p.Actions().List(), shared.CardAction{CardID: target.ID, CardName: target.Name, BehaviorIndex: behaviorIndex, Behavior: target.Behaviors[behaviorIndex], TimesUsedThisGeneration: 1}))
				g.InitDeck([]string{"001", "002", "003"}, nil, nil)
				testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "set turn")
				use := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger())
				source := "V05"
				testutil.AssertNoError(t, use.Execute(ctx, g.ID(), id, target.ID, behaviorIndex, nil, nil, nil, nil, nil, nil, &source, nil), "reuse")
				selection := p.Selection().GetPendingCardDrawSelection()
				if selection == nil {
					t.Fatal("expected purchase selection")
				}
				testutil.AssertEqual(t, target.ID, selection.SourceCardID, "effect source remains target")
				reconnected := dto.ToGameDtoFull(g, registry, id, dto.Registries{})
				if reconnected.CurrentPlayer.PendingCardDrawSelection == nil {
					t.Fatal("reconnecting loses pending purchase")
				}
				testutil.AssertEqual(t, selection.AvailableCards[0], reconnected.CurrentPlayer.PendingCardDrawSelection.AvailableCards[0].ID, "same offered card on reconnect")

				testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "completion deferred")
				bought := []string{}
				payment := shared.NativePayment(shared.ResourceCredit, 0)
				if buy {
					bought = append(bought, selection.AvailableCards[0])
					payment = shared.NativePayment(shared.ResourceCredit, selection.CardBuyCost)
				}
				confirm := confirmation.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
				testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, nil, bought, payment), "confirm")
				testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "one turn action consumed")
				for _, action := range p.Actions().List() {
					if action.CardID == source {
						testutil.AssertEqual(t, 1, action.TimesUsedThisGeneration, "Viron consumed")
					}
					if action.CardID == target.ID {
						testutil.AssertEqual(t, 1, action.TimesUsedThisGeneration, "target usage unchanged")
					}
				}
				testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, nil, bought, payment), "no pending card draw selection", "cannot confirm twice")
				testutil.AssertErrorContains(t, use.Execute(ctx, g.ID(), id, target.ID, behaviorIndex, nil, nil, nil, nil, nil, nil, &source, nil), "reuse action already played this generation", "Viron cannot repeat")
			})
		}
	}
}

func setupVironTarget(t *testing.T, name string) (*game.Game, game.GameRepository, gamecards.CardRegistry, string, shared.CardAction) {
	t.Helper()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, "V05"), "set Viron")
	p, _ := g.GetPlayer(id)
	target := testutil.GetCardByName(name)
	p.PlayedCards().AddCard(target.ID, target.Name, string(target.Type), nil)
	for index, behavior := range target.Behaviors {
		if gamecards.HasManualTrigger(behavior) {
			act := shared.CardAction{CardID: target.ID, CardName: target.Name, BehaviorIndex: index, Behavior: behavior, TimesUsedThisGeneration: 1}
			p.Actions().SetActions(append(p.Actions().List(), act))
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 2), "set turn")
			return g, repo, registry, id, act
		}
	}
	t.Fatal("manual action missing")
	return nil, nil, nil, "", shared.CardAction{}
}

func TestViron_VariableAmount(t *testing.T) {
	for _, amount := range []int{-2, -1, 0, 3, 6} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			g, repo, registry, id, target := setupVironTarget(t, "Power Infrastructure")
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceEnergy: 5})
			before := p.Resources().Get()
			view := dto.ToGameDtoFull(g, registry, id, dto.Registries{})
			for _, act := range view.CurrentPlayer.Actions {
				if act.CardID == "V05" {
					testutil.AssertEqual(t, 1, len(act.ReuseOptions), "server supplies reusable target")
					testutil.AssertTrue(t, act.ReuseOptions[0].Available, "zero or more energy may be spent")
				}
				if act.CardID == target.CardID {
					if act.Behavior.InputOptions == nil || act.Behavior.InputOptions.VariableAmount == nil {
						t.Fatal("amount picker metadata missing")
					}
					testutil.AssertEqual(t, 5, act.Behavior.InputOptions.VariableAmount.Max, "server amount limit")
				}
			}
			selected := &amount
			if amount == -2 {
				selected = nil
			}
			source := "V05"
			err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, nil, nil, nil, nil, selected, nil, &source, nil)
			if amount < 0 || amount > 5 {
				wantErr := map[int]string{-2: "must select an amount", -1: "selected amount must be nonnegative", 6: "payment exceeds available source pool"}[amount]
				testutil.AssertErrorContains(t, err, wantErr, "invalid amount rejected")
				testutil.AssertEqual(t, before, p.Resources().Get(), "invalid request preserves resources")
				testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "invalid request preserves turn")
				testutil.AssertEqual(t, 0, p.Actions().List()[0].TimesUsedThisGeneration, "Viron remains unused")
			} else {
				testutil.AssertNoError(t, err, "variable reuse")
				testutil.AssertEqual(t, 5-amount, p.Resources().Get().Energy, "energy paid")
				testutil.AssertEqual(t, before.Credits+amount, p.Resources().Get().Credits, "credits gained")
				testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "one action consumed")
				p.Actions().ResetGenerationCounts()
				testutil.AssertErrorContains(t, cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, nil, nil, nil, nil, selected, nil, &source, nil), "target must be a manual action already used this generation", "new generation needs target used again")
			}
		})
	}
}

func TestViron_TitaniumPaymentAndOceanCompletion(t *testing.T) {
	g, repo, registry, id, target := setupVironTarget(t, "Water Import From Europa")
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceTitanium: 4})
	before := p.Resources().Get().Credits
	quote, err := baseaction.QuoteActionPayment(g, p, registry, nil, nil, nil, baseaction.PaymentIntent{Action: "card-action", CardID: target.CardID, BehaviorIndex: target.BehaviorIndex})
	testutil.AssertNoError(t, err, "reuse uses the normal payment quote")
	testutil.AssertEqual(t, 12, quote.Costs[shared.ResourceCredit], "full action cost")
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-player", Resource: shared.ResourceTitanium}, TargetResource: shared.ResourceCredit, Amount: 4}}}
	source := "V05"
	testutil.AssertNoError(t, cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, nil, nil, nil, nil, nil, &payment, &source, nil), "pay titanium")
	testutil.AssertEqual(t, before, p.Resources().Get().Credits, "no credits paid")
	testutil.AssertEqual(t, 0, p.Resources().Get().Titanium, "titanium paid")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "one action consumed")
	pending := g.GetPendingTileSelection(id)
	if pending == nil {
		t.Fatal("ocean selection missing")
	}
	_, err = tileaction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, pending.AvailableHexes[0])
	testutil.AssertNoError(t, err, "place ocean")
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "tile confirmation does not consume again")
}

func TestViron_ChoiceValidationAndSubstituteAvailability(t *testing.T) {
	g, repo, registry, id, target := setupVironTarget(t, "Rotator Impacts")
	p, _ := g.GetPlayer(id)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: -p.Resources().Get().Credits, shared.ResourceTitanium: 2})
	options := baseaction.CalculateActionReuseOptions("V05", p, g, registry)
	testutil.AssertEqual(t, 1, len(options), "one used action")
	testutil.AssertEqual(t, 0, len(options[0].Errors), "titanium makes choice affordable")
	source := "V05"
	use := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger())
	for _, index := range []int{-2, -1, 2} {
		choice := &index
		wantErr := "invalid action choice"
		if index == -2 {
			choice = nil
			wantErr = "must select an action choice"
		}
		testutil.AssertErrorContains(t, use.Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, choice, nil, nil, nil, nil, nil, &source, nil), wantErr, "missing or invalid choice rejected")
		testutil.AssertEqual(t, 2, p.Resources().Get().Titanium, "invalid choice spends nothing")
	}
	index := 0
	testutil.AssertErrorContains(t, use.Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, &index, nil, nil, nil, nil, nil, &source, nil), "payment exceeds available source pool", "missing affordable payment rejected")
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-player", Resource: shared.ResourceTitanium}, TargetResource: shared.ResourceCredit, Amount: 2}}}
	testutil.AssertNoError(t, use.Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, &index, nil, nil, nil, nil, &payment, &source, nil), "valid choice with titanium")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(target.CardID), "asteroid goes on target")
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(source), "no asteroid on Viron")
}

func TestViron_NormalPurchaseDoesNotConsumeReuse(t *testing.T) {
	g, repo, registry, id, target := setupVironTarget(t, "Inventors' Guild")
	p, _ := g.GetPlayer(id)
	p.Actions().ResetGenerationCounts()
	g.InitDeck([]string{"001"}, nil, nil)
	testutil.AssertNoError(t, cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, target.CardID, target.BehaviorIndex, nil, nil, nil, nil, nil, nil, nil, nil), "normal action")
	selection := p.Selection().GetPendingCardDrawSelection()
	if selection == nil || selection.CompleteAction == nil {
		t.Fatal("missing deferred action completion")
	}
	testutil.AssertEqual(t, target.CardID, selection.CompleteAction.CardID, "normal action completes itself")
	testutil.AssertNoError(t, confirmation.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, nil, nil, shared.Payment{}), "discard")
	for _, action := range p.Actions().List() {
		if action.CardID == target.CardID {
			testutil.AssertEqual(t, 1, action.TimesUsedThisGeneration, "target used")
		}
		if action.CardID == "V05" {
			testutil.AssertEqual(t, 0, action.TimesUsedThisGeneration, "Viron unused")
		}
	}
	testutil.AssertEqual(t, 1, g.CurrentTurn().ActionsRemaining(), "one turn action consumed")
}

func TestManutech_CreditProductionGainAndDecrease(t *testing.T) {
	g, repo, registry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	ctx := context.Background()
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), playerID, testutil.CardID("Manutech")), "set corporation")
	p, _ := g.GetPlayer(playerID)
	before := p.Resources().Get().Credits
	p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceCreditProduction: 3})
	testutil.AssertEqual(t, before+3, p.Resources().Get().Credits, "gain full credit increase")
	p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceCreditProduction: -1})
	testutil.AssertEqual(t, before+3, p.Resources().Get().Credits, "decrease grants nothing")
}
