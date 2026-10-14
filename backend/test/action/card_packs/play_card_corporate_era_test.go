package card_packs_test

import (
	"context"
	"fmt"
	"slices"

	baseaction "openmars/internal/action"
	cardAction "openmars/internal/action/card"
	confirmAction "openmars/internal/action/confirmation"
	tileAction "openmars/internal/action/tile"
	"openmars/internal/delivery/dto"
	"openmars/internal/game"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
	"testing"
)

// =============================================================================
// Card 006: Inventors' Guild (active, cost 9, tags: [science])
// "Action: look at the top card and either buy it or discard it"
// Behavior: manual trigger, outputs: card-buy 1 + card-peek 1 to self-player
// =============================================================================
func TestInventorsGuild_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Inventors' Guild")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Inventors' Guild should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Inventors' Guild should be in played cards")
}
func TestInventorsGuild_ActionCanBeUsed(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Inventors' Guild")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 4), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	// Play the card first (registers the manual action)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Inventors' Guild should play successfully")
	// Use the card action (behavior index 0 since only one behavior, manual)
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Inventors' Guild action should succeed")
}

// =============================================================================
// Card 014: Development Center (active, cost 11, tags: [building, science])
// "Action: Spend 1 energy to draw a card."
// Behavior: manual trigger, inputs: energy 1, outputs: card-draw 1
// =============================================================================
func TestDevelopmentCenter_PlaysAndActionSpendEnergyDrawCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Development Center")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 4), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourceEnergy: 3,
	})
	p.Hand().AddCard(card.ID)
	energyBefore := p.Resources().Get().Energy
	// Play the card first
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Development Center should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Development Center should be in played cards")
	// Use the card action: spend 1 energy to draw a card
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Development Center action should succeed")
	energyAfter := p.Resources().Get().Energy
	testutil.AssertEqual(t, energyBefore-1, energyAfter,
		"Should have 1 less energy after using Development Center action")
}
func TestDevelopmentCenter_ActionFailsWithoutEnergy(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Development Center")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 4), "SetCurrentTurn failed")
	// Give credits to play the card but NO energy
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	// Play the card first
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Development Center should play successfully")
	// Try to use the action with 0 energy - should fail
	handBefore := p.Hand().CardCount()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Development Center action should fail without energy")
	testutil.AssertEqual(t, handBefore, p.Hand().CardCount(), "no card drawn")
}

// --- Space Station (025) ---
// active, cost 10, tags: [space]
// "Effect: When you play a space card, you pay 2 M€ less for it."
// Auto trigger, outputs: discount 2 to self-player with selector tags:[space].
func TestSpaceStation_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Space Station")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Space Station should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Space Station should be in played cards")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-10, creditsAfter,
		"Should have paid 10 credits for Space Station")
}

// --- Virus (050) ---
// event, cost 1, tags: [microbe]
// "Remove up to 2 animals or 5 plants from any player."
// Auto trigger with choices:
//
//	choice 0 = animal removal (2) from any-card
//	choice 1 = plant removal (5) from any-player
func TestVirus_Choice1_RemovePlantsFromOpponent(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Virus")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "SetCurrentTurn failed")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 8,
	})
	attacker.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	targetID := target.ID()
	choiceIndex := 1
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, &choiceIndex, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Virus should play successfully with choice 1 (remove plants)")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 3, targetResources.Plants, "Target should have 3 plants (8 - 5)")
}
func TestVirus_Choice1_PartialPlantRemoval(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Virus")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "SetCurrentTurn failed")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 2,
	})
	attacker.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	targetID := target.ID()
	choiceIndex := 1
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, &choiceIndex, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Virus should play successfully with partial plant removal")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 0, targetResources.Plants, "Target should have 0 plants (had 2, Virus removes up to 5)")
}
func TestVirus_Choice0_RemoveAnimalsFromCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Virus")
	animalHost := gamecards.Card{
		ID:              "card-animal-host-virus",
		Name:            "Animal Host",
		Type:            gamecards.CardTypeActive,
		Cost:            0,
		Tags:            []shared.CardTag{shared.TagAnimal},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	additionalCards := []gamecards.Card{animalHost}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.PlayedCards().AddCard("card-animal-host-virus", "Animal Host", "active", []string{"animal"})
	p.Resources().AddToStorage("card-animal-host-virus", 5)
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	targetCardID := "card-animal-host-virus"
	choiceIndex := 0
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, &choiceIndex, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Virus should play successfully with choice 0 (remove animals)")
	animalStorage := p.Resources().GetCardStorage("card-animal-host-virus")
	testutil.AssertEqual(t, 3, animalStorage, "Animal host should have 3 animals (5 - 2)")
}

// --- Electro Catapult (069) ---
// Active card, cost 17, tags: [building]. Requirements: oxygen max 8.
// Behavior 0 (auto): outputs energy-production -1 to self-player
// Behavior 1 (manual): choices: [spend 1 plant, spend 1 steel] -> outputs credit 7 to self-player
func TestElectroCatapult_PlayDecreasesEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Electro Catapult")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 17)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Electro Catapult should play successfully at 0% oxygen")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1 after playing Electro Catapult")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Electro Catapult should be in played cards")
}
func TestElectroCatapult_ActionSpendPlantGainCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-electro-catapult"
	p.PlayedCards().AddCard(cardID, "Electro Catapult", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 5,
	})
	creditsBefore := p.Resources().Get().Credits
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 7, "self-player"),
		},
		Choices: []shared.Choice{
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourcePlant, 1, "self-player"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Electro Catapult",
			BehaviorIndex: 1,
			Behavior:      behavior,
		},
	})
	choiceIndex := 0 // spend 1 plant
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, &choiceIndex, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Electro Catapult action (spend plant) should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 4, resources.Plants, "Should have 4 plants after spending 1")
	testutil.AssertEqual(t, creditsBefore+7, resources.Credits, "Should gain 7 credits")
}
func TestElectroCatapult_ActionSpendSteelGainCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-electro-catapult-steel"
	p.PlayedCards().AddCard(cardID, "Electro Catapult", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceSteel: 3,
	})
	creditsBefore := p.Resources().Get().Credits
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 7, "self-player"),
		},
		Choices: []shared.Choice{
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourcePlant, 1, "self-player"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player"),
				},
			},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Electro Catapult",
			BehaviorIndex: 1,
			Behavior:      behavior,
		},
	})
	choiceIndex := 1 // spend 1 steel
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, &choiceIndex, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Electro Catapult action (spend steel) should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Steel, "Should have 2 steel after spending 1")
	testutil.AssertEqual(t, creditsBefore+7, resources.Credits, "Should gain 7 credits")
}

// --- Earth Catapult (070) ---
// Active card, cost 23, tags: [earth].
// "When you play a card, you pay 2 M€ less."
// Auto trigger, outputs: discount 2 to self-player (no selectors = applies to all cards).
func TestEarthCatapult_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Earth Catapult")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 23)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Earth Catapult should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Earth Catapult should be in played cards")
}

// =============================================================================
// Card 071: Advanced Alloys (active, cost 9, tags: [science])
// "Each steel/titanium worth 1 M€ extra."
// Auto trigger, outputs: 2x value-modifier (1 for steel, 1 for titanium) to
// self-player with selectors.
// =============================================================================
func TestAdvancedAlloys_PlaysSuccessfully_ValueModifiersApplied(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Advanced Alloys")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	steelModBefore := p.Resources().GetValueModifier(shared.ResourceSteel)
	titaniumModBefore := p.Resources().GetValueModifier(shared.ResourceTitanium)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Advanced Alloys should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Advanced Alloys should be in played cards")
	steelModAfter := p.Resources().GetValueModifier(shared.ResourceSteel)
	titaniumModAfter := p.Resources().GetValueModifier(shared.ResourceTitanium)
	testutil.AssertEqual(t, steelModBefore+1, steelModAfter,
		"Steel value modifier should increase by 1")
	testutil.AssertEqual(t, titaniumModBefore+1, titaniumModAfter,
		"Titanium value modifier should increase by 1")
}

// =============================================================================
// Card 064: Mining Area (automated, cost 4, tags: [building])
// "Place mining tile on steel/titanium bonus area adjacent to your tile.
//
//	Increase production of that resource."
//
// Auto trigger with choices AND tile-placement output.
// Mining Area no longer uses choices — production is determined by which bonus tile is placed on.
// The tile-placed trigger handles this automatically (not yet implemented).
// These tests verify the card plays without a choice index and queues a tile placement.
// =============================================================================
func TestMiningArea_PlaceOnSteelBonus_IncreaseSteelProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Area")
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")

	// Place a player-owned tile adjacent to the steel bonus tile at (-4,3,1)
	adjacentToSteelBonus := shared.HexPosition{Q: -3, R: 3, S: 0}
	err := testGame.Board().UpdateTileOccupancy(ctx, adjacentToSteelBonus,
		board.TileOccupant{Type: shared.ResourceCityTile}, p.ID())
	testutil.AssertNoError(t, err, "Should place player tile adjacent to steel bonus")

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)

	prodBefore := p.Resources().Production()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, card.Cost)
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mining Area should play without requiring a choice index")

	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending mining tile selection")

	// Select the steel bonus hex at (-4,3,1) — land tile with Steel×2
	steelBonusHex := fmt.Sprintf("%d,%d,%d", -4, 3, 1)
	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err = selectTileAction.Execute(ctx, testGame.ID(), p.ID(), steelBonusHex)
	testutil.AssertNoError(t, err, "Should be able to select steel bonus hex")

	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Steel+1, prodAfter.Steel,
		"Steel production should increase by 1 when mining tile placed on steel bonus")
	testutil.AssertEqual(t, prodBefore.Titanium, prodAfter.Titanium,
		"Titanium production should not change")
}

func TestMiningArea_PlaceOnTitaniumBonus_IncreaseTitaniumProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Area")
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")

	// Place a player-owned tile adjacent to the titanium bonus tile at (1,3,-4)
	adjacentToTitaniumBonus := shared.HexPosition{Q: 0, R: 3, S: -3}
	err := testGame.Board().UpdateTileOccupancy(ctx, adjacentToTitaniumBonus,
		board.TileOccupant{Type: shared.ResourceCityTile}, p.ID())
	testutil.AssertNoError(t, err, "Should place player tile adjacent to titanium bonus")

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)

	prodBefore := p.Resources().Production()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, card.Cost)
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mining Area should play without requiring a choice index")

	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending mining tile selection")

	// Select the titanium bonus hex at (1,3,-4) — land tile with Titanium×1
	titaniumBonusHex := fmt.Sprintf("%d,%d,%d", 1, 3, -4)
	selectTileAction := tileAction.NewSelectTileAction(repo, cardRegistry, stateRepo, logger)
	_, err = selectTileAction.Execute(ctx, testGame.ID(), p.ID(), titaniumBonusHex)
	testutil.AssertNoError(t, err, "Should be able to select titanium bonus hex")

	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium,
		"Titanium production should increase by 1 when mining tile placed on titanium bonus")
	testutil.AssertEqual(t, prodBefore.Steel, prodAfter.Steel,
		"Steel production should not change")
}

// --- Mars University (073) ---
// "When you play a science tag, including this, you may discard a card from hand to draw a card."
// Passive triggered effect: auto trigger with condition type:"tag-played" for science tags.
// Optional card-discard input, card-draw output. Just test that the card plays successfully.
func TestMarsUniversity_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mars University")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mars University should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Mars University should be in played cards")
}

// --- Viral Enhancers (074) ---
// "When you play a plant, microbe, or an animal tag, including this, gain 1 plant or add 1 resource to that card."
// Passive triggered effect with choices. Just test that the card plays successfully.
func TestViralEnhancers_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Viral Enhancers")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Viral Enhancers should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Viral Enhancers should be in played cards")
}

// --- Robotic Workforce (086) ---
// "Duplicate only the production box of one of your building cards."
func TestRoboticWorkforce_RequiresProductionSourceChoice(t *testing.T) {
	testGame, repo, registry, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()
	p, err := testGame.GetPlayer(playerID)
	testutil.AssertNoError(t, err, "Get player")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	for _, name := range []string{"Mine", "Power Plant"} {
		source := testutil.GetCardByName(name)
		p.Hand().AddCard(source.ID)
		testutil.AssertNoError(t, play.Execute(ctx, testGame.ID(), playerID, source.ID, shared.NativePayment(shared.
			ResourceCredit, source.Cost), nil, nil, nil, nil, nil), "Play production source")
	}
	before := p.Resources().Production()
	card := testutil.GetCardByName("Robotic Workforce")
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, play.Execute(ctx, testGame.ID(), playerID, card.ID, shared.NativePayment(shared.
		ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "Play Robotic Workforce")
	testutil.AssertEqual(t, before, p.Resources().Production(), "Production must wait for the player's choice")
	testutil.AssertTrue(t, testGame.HasAnyPendingSelection(playerID),
		"Robotic Workforce must ask which building's production box to copy")
}

// --- Earth Office (105) ---
// Active card, cost 1, tags: [earth].
// "When you play an Earth tag, you pay 3 M€ less."
// Auto trigger, outputs: discount 3 to self-player with selector tags:[earth].
func TestEarthOffice_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Earth Office")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Earth Office should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Earth Office should be in played cards")
}

// --- Business Contacts (111) ---
// Event card, cost 7, tags: [earth].
// "Look at top 4 cards, take 2, discard 2."
// Auto trigger, outputs: card-take 2 + card-peek 4 to self-player.
func TestBusinessContacts_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Business Contacts")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Business Contacts should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Business Contacts should be in played cards")
}

// --- Sabotage (121) ---
// Event card, cost 1, tags: none.
// "Remove up to 3 titanium from any player, or 4 steel, or 7 M€."
// Auto trigger with 3 choices targeting any-player.
func TestSabotage_RemoveTitaniumFromOpponent(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Sabotage")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "SetCurrentTurn failed")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(card.ID)
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceTitanium: 5,
	})
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	choiceIndex := 0
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, &choiceIndex, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Sabotage should play successfully with choice 0 (remove titanium)")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 2, targetResources.Titanium, "Target should have 2 titanium after 3 removed (5 - 3)")
}
func TestSabotage_RemoveSteelFromOpponent(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Sabotage")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "SetCurrentTurn failed")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(card.ID)
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceSteel: 6,
	})
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	choiceIndex := 1
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, &choiceIndex, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Sabotage should play successfully with choice 1 (remove steel)")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 2, targetResources.Steel, "Target should have 2 steel after 4 removed (6 - 4)")
}

// --- CEO's Favorite Project (149) ---
// "Add 1 resource to a card with at least 1 resource on it."
// Event, cost 1, no tags. Auto trigger, outputs: card-resource 1 to any-card.
func TestCEOsFavoriteProject_AddsMicrobeToTargetCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	ceosFavorite := testutil.GetCardByName("CEO's Favorite Project")
	targetCard := gamecards.Card{
		ID:              "card-target-microbes",
		Name:            "Target Microbe Card",
		Type:            gamecards.CardTypeActive,
		Cost:            0,
		Tags:            []shared.CardTag{shared.TagMicrobe},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceMicrobe},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{ceosFavorite, targetCard})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	// Set up a played card with microbe storage containing 2 microbes
	p.PlayedCards().AddCard("card-target-microbes", "Target Microbe Card", "active", []string{"microbe"})
	p.Resources().AddToStorage("card-target-microbes", 2)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(ceosFavorite.ID)
	storageBefore := p.Resources().GetCardStorage("card-target-microbes")
	testutil.AssertEqual(t, 2, storageBefore, "Target card should start with 2 microbes")
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	targetCardID := "card-target-microbes"
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), ceosFavorite.ID, payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "CEO's Favorite Project should play successfully")
	storageAfter := p.Resources().GetCardStorage("card-target-microbes")
	testutil.AssertEqual(t, 3, storageAfter, "Target card should have 3 microbes (2 existing + 1 added)")
	testutil.AssertTrue(t, p.PlayedCards().Contains(ceosFavorite.ID),
		"CEO's Favorite Project should be in played cards")
	testutil.AssertEqual(t, false, p.Hand().HasCard(ceosFavorite.ID),
		"CEO's Favorite Project should be removed from hand")
}
func TestCEOsFavoriteProject_FailsWithoutTargetCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	ceosFavorite := testutil.GetCardByName("CEO's Favorite Project")
	targetCard := gamecards.Card{
		ID:              "card-target-animals",
		Name:            "Target Animal Card",
		Type:            gamecards.CardTypeActive,
		Cost:            0,
		Tags:            []shared.CardTag{shared.TagAnimal},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{ceosFavorite, targetCard})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.PlayedCards().AddCard("card-target-animals", "Target Animal Card", "active", []string{"animal"})
	p.Resources().AddToStorage("card-target-animals", 1)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(ceosFavorite.ID)
	// Play without specifying a target card - should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 1)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), ceosFavorite.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "select a card for resource storage", "Should fail without target card for card-resource output")
	testutil.AssertTrue(t, p.Hand().HasCard(ceosFavorite.ID), "rejected card stays in hand")
}

// --- Protected Habitats (173) ---
// "Opponents may not remove your plants/animals/microbes."
// Protects the player's plants, microbes and animals against opponents.
func TestProtectedHabitats_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	protectedHabitats := testutil.GetCardByName("Protected Habitats")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(protectedHabitats.ID)
	creditsBefore := p.Resources().Get().Credits
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), protectedHabitats.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Protected Habitats should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(protectedHabitats.ID),
		"Protected Habitats should be in played cards")
	testutil.AssertEqual(t, false, p.Hand().HasCard(protectedHabitats.ID),
		"Protected Habitats should be removed from hand")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-5, creditsAfter,
		"Should have paid 5 credits for Protected Habitats")
}

// --- Corporate Stronghold (182) ---
// "Decrease your energy production 1 step and increase your M€ production 3 steps. Place a city tile."
func TestCorporateStronghold_ProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Corporate Stronghold")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Corporate Stronghold should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}
func TestCorporateStronghold_FailsWithoutEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Corporate Stronghold")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient energy-production", "Corporate Stronghold should fail without energy production")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// --- Olympus Conference (185) ---
// "When you play a science tag, including this, either add a science resource to this card,
// or remove a science resource from this card to draw a card."
// Passive triggered effect with choices and resource storage. Just test that the card plays successfully.
func TestOlympusConference_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Olympus Conference")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Olympus Conference should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Olympus Conference should be in played cards")
}

// --- Invention Contest (192) ---
// "Look at top 3 cards, take 1, discard 2."
// Auto trigger, outputs: card-take 1 + card-peek 3.
func TestInventionContest_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Invention Contest")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 2)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Invention Contest should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Invention Contest should be in played cards")
}

// --- Power Infrastructure (194) ---
// "Action: Spend any number of energy to gain that amount of M€."
// Manual trigger, variableAmount inputs (energy) and outputs (credit).
func TestPowerInfrastructure_PlayAndUseAction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Power Infrastructure")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourceEnergy: 10,
	})
	p.Hand().AddCard(card.ID)
	// Play the card first
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 4)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Infrastructure should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Power Infrastructure should be in played cards")
	creditsBefore := p.Resources().Get().Credits
	energyBefore := p.Resources().Get().Energy
	// Give player another action since playing the card consumed one
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Use the action: spend 3 energy to gain 3 credits
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	selectedAmount := 3
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, 0, nil, nil, nil, nil, &selectedAmount, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Infrastructure action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, energyBefore-3, resources.Energy, "Energy should decrease by 3")
	testutil.AssertEqual(t, creditsBefore+3, resources.Credits, "Credits should increase by 3")
}
func TestPowerInfrastructure_UseActionSpendAllEnergy(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-power-infra-ce9"
	p.PlayedCards().AddCard(cardID, "Power Infrastructure", "active", []string{"building", "power"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 7,
		shared.ResourceCredit: 5,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			&shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceEnergy, Amount: 1, Target: "self-player"}, VariableAmount: true},
		},
		Outputs: []shared.BehaviorCondition{
			&shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCredit, Amount: 1, Target: "self-player"}, VariableAmount: true},
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Power Infrastructure",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	// Spend all 7 energy
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	selectedAmount := 7
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, &selectedAmount, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Infrastructure should succeed spending all energy")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 0, resources.Energy, "Should have 0 energy after spending all 7")
	testutil.AssertEqual(t, 12, resources.Credits, "Should have 12 credits (5 + 7)")
}

// --- Indentured Workers (195) ---
// "The next card you play this generation costs 8 M€ less."
// Auto trigger, outputs: discount 8 with temporary:"next-card".
func TestIndenturedWorkers_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Indentured Workers")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "UpdateStatus failed")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "UpdatePhase failed")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "SetCurrentTurn failed")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Indentured Workers should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Indentured Workers should be in played cards")
}

func TestRoboticWorkforce_CopiesCompleteProductionBox(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		source                         string
		credits, steel, energy, plants int
	}{
		{"simple", "Mine", 0, 1, 0, 0},
		{"decrease", "Food Factory", 4, 0, 0, -1},
		{"no tile or passive replay", "Immigrant City", -2, 0, -1, 0},
		{"recount building tags", "Medical Lab", 1, 0, 0, 0},
		{"prelude", "Martian Industries", 0, 1, 1, 0},
		{"corporation", "Mining Guild", 0, 1, 0, 0},
		{"combined tags plus wild", "Gyropolis", 2, 0, -2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, registry, id := testutil.SetupSoloGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceEnergyProduction: 3, shared.ResourcePlantProduction: 3})
			source := testutil.GetCardByName(tc.source)
			if source.Type == gamecards.CardTypeCorporation {
				p.SetCorporationID(source.ID)
			} else {
				addColonyTestPlayedCards(p, tc.source)
			}
			if tc.source == "Medical Lab" {
				addColonyTestPlayedCards(p, "Power Plant")
			}
			if tc.source == "Gyropolis" {
				addColonyTestPlayedCards(p, "Research Coordination", "Acquired Company")
			}

			workforce := testutil.GetCardByName("Robotic Workforce")
			p.Hand().AddCard(workforce.ID)
			before := p.Resources().Production()
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), id, workforce.ID, shared.NativePayment(shared.
				ResourceCredit, 9), nil, nil, nil, nil, nil), "Play copy")
			pending := p.Selection().GetPendingEffectSelection()
			if pending == nil {
				t.Fatal("Expected production source selection")
			}
			option := -1
			for i, o := range pending.Options {
				if o.CardID == source.ID {
					option = i
					break
				}
			}
			if option < 0 {
				t.Fatal("Expected source in available options")
			}
			confirm := confirmAction.NewConfirmEffectSelectionAction(repo, registry, nil, nil)
			resources := p.Resources().Get()
			testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, option), "Confirm production source")
			expected := before
			expected.Credits += tc.credits
			expected.Steel += tc.steel
			expected.Energy += tc.energy
			expected.Plants += tc.plants
			testutil.AssertEqual(t, expected, p.Resources().Production(), "Copy every production increase/decrease")
			testutil.AssertEqual(t, resources, p.Resources().Get(), "Do not copy resource gains or source cost")
			testutil.AssertFalse(t, g.HasAnyPendingSelection(id), "No copied tile placement")
			testutil.AssertErrorContains(t, confirm.Execute(context.Background(), g.ID(), id, option), "no pending effect selection", "Cannot confirm twice")
		})
	}
}

func TestRoboticWorkforce_RejectsMissingOrUnaffordableSources(t *testing.T) {
	for _, name := range []string{"", "Industrial Center", "Food Factory", "Sponsors"} {
		t.Run(name, func(t *testing.T) {
			g, repo, registry, id := testutil.SetupSoloGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			if name != "" {
				addColonyTestPlayedCards(p, name)
			}
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			card := testutil.GetCardByName("Robotic Workforce")
			p.Hand().AddCard(card.ID)
			before := p.Resources().Get()
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.
				ResourceCredit, 9), nil, nil, nil, nil, nil)
			testutil.AssertErrorContains(t, err, "no legal effect selection", "Must have a legal production box before payment")
			testutil.AssertEqual(t, before, p.Resources().Get(), "Rejected copy cannot spend")
			testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "Rejected copy stays in hand")
		})
	}
}

func TestRoboticWorkforce_CopiesResolvedMiningProduction(t *testing.T) {
	g, repo, registry, id := testutil.SetupSoloGame(t)
	p, _ := g.GetPlayer(id)
	ctx := context.Background()
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	mining := testutil.GetCardByName("Mining Rights")
	p.Hand().AddCard(mining.ID)
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, mining.ID, shared.NativePayment(shared.
		ResourceCredit, mining.Cost), nil, nil, nil, nil, nil), "Play Mining Rights")
	_, err := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), testutil.TestLogger()).Execute(ctx, g.ID(), id, "-4,3,1")
	testutil.AssertNoError(t, err, "Select steel bonus tile")
	before := p.Resources().Production()
	resources := p.Resources().Get()
	card := testutil.GetCardByName("Robotic Workforce")
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
		ResourceCredit, 9), nil, nil, nil, nil, nil), "Play copy")
	pending := p.Selection().GetPendingEffectSelection()
	if pending == nil {
		t.Fatal("No copy selection")
	}
	for i, o := range pending.Options {
		if o.CardID != mining.ID {
			continue
		}
		testutil.AssertNoError(t, confirmAction.NewConfirmEffectSelectionAction(repo, registry, nil, nil).Execute(ctx, g.ID(), id, i), "Copy mining production")
		testutil.AssertEqual(t, before.Steel+1, p.Resources().Production().Steel, "Copy resolved steel production")
		testutil.AssertEqual(t, before.Titanium, p.Resources().Production().Titanium, "Do not copy unchosen titanium production")
		testutil.AssertEqual(t, resources.Steel, p.Resources().Get().Steel, "Do not replay placement bonus")
		testutil.AssertFalse(t, g.HasAnyPendingSelection(id), "Do not replay mining tile")
		return
	}
	t.Fatal("Mining Rights missing from copy sources")
}

func TestPeekAndTake_RequiresExactSelection(t *testing.T) {
	for _, tc := range []struct {
		name       string
		peek, take int
	}{{"Business Contacts", 4, 2}, {"Invention Contest", 3, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			ctx := context.Background()
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			card := testutil.GetCardByName(tc.name)
			p.Hand().AddCard(card.ID)
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 1), "last action")
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
				ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play peek card")
			selection := p.Selection().GetPendingCardDrawSelection()
			if selection == nil {
				t.Fatal("missing selection")
			}
			testutil.AssertEqual(t, tc.peek, len(selection.AvailableCards), "peek count")
			testutil.AssertEqual(t, tc.take, selection.MinFreeTakeCount, "mandatory count")
			testutil.AssertEqual(t, 0, g.CurrentTurn().ActionsRemaining(), "card play consumes action once")
			testutil.AssertEqual(t, id, g.CurrentTurn().PlayerID(), "pending selection holds turn")
			credits, hand, discard := p.Resources().Get().Credits, p.Hand().CardCount(), len(g.Deck().DiscardPile())
			confirm := confirmAction.NewConfirmCardDrawAction(repo, registry, testutil.TestLogger())
			unknown := append(slices.Clone(selection.AvailableCards[:tc.take-1]), "not-in-selection")
			type invalidSelection struct {
				ids     []string
				wantErr string
			}
			invalid := []invalidSelection{
				{nil, "must take at least"},
				{selection.AvailableCards[:tc.take-1], "must take at least"},
				{selection.AvailableCards[:tc.take+1], "too many cards selected"},
				{unknown, "not in available cards"},
			}
			if tc.take == 2 {
				invalid = append(invalid, invalidSelection{[]string{selection.AvailableCards[0], selection.AvailableCards[0]}, "duplicate selected card"})
			}
			for _, tt := range invalid {
				testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, tt.ids, nil, shared.NativePayment(shared.ResourceCredit, 0*3)), tt.wantErr, "reject invalid selection")
				testutil.AssertEqual(t, credits, p.Resources().Get().Credits, "no charge on rejection")
				testutil.AssertEqual(t, hand, p.Hand().CardCount(), "no hand mutation")
				testutil.AssertEqual(t, discard, len(g.Deck().DiscardPile()), "no discard on rejection")
				if p.Selection().GetPendingCardDrawSelection() != selection {
					t.Fatal("rejected selection was cleared")
				}
			}
			testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), opponentID, selection.AvailableCards[:tc.take], nil, shared.NativePayment(shared.ResourceCredit, 0*3)), "no pending card draw selection", "opponent cannot confirm")
			testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), id, selection.AvailableCards[:tc.take], nil, shared.NativePayment(shared.ResourceCredit, 0*3)), "take exact count")
			testutil.AssertEqual(t, hand+tc.take, p.Hand().CardCount(), "selected cards enter hand")
			testutil.AssertEqual(t, discard+tc.peek-tc.take, len(g.Deck().DiscardPile()), "discard leftovers")
			testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "advance after confirmation")
			testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "do not consume opponent action")
			testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), id, selection.AvailableCards[:tc.take], nil, shared.NativePayment(shared.ResourceCredit, 0*3)), "no pending card draw selection", "cannot confirm twice")
		})
	}
}

func TestInteractiveEffects_TriggerOnTheirOwnTags(t *testing.T) {
	for _, name := range []string{"Mars University", "Olympus Conference", "Viral Enhancers"} {
		t.Run(name, func(t *testing.T) {
			g, repo, registry, playerID, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(playerID)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			c := testutil.GetCardByName(name)
			p.Hand().AddCard(c.ID)
			p.Hand().AddCard(testutil.CardID("Power Plant"))
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), playerID, c.ID, shared.NativePayment(shared.
				ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "Play source card")
			if name == "Mars University" {
				testutil.AssertEqual(t, 1, len(p.Selection().GetPendingBehaviorResolutions()), "optional discard remains a decision")
			} else {
				testutil.AssertEqual(t, 0, len(p.Selection().GetPendingBehaviorResolutions()), "forced self trigger resolves automatically")
				if name == "Olympus Conference" {
					testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(c.ID), "self trigger adds science once")
				} else {
					testutil.AssertEqual(t, 1, p.Resources().Get().Plants, "self trigger gains one plant")
				}
			}
		})
	}
}

func TestInteractiveEffects_ResearchResolutionsCanBeOrdered(t *testing.T) {
	for _, skipFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("skipFirst=%v", skipFirst), func(t *testing.T) {
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			ctx := context.Background()
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 10), "setup actions")
			stateRepo := game.NewInMemoryGameStateRepository()
			play := cardAction.NewPlayCardAction(repo, registry, stateRepo, testutil.TestLogger())
			choose := confirmAction.NewConfirmBehaviorChoiceAction(repo, registry, stateRepo, testutil.TestLogger())
			discard := confirmAction.NewConfirmCardDiscardAction(repo, registry, stateRepo, testutil.TestLogger())
			playName := func(name string) {
				c := testutil.GetCardByName(name)
				p.Hand().AddCard(c.ID)
				testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.
					ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "play "+name)
			}
			playName("Mars University")
			testutil.AssertNoError(t, discard.Execute(ctx, g.ID(), id, p.Selection().GetPendingBehaviorResolutions()[0].ID, nil), "skip own Mars trigger")
			playName("Olympus Conference")
			for _, r := range p.Selection().GetPendingBehaviorResolutions() {
				if p.Selection().GetPendingBehaviorResolution(r.ID) == nil {
					testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(testutil.CardID("Olympus Conference")), "sole forced science choice resolves after discard")
					continue
				}
				if r.Kind == "choice" {
					testutil.AssertNoError(t, choose.Execute(ctx, g.ID(), id, r.ID, 0, nil), "add own science")
				} else {
					testutil.AssertNoError(t, discard.Execute(ctx, g.ID(), id, r.ID, nil), "skip Mars")
				}
			}
			// Start Research without science or cards in hand, to exercise live eligibility.
			p.Resources().AddToStorage(testutil.CardID("Olympus Conference"), -1)
			g.InitDeck(nil, nil, nil)
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 1), "last action")
			playName("Research")
			testutil.AssertEqual(t, 0, p.Hand().CardCount(), "empty deck leaves hand empty")
			pending := p.Selection().GetPendingBehaviorResolutions()
			testutil.AssertEqual(t, 4, len(pending), "two science tags trigger both sources twice")
			logsBefore, err := stateRepo.GetDiff(ctx, g.ID())
			testutil.AssertNoError(t, err, "read play logs")
			var mars, olympus []string
			seen := map[string]bool{}
			for _, r := range pending {
				if r.ID == "" || seen[r.ID] {
					t.Fatal("resolution IDs must be unique")
				}
				seen[r.ID] = true
				testutil.AssertEqual(t, testutil.CardID("Research"), r.TriggeringCardID, "keep trigger context")
				if r.SourceCardID == testutil.CardID("Mars University") {
					mars = append(mars, r.ID)
				} else {
					olympus = append(olympus, r.ID)
				}
			}
			testutil.AssertEqual(t, 2, len(mars), "two Mars decisions")
			testutil.AssertEqual(t, 2, len(olympus), "two Olympus decisions")
			snapshot := dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil)
			testutil.AssertEqual(t, 4, len(snapshot.PendingBehaviorResolutions), "reconnect retains all decisions")
			for _, r := range snapshot.PendingBehaviorResolutions {
				testutil.AssertTrue(t, seen[r.ID], "snapshot preserves identity")
				if r.Kind == "choice" {
					testutil.AssertTrue(t, !r.Choices[1].Available, "cannot spend science yet")
				}
			}
			testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), id, olympus[0], 1, nil), "Not enough resources on Olympus Conference", "reject insufficient science")
			testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), opponentID, olympus[0], 0, nil), "no pending behavior choice selection", "reject wrong owner")
			testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), id, mars[0], 0, nil), "no pending behavior choice selection", "reject wrong kind")
			testutil.AssertErrorContains(t, discard.Execute(ctx, g.ID(), id, olympus[0], nil), "no pending card discard selection", "reject wrong discard kind")
			testutil.AssertErrorContains(t, discard.Execute(ctx, g.ID(), id, mars[0], []string{"absent"}), "card absent not in player's hand", "reject missing card")
			testutil.AssertEqual(t, 4, len(p.Selection().GetPendingBehaviorResolutions()), "rejections retain all decisions")
			testutil.AssertEqual(t, id, g.CurrentTurn().PlayerID(), "pending decisions hold turn")
			testutil.AssertEqual(t, 0, g.CurrentTurn().ActionsRemaining(), "only card play consumes action")
			if skipFirst {
				testutil.AssertNoError(t, discard.Execute(ctx, g.ID(), id, mars[1], nil), "skip while empty")
			}
			g.InitDeck([]string{testutil.CardID("Mine"), testutil.CardID("Power Plant"), testutil.CardID("Asteroid")}, nil, nil)
			testutil.AssertNoError(t, choose.Execute(ctx, g.ID(), id, olympus[1], 0, nil), "resolve second Olympus first")
			snapshot = dto.ToPlayerDto(p, g, registry, nil, nil, nil, nil)
			for _, r := range snapshot.PendingBehaviorResolutions {
				if r.ID == olympus[0] {
					testutil.AssertTrue(t, r.Choices[1].Available, "science becomes spendable")
				}
			}
			testutil.AssertNoError(t, choose.Execute(ctx, g.ID(), id, olympus[0], 1, nil), "spend newly added science")
			testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(testutil.CardID("Olympus Conference")), "spent once")
			drawn := p.Hand().Cards()[0]
			testutil.AssertNoError(t, discard.Execute(ctx, g.ID(), id, mars[0], []string{drawn}), "discard newly drawn card")
			testutil.AssertEqual(t, 1, p.Hand().CardCount(), "exchange one card")
			if !skipFirst {
				testutil.AssertEqual(t, id, g.CurrentTurn().PlayerID(), "last decision still holds turn")
				testutil.AssertNoError(t, discard.Execute(ctx, g.ID(), id, mars[1], []string{p.Hand().Cards()[0]}), "second exchange")
			}
			testutil.AssertEqual(t, 0, len(p.Selection().GetPendingBehaviorResolutions()), "all resolved")
			testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "advance exactly after final decision")
			testutil.AssertEqual(t, 2, g.CurrentTurn().ActionsRemaining(), "opponent keeps two actions")
			hand, deck := p.Hand().CardCount(), len(g.Deck().ProjectCards())
			testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), id, olympus[0], 1, nil), "no pending behavior choice selection", "reject replayed choice")
			testutil.AssertErrorContains(t, discard.Execute(ctx, g.ID(), id, mars[0], p.Hand().Cards()), "no pending card discard selection", "reject replayed discard")
			testutil.AssertEqual(t, hand, p.Hand().CardCount(), "replay leaves hand intact")
			testutil.AssertEqual(t, deck, len(g.Deck().ProjectCards()), "replay leaves deck intact")
			testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "replay does not advance again")
			logsAfter, err := stateRepo.GetDiff(ctx, g.ID())
			testutil.AssertNoError(t, err, "read resolution logs")
			testutil.AssertEqual(t, len(logsBefore)+4, len(logsAfter), "exactly one log per accepted resolution")
			for _, entry := range logsAfter[len(logsBefore):] {
				if entry.Source == "Olympus Conference" {
					testutil.AssertTrue(t, entry.ChoiceIndex != nil, "log includes selected choice")
					testutil.AssertEqual(t, 1, len(entry.CalculatedOutputs), "log has actual single reward")
				}
			}

		})
	}
}

func TestViralEnhancers_OnlyRewardsTheTriggeringCard(t *testing.T) {
	for _, tc := range []struct {
		name          string
		choice, count int
	}{
		{"Fish", 2, 1}, {"Tardigrades", 1, 1}, {"Ecological Zone", 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			ctx := context.Background()
			p, _ := g.GetPlayer(id)
			opponent, _ := g.GetPlayer(opponentID)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 10), "setup actions")
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			choose := confirmAction.NewConfirmBehaviorChoiceAction(repo, registry, nil, testutil.TestLogger())
			// An existing compatible card must never receive the triggering card's bonus.
			other := testutil.GetCardByName("Pets")
			p.PlayedCards().AddCard(other.ID, other.Name, string(other.Type), []string{"animal"})
			source := testutil.GetCardByName("Viral Enhancers")
			p.Hand().AddCard(source.ID)
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, source.ID, shared.NativePayment(shared.
				ResourceCredit, source.Cost), nil, nil, nil, nil, nil), "play Viral Enhancers")
			testutil.AssertEqual(t, 0, len(p.Selection().GetPendingBehaviorResolutions()), "source cannot store resources, so self trigger is forced")
			testutil.AssertEqual(t, 1, p.Resources().Get().Plants, "one plant on self play")
			if tc.name == "Fish" {
				testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, 2), "Fish requirement")
				opponent.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourcePlantProduction: 1})
			}
			if tc.name == "Ecological Zone" {
				testutil.PlaceTileForPlayer(ctx, t, g, repo, id, "greenery", testutil.FindUnoccupiedLandHex(t, g))
			}
			c := testutil.GetCardByName(tc.name)
			p.Hand().AddCard(c.ID)
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 1), "last action")
			var target *string
			if tc.name == "Fish" {
				target = &opponentID
			}
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.
				ResourceCredit, c.Cost), nil, nil, target, nil, nil), "play triggering card")
			pending := p.Selection().GetPendingBehaviorResolutions()
			testutil.AssertEqual(t, tc.count, len(pending), "one bonus per matching tag")
			initialStorage, initialPlants := p.Resources().GetCardStorage(c.ID), p.Resources().Get().Plants
			for _, r := range pending {
				testutil.AssertEqual(t, c.ID, r.TriggeringCardID, "retain exact destination")
				testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), id, r.ID, tc.choice, []string{other.ID}), "unexpected card storage targets", "cannot override fixed target")
				testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), opponentID, r.ID, tc.choice, nil), "no pending behavior choice selection", "cannot resolve another player's decision")
			}
			testutil.AssertEqual(t, initialStorage, p.Resources().GetCardStorage(c.ID), "rejections don't award")
			for i, r := range pending {
				selected := tc.choice
				if i == 1 {
					selected = 0
				} // Ecological Zone permits different rewards for its two tags.
				testutil.AssertNoError(t, choose.Execute(ctx, g.ID(), id, r.ID, selected, nil), "resolve bonus")
				testutil.AssertErrorContains(t, choose.Execute(ctx, g.ID(), id, r.ID, selected, nil), "no pending behavior choice selection", "no double award")
			}
			testutil.AssertEqual(t, initialStorage+1, p.Resources().GetCardStorage(c.ID), "exactly one resource on triggering card")
			testutil.AssertEqual(t, initialPlants+tc.count-1, p.Resources().Get().Plants, "independent plant choice")
			testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(other.ID), "existing compatible card untouched")
			testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(source.ID), "source is not destination")
			if tc.name == "Ecological Zone" {
				testutil.AssertEqual(t, id, g.CurrentTurn().PlayerID(), "tile selection also holds turn")
				selection := g.GetPendingTileSelection(id)
				testutil.AssertTrue(t, selection != nil, "ecological tile remains pending")
				tile := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger())
				_, err := tile.Execute(ctx, g.ID(), id, selection.AvailableHexes[0])
				testutil.AssertNoError(t, err, "place ecological zone")
			}
			testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "advance after all selections")
		})
	}
}

func TestProtectedHabitats_VirusProtectionAndOwnRemoval(t *testing.T) {
	for _, own := range []bool{false, true} {
		t.Run(fmt.Sprint(own), func(t *testing.T) {
			g, repo, registry, ownerID, opponentID := testutil.SetupTwoPlayerGame(t)
			ctx := context.Background()
			owner, _ := g.GetPlayer(ownerID)
			habitats := testutil.GetCardByName("Protected Habitats")
			owner.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100, shared.ResourcePlant: 8})
			owner.Hand().AddCard(habitats.ID)
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), ownerID, habitats.ID, shared.NativePayment(shared.
				ResourceCredit, habitats.Cost), nil, nil, nil, nil, nil), "play habitats")
			actorID := opponentID
			if own {
				actorID = ownerID
			}
			actor, _ := g.GetPlayer(actorID)
			virus := testutil.GetCardByName("Virus")
			actor.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
			actor.Hand().AddCard(virus.ID)
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, actorID, 2), "turn")
			choice := 1
			err := play.Execute(ctx, g.ID(), actorID, virus.ID, shared.NativePayment(shared.
				ResourceCredit, virus.Cost), &choice, nil, &ownerID, nil, nil)
			if own {
				testutil.AssertNoError(t, err, "own removal allowed")
				testutil.AssertEqual(t, 3, owner.Resources().Get().Plants, "own plants removed")
			} else {
				if err == nil {
					t.Fatal("opponent must not remove protected plants")
				}
				testutil.AssertTrue(t, actor.Hand().HasCard(virus.ID), "rejected card remains in hand")
				testutil.AssertEqual(t, 8, owner.Resources().Get().Plants, "plants protected")
			}
		})
	}
}

func TestProtectedHabitats_StorageTransfers(t *testing.T) {
	for _, tc := range []struct {
		source, action string
		own            bool
	}{
		{"Birds", "Predators", false}, {"Decomposers", "Ants", false},
		{"Arklight", "Predators", false}, {"Birds", "Predators", true}, {"Decomposers", "Ants", true},
	} {
		t.Run(fmt.Sprintf("%s/%v", tc.source, tc.own), func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, ownerID, otherID := testutil.SetupTwoPlayerGame(t)
			owner, _ := g.GetPlayer(ownerID)
			habitats := testutil.GetCardByName("Protected Habitats")
			owner.Hand().AddCard(habitats.ID)
			owner.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), ownerID, habitats.ID, shared.NativePayment(shared.
				ResourceCredit, habitats.Cost), nil, nil, nil, nil, nil), "play habitats")
			source := testutil.GetCardByName(tc.source)
			if source.Type == gamecards.CardTypeCorporation {
				owner.SetCorporationID(source.ID)
			} else {
				owner.PlayedCards().AddCard(source.ID, source.Name, string(source.Type), nil)
			}
			owner.Resources().AddToStorage(source.ID, 3)
			actorID := otherID
			if tc.own {
				actorID = ownerID
			}
			actor, _ := g.GetPlayer(actorID)
			actionCard := testutil.GetCardByName(tc.action)
			actor.PlayedCards().AddCard(actionCard.ID, actionCard.Name, string(actionCard.Type), nil)
			for i, b := range actionCard.Behaviors {
				if gamecards.HasManualTrigger(b) {
					actor.Actions().SetActions([]shared.CardAction{{CardID: actionCard.ID, CardName: actionCard.Name, BehaviorIndex: i, Behavior: b}})
					testutil.AssertNoError(t, g.SetCurrentTurn(ctx, actorID, 2), "turn")
					state := baseaction.CalculatePlayerCardActionState(actionCard.ID, b, 0, actor, g, registry)
					testutil.AssertEqual(t, tc.own, state.Available(), "availability agrees with protection")
					err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), actorID, actionCard.ID, i, nil, nil, nil, &source.ID, nil, nil, nil, nil)
					if tc.own {
						testutil.AssertNoError(t, err, "own transfer allowed")
						testutil.AssertEqual(t, 2, owner.Resources().GetCardStorage(source.ID), "source deducted")
						testutil.AssertEqual(t, 1, actor.Resources().GetCardStorage(actionCard.ID), "destination gained")
					} else {
						testutil.AssertErrorContains(t, err, "are protected", "opponent blocked")
						testutil.AssertEqual(t, 3, owner.Resources().GetCardStorage(source.ID), "source unchanged")
						testutil.AssertEqual(t, 0, actor.Actions().List()[0].TimesUsedThisGeneration, "action not consumed")
					}
				}
			}
		})
	}
}

func TestProtectedHabitats_DeferredRemovalRechecksProtection(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, actorID, ownerID := testutil.SetupTwoPlayerGame(t)
	actor, _ := g.GetPlayer(actorID)
	owner, _ := g.GetPlayer(ownerID)
	owner.Resources().Add(map[shared.ResourceType]int{shared.ResourcePlant: 5})
	output := shared.NewBasicResourceCondition(shared.ResourcePlant, -3, "any-player")
	output.TargetRestriction = &shared.TargetRestriction{}
	testutil.AssertNoError(t, gamecards.QueueResourceRemoval(g, actor, output, nil, "test", "test", registry), "queue")
	selection := actor.Selection().GetPendingResourceRemovalSelection()
	habitats := testutil.GetCardByName("Protected Habitats")
	owner.Effects().AddEffect(shared.CardEffect{CardID: habitats.ID, Behavior: habitats.Behaviors[0]})
	confirm := confirmAction.NewConfirmResourceRemovalAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertErrorContains(t, confirm.Execute(ctx, g.ID(), actorID, selection.ID, ownerID, 3), "target or amount is not eligible for resource removal", "stale protected selection")
	testutil.AssertEqual(t, 5, owner.Resources().Get().Plants, "resources retained")
	testutil.AssertTrue(t, actor.Selection().GetPendingResourceRemovalSelection() != nil, "selection retained")
	testutil.AssertNoError(t, confirm.Execute(ctx, g.ID(), actorID, selection.ID, "", 0), "skip remains allowed")
}

func TestCommercialDistrict_PlacesSpecialTile(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Commercial District")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceEnergyProduction: 1})
	p.Hand().AddCard(card.ID)
	err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
		ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "play Commercial District")
	pending := g.GetPendingTileSelection(id)
	if pending == nil {
		t.Fatal("Commercial District must place its tile")
	}
	testutil.AssertEqual(t, "commercial-district", pending.TileType, "special tile")
	testutil.AssertEqual(t, card.ID, pending.SourceCardID, "tile linked to card")
}

func TestIndustrialCenter_RequiresPlacementBeforePayment(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Industrial Center")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	before := p.Resources().Get()
	err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
		ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "No valid tile placements", "cannot place Industrial Center without a city")
	testutil.AssertEqual(t, before, p.Resources().Get(), "no payment on invalid play")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "card stays in hand")
}

// --- Commercial District (085) ---
// "Decrease your energy production 1 step and increase your M€ production 4 steps."
func TestCommercialDistrict_ProductionChange(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	commercialDistrict := testutil.GetCardByName("Commercial District")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 1,
	})
	p.Hand().AddCard(commercialDistrict.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), commercialDistrict.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Commercial District should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+4, productionAfter.Credits, "Should gain 4 credit production")
	testutil.AssertEqual(t, productionBefore.Energy-1, productionAfter.Energy, "Energy production should decrease by 1")
}

func TestIndustrialCenter_PlacementAndAction(t *testing.T) {
	for _, owner := range []string{"self", "opponent", "neutral"} {
		t.Run(owner, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			card := testutil.GetCardByName("Industrial Center")
			cityOwner := id
			if owner == "opponent" {
				cityOwner = otherID
			}
			if owner == "neutral" {
				cityOwner = ""
			}
			cityPos := shared.HexPosition{Q: 2, R: -4, S: 2}
			testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, cityPos, board.TileOccupant{Type: shared.ResourceCityTile}, cityOwner), "city")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			p.Hand().AddCard(card.ID)
			testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
				ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play")
			pending := g.GetPendingTileSelection(id)
			if pending == nil || len(pending.AvailableHexes) == 0 {
				t.Fatal("missing Industrial Center placement")
			}
			testutil.AssertEqual(t, "industrial-center", pending.TileType, "special tile")
			for _, hex := range pending.AvailableHexes {
				adjacent := false
				for _, n := range cityPos.GetNeighbors() {
					if n.String() == hex {
						adjacent = true
					}
				}
				testutil.AssertTrue(t, adjacent, "every offered hex adjoins a city")
			}
			selected := pending.AvailableHexes[0]
			_, err := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, selected)
			testutil.AssertNoError(t, err, "place")
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "turn")
			before := p.Resources().Get().Credits
			production := p.Resources().Production().Steel
			testutil.AssertNoError(t, cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), "manual action")
			testutil.AssertEqual(t, before-7, p.Resources().Get().Credits, "manual cost")
			testutil.AssertEqual(t, production+1, p.Resources().Production().Steel, "manual production")
			testutil.AssertTrue(t, g.GetPendingTileSelection(id) == nil, "action does not place another tile")
		})
	}
}

func TestCommercialDistrict_ScoresOnlyAdjacentCities(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Commercial District")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourceEnergyProduction: 1})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
		ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play")
	pending := g.GetPendingTileSelection(id)
	if pending == nil {
		t.Fatal("missing placement")
	}
	var center shared.HexPosition
	var neighbors []shared.HexPosition
	for _, hex := range pending.AvailableHexes {
		for _, tile := range g.Board().Tiles() {
			if tile.Coordinates.String() != hex {
				continue
			}
			var candidates []shared.HexPosition
			for _, n := range tile.Coordinates.GetNeighbors() {
				neighbor, err := g.Board().GetTile(n)
				if err == nil && neighbor.Type == shared.ResourceLandTile && len(neighbor.Tags) == 0 {
					candidates = append(candidates, n)
				}
			}
			if len(candidates) >= 3 {
				center = tile.Coordinates
				neighbors = candidates
				break
			}
		}
		if len(neighbors) >= 3 {
			break
		}
	}
	if len(neighbors) < 3 {
		t.Fatal("fixture needs neighboring land")
	}
	for i, owner := range []string{id, otherID, ""} {
		testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, neighbors[i], board.TileOccupant{Type: shared.ResourceCityTile}, owner), "adjacent city")
	}
	// An unrelated city elsewhere must never enter the card's score.
	for _, tile := range g.Board().Tiles() {
		if tile.Type != shared.ResourceLandTile || len(tile.Tags) > 0 || tile.OccupiedBy != nil || tile.Coordinates == center {
			continue
		}
		adjacent := false
		for _, n := range center.GetNeighbors() {
			if n == tile.Coordinates {
				adjacent = true
			}
		}
		if !adjacent {
			testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, tile.Coordinates, board.TileOccupant{Type: shared.ResourceCityTile}, otherID), "distant city")
			break
		}
	}
	score := func() int {
		return gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil).CardVP
	}
	testutil.AssertEqual(t, 0, score(), "no points before card's tile exists")
	_, err := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, center.String())
	testutil.AssertNoError(t, err, "place district")
	testutil.AssertEqual(t, 3, score(), "counts only three adjacent cities of any owner")
	placed, err := g.Board().GetTile(center)
	testutil.AssertNoError(t, err, "tile")
	testutil.AssertEqual(t, shared.ResourceCommercialDistrictTile, placed.OccupiedBy.Type, "district is not a city")
	testutil.AssertNoError(t, g.Board().ClearTileOccupant(ctx, center), "remove district")
	testutil.AssertEqual(t, 0, score(), "no phantom score after tile removal")
}

func TestIndustrialCenter_StalePlacementRejected(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	city := shared.HexPosition{Q: 2, R: -4, S: 2}
	testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, city, board.TileOccupant{Type: shared.ResourceCityTile}, otherID), "city")
	card := testutil.GetCardByName("Industrial Center")
	p.Hand().AddCard(card.ID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.
		ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play")
	pending := g.GetPendingTileSelection(id)
	if pending == nil {
		t.Fatal("missing pending placement")
	}
	testutil.AssertNoError(t, g.Board().ClearTileOccupant(ctx, city), "remove city")
	resources := p.Resources().Get()
	actions := g.CurrentTurn().ActionsRemaining()
	_, err := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, pending.AvailableHexes[0])
	testutil.AssertErrorContains(t, err, "is no longer valid for placement", "recheck adjacency")
	testutil.AssertEqual(t, resources, p.Resources().Get(), "no bonuses on invalid placement")
	testutil.AssertEqual(t, actions, g.CurrentTurn().ActionsRemaining(), "no turn advancement")
	testutil.AssertTrue(t, g.GetPendingTileSelection(id) != nil, "pending preserved")
}

func TestStorageScoringCards_RealActionsAndVP(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resource shared.ResourceType
		cost, vp int
	}{
		{"Physics Complex", shared.ResourceEnergy, 6, 2},
		{"Security Fleet", shared.ResourceTitanium, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			p.Resources().Set(shared.Resources{Credits: 100})
			c := testutil.GetCardByName(tc.name)
			p.Hand().AddCard(c.ID)
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 10), "turn")
			testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "play actual card")
			// Action DTO calculation normally registers actions before the next request.
			p.Actions().SetActions([]shared.CardAction{{CardID: c.ID, CardName: c.Name, BehaviorIndex: 0, Behavior: c.Behaviors[0]}})
			use := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger())
			p.Resources().Add(map[shared.ResourceType]int{tc.resource: tc.cost - 1})
			before, actions := p.Resources().Get(), g.CurrentTurn().ActionsRemaining()
			testutil.AssertErrorContains(t, use.Execute(ctx, g.ID(), id, c.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), "payment exceeds available source pool", "insufficient input")
			testutil.AssertEqual(t, before, p.Resources().Get(), "rejected action changes no resources")
			testutil.AssertEqual(t, actions, g.CurrentTurn().ActionsRemaining(), "rejected action is not consumed")
			p.Resources().Add(map[shared.ResourceType]int{tc.resource: 1})
			testutil.AssertNoError(t, use.Execute(ctx, g.ID(), id, c.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), "use actual action")
			testutil.AssertEqual(t, 0, p.Resources().Get().GetAmount(tc.resource), "exact input spent")
			testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(c.ID), "one resource added")
			p.Resources().Add(map[shared.ResourceType]int{tc.resource: tc.cost})
			testutil.AssertErrorContains(t, use.Execute(ctx, g.ID(), id, c.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), "action already played this generation", "cannot repeat this generation")
			testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(c.ID), "no repeat award")
			unrelated := addRegistryPlayedCard(p, "Search For Life")
			p.Resources().AddToStorage(unrelated.ID, 10)
			for _, count := range []int{0, 1, 3, 4} {
				p.Resources().AddToStorage(c.ID, count-p.Resources().GetCardStorage(c.ID))
				score := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil)
				testutil.AssertEqual(t, 3+count*tc.vp, score.CardVP, "actual card scoring excludes unrelated storage")
			}
		})
	}
}

func TestLandClaim_ReservationLifecycle(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	p.SetCorporationID("")
	p.Resources().Set(shared.Resources{Credits: 100})
	c := testutil.GetCardByName("Land Claim")
	p.Hand().AddCard(c.ID)
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 10), "turn")
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "play claim")
	selection := g.GetPendingTileSelection(id)
	if selection == nil {
		t.Fatal("missing claim selection")
	}
	var chosen shared.HexPosition
	found := false
	plantBonus := 0
	for _, tile := range g.Board().Tiles() {
		if testutil.ContainsHex(selection.AvailableHexes, tile.Coordinates.String()) {
			testutil.AssertEqual(t, shared.ResourceLandTile, tile.Type, "only land claimable")
			testutil.AssertTrue(t, tile.OccupiedBy == nil && tile.ReservedBy == nil && len(tile.Tags) == 0, "only unreserved empty areas")
			for _, bonus := range tile.Bonuses {
				if bonus.Type == shared.ResourcePlant && bonus.Amount > 0 {
					chosen = tile.Coordinates
					found = true
					plantBonus = bonus.Amount
				}
			}
		}
	}
	if !found {
		t.Fatal("need claimable plant bonus hex")
	}
	before := p.Resources().Get()
	selectTile := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger())
	_, err := selectTile.Execute(ctx, g.ID(), id, chosen.String())
	testutil.AssertNoError(t, err, "finish claim")
	tile, err := g.Board().GetTile(chosen)
	testutil.AssertNoError(t, err, "claimed tile")
	testutil.AssertTrue(t, tile.ReservedBy != nil && *tile.ReservedBy == id && tile.OccupiedBy == nil, "claim reserves but does not occupy")
	testutil.AssertEqual(t, before, p.Resources().Get(), "claim does not pay placement bonuses")
	testutil.AssertTrue(t, !testutil.ContainsHex(g.CalculateAvailableHexesForTile("city", otherID, nil), chosen.String()), "opponent excluded")
	testutil.AssertTrue(t, testutil.ContainsHex(g.CalculateAvailableHexesForTile("city", id, nil), chosen.String()), "owner can build")
	// A forged stale selection must still be rejected by the server.
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, otherID, 2), "other turn")
	testutil.AssertNoError(t, g.SetPendingTileSelection(ctx, otherID, &shared.PendingTileSelection{TileType: "city", AvailableHexes: []string{chosen.String()}}), "stale selection")
	_, err = selectTile.Execute(ctx, g.ID(), otherID, chosen.String())
	testutil.AssertErrorContains(t, err, "selected hex is occupied or reserved", "cannot build on another player's claim")
	testutil.AssertNoError(t, g.SetPendingTileSelection(ctx, otherID, nil), "clear test selection")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 10), "owner turn")
	city := testutil.GetCardByName("Research Outpost")
	p.Hand().AddCard(city.ID)
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, city.ID, shared.NativePayment(shared.ResourceCredit, city.Cost), nil, nil, nil, nil, nil), "play city on own claim")
	beforePlants := p.Resources().Get().Plants
	_, err = selectTile.Execute(ctx, g.ID(), id, chosen.String())
	testutil.AssertNoError(t, err, "build city")
	tile, err = g.Board().GetTile(chosen)
	testutil.AssertNoError(t, err, "placed tile")
	testutil.AssertTrue(t, tile.ReservedBy == nil && tile.OccupiedBy != nil, "building clears claim")
	testutil.AssertEqual(t, beforePlants+plantBonus, p.Resources().Get().Plants, "normal bonus once")
	_, err = selectTile.Execute(ctx, g.ID(), id, chosen.String())
	testutil.AssertErrorContains(t, err, "no pending tile selection", "cannot repeat selection")
}
