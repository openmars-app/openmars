package card_packs_test

import (
	"context"
	"fmt"
	baseaction "openmars/internal/action"
	"openmars/internal/delivery/dto"

	"openmars/internal/action/admin"
	cardAction "openmars/internal/action/card"
	confirmAction "openmars/internal/action/confirmation"
	resconv "openmars/internal/action/resource_conversion"
	tileAction "openmars/internal/action/tile"
	"openmars/internal/game"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
	"testing"
)

func nitriteReducingBacteriaBehavior() shared.CardBehavior {
	return shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceMicrobe, 3, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewGlobalParameterCondition(shared.ResourceTR, 1, "none"),
				},
			},
		},
	}
}

// --- Colonizer Training Camp (001) ---
// No behaviors, just VP and a max oxygen requirement.
// Test that the card can be played when oxygen is low enough.
func TestColonizerTrainingCamp_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Colonizer Training Camp")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Colonizer Training Camp should play successfully at 0% oxygen")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Colonizer Training Camp should be in played cards")
}

// --- Pets (172) ---
// "Effect: Add an animal to this card when any city is built. Add 1 animal to this card. 1 VP per 2 animals here."
func TestPets_PlaysSuccessfully(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Pets")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Pets should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID), "Pets should be in played cards")
	storage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 1, storage, "Pets should have 1 initial animal on play")
}

// --- Deep Well Heating (003) ---
// "Increase your energy production 1 step. Increase temperature 1 step."
func TestDeepWellHeating_EnergyProductionAndTemperature(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Deep Well Heating")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Deep Well Heating should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy,
		"Energy production should increase by 1")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+2, tempAfter,
		"Temperature should increase by 1 step (2 degrees)")
}

// --- Cloud Seeding (004) ---
// "Decrease your M€ production 1 step and any heat production 1 step. Increase your plant production 2 steps."
func TestCloudSeeding_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Cloud Seeding")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	target := players[1]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 3), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	// Give player some credit production so it can decrease
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceCreditProduction: 2,
	})
	// Give target heat production
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceHeatProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Cloud Seeding should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-1, prodAfter.Credits,
		"Credit production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants,
		"Plant production should increase by 2")
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, targetProdBefore.Heat-1, targetProdAfter.Heat,
		"Target heat production should decrease by 1")
}

// --- Capital (008) ---
// "Decrease your energy production 2 steps and increase your M€ production 5 steps. Place a city tile."
func TestCapital_ProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Capital")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 4), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 26)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Capital should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+5, prodAfter.Credits,
		"Credit production should increase by 5")
	testutil.AssertEqual(t, prodBefore.Energy-2, prodAfter.Energy,
		"Energy production should decrease by 2")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// --- Big Asteroid (011) ---
// "Raise temperature 2 steps and gain 4 titanium. Remove up to 4 plants from any player."
func TestBigAsteroid_TempTitaniumAndRemovePlants(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Big Asteroid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 10,
	})
	attacker.Hand().AddCard(card.ID)
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 27)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Big Asteroid should play successfully")
	attackerResources := attacker.Resources().Get()
	testutil.AssertEqual(t, 4, attackerResources.Titanium, "Attacker should gain 4 titanium")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 6, targetResources.Plants, "Target should have 6 plants (10 - 4)")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+4, tempAfter,
		"Temperature should increase by 2 steps (4 degrees)")
}

// --- Space Elevator (013) ---
// Auto behavior: "Increase your titanium production 1 step."
// Action behavior: "Spend 1 steel to gain 5 M€."
func TestSpaceElevator_TitaniumProductionOnPlay(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Space Elevator")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 27)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Space Elevator should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium,
		"Titanium production should increase by 1")
}
func TestSpaceElevator_ActionSpendSteelGainCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-space-elevator"
	p.PlayedCards().AddCard(cardID, "Space Elevator", "active", []string{"building", "space"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceSteel: 3,
	})
	creditsBefore := p.Resources().Get().Credits
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 5, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Space Elevator",
			BehaviorIndex: 1,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Space Elevator action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Steel, "Should have 2 steel after spending 1")
	testutil.AssertEqual(t, creditsBefore+5, resources.Credits, "Should gain 5 credits")
}

// --- Equatorial Magnetizer (015) ---
// "Action: Decrease your energy production 1 step to increase your terraform rating 1 step."
func TestEquatorialMagnetizer_DecreaseEnergyProdIncreaseTR(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-equatorial-magnetizer"
	p.PlayedCards().AddCard(cardID, "Equatorial Magnetizer", "active", []string{"building"})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceTR, 1, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Equatorial Magnetizer",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	trBefore := p.Resources().TerraformRating()
	prodBefore := p.Resources().Production()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Equatorial Magnetizer action should succeed")
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+1, trAfter, "TR should increase by 1")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
}

// --- Domed Crater (016) ---
// "Gain 3 plants and place a city tile. Decrease your energy production 1 step and increase M€ production 3 steps."
func TestDomedCrater_PlantsProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Domed Crater")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 24)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Domed Crater should play successfully")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+3, plantsAfter, "Should gain 3 plants")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// --- Noctis City (017) ---
// "Decrease your energy production 1 step and increase your M€ production 3 steps. Place a city tile on the reserved area."
func TestNoctisCity_ProductionAndReservedCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Noctis City")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Noctis City should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
}

// --- Methane From Titan (018) ---
// "Increase your heat production 2 steps and your plant production 2 steps."
func TestMethaneFromTitan_HeatAndPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Methane From Titan")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 2), "set oxygen")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 28)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Methane From Titan should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants,
		"Plant production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat,
		"Heat production should increase by 2")
}

// --- Phobos Space Haven (021) ---
// "Increase your titanium production 1 step and place a city tile on the reserved area."
func TestPhobosSpaceHaven_TitaniumProdAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Phobos Space Haven")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 25)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Phobos Space Haven should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium,
		"Titanium production should increase by 1")
}

// --- Black Polar Dust (022) ---
// "Place an ocean tile. Decrease your M€ production 2 steps and increase your heat production 3 steps."
func TestBlackPolarDust_ProductionAndOceanPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Black Polar Dust")
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
		shared.ResourceCreditProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 15)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Black Polar Dust should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-2, prodAfter.Credits,
		"Credit production should decrease by 2")
	testutil.AssertEqual(t, prodBefore.Heat+3, prodAfter.Heat,
		"Heat production should increase by 3")
}

// --- Arctic Algae (023) ---
// Auto: "Gain 1 plant."
// Passive: "When anyone places an ocean tile, gain 2 plants."
func TestArcticAlgae_GainPlantOnPlay(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Arctic Algae")
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
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Arctic Algae should play successfully")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+1, plantsAfter, "Should gain 1 plant on play")
}

// --- Eos Chasma National Park (026) ---
// "Add 1 animal to any animal card. Gain 3 plants. Increase your M€ production 2 steps."
func TestEosChasmaNationalPark_PlantsAndProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Eos Chasma National Park")
	animalHost := gamecards.Card{
		ID:              "card-animal-host-eos",
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
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -12), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.PlayedCards().AddCard("card-animal-host-eos", "Animal Host", "active", []string{"animal"})
	p.Resources().AddToStorage("card-animal-host-eos", 0)
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	targetCardID := "card-animal-host-eos"
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Eos Chasma National Park should play successfully")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+3, plantsAfter, "Should gain 3 plants")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits,
		"Credit production should increase by 2")
	animalStorage := p.Resources().GetCardStorage("card-animal-host-eos")
	testutil.AssertEqual(t, 1, animalStorage, "Animal host should have 1 animal")
}

// --- Cupola City (029) ---
// "Place a city tile. Decrease your energy production 1 step and increase your M€ production 3 steps."
func TestCupolaCity_ProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Cupola City")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Cupola City should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// --- Lunar Beam (030) ---
// "Decrease your M€ production 2 steps and increase your heat production and energy production 2 steps each."
func TestLunarBeam_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Lunar Beam")
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
		shared.ResourceCreditProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Lunar Beam should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-2, prodAfter.Credits,
		"Credit production should decrease by 2")
	testutil.AssertEqual(t, prodBefore.Energy+2, prodAfter.Energy,
		"Energy production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat,
		"Heat production should increase by 2")
}

// --- Underground City (032) ---
// "Place a city tile. Decrease your energy production 2 steps and increase your steel production 2 steps."
func TestUndergroundCity_ProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Underground City")
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
		shared.ResourceEnergyProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Underground City should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Steel+2, prodAfter.Steel,
		"Steel production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Energy-2, prodAfter.Energy,
		"Energy production should decrease by 2")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// --- Release Of Inert Gases (036) ---
// "Raise your terraform rating 2 steps."
func TestReleaseOfInertGases_RaiseTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Release Of Inert Gases")
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
	p.Hand().AddCard(card.ID)
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 14)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Release Of Inert Gases should play successfully")
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+2, trAfter, "TR should increase by 2")
}

// --- Deimos Down (039) ---
// "Raise temperature 3 steps and gain 4 steel. Remove up to 8 plants from any player."
func TestDeimosDown_TempSteelAndRemovePlants(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Deimos Down")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 10,
	})
	attacker.Hand().AddCard(card.ID)
	tempBefore := testGame.GlobalParameters().Temperature()
	trBefore := attacker.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 31)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Deimos Down should play successfully")
	attackerResources := attacker.Resources().Get()
	testutil.AssertEqual(t, 4, attackerResources.Steel, "Attacker should gain 4 steel")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 2, targetResources.Plants, "Target should have 2 plants (10 - 8)")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+6, tempAfter,
		"Temperature should increase by 3 steps (6 degrees)")
	trAfter := attacker.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+3, trAfter,
		"Attacker should gain +3 TR from 3 temperature steps")
}

// --- Asteroid Mining (040) ---
// "Increase your titanium production 2 steps."
func TestAsteroidMining_TitaniumProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Asteroid Mining")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 30)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Asteroid Mining should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+2, prodAfter.Titanium,
		"Titanium production should increase by 2")
}

// --- Food Factory (041) ---
// "Decrease your plant production 1 step and increase your M€ production 4 steps."
func TestFoodFactory_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Food Factory")
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
		shared.ResourcePlantProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Food Factory should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+4, prodAfter.Credits,
		"Credit production should increase by 4")
	testutil.AssertEqual(t, prodBefore.Plants-1, prodAfter.Plants,
		"Plant production should decrease by 1")
}

// --- Archaebacteria (042) ---
// "Increase your plant production 1 step."
func TestArchaebacteria_PlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Archaebacteria")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Archaebacteria should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants,
		"Plant production should increase by 1")
}

// --- Carbonate Processing (043) ---
// "Decrease your energy production 1 step and increase your heat production 3 steps."
func TestCarbonateProcessing_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Carbonate Processing")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Carbonate Processing should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Heat+3, prodAfter.Heat,
		"Heat production should increase by 3")
}

// --- Nuclear Power (045) ---
// "Decrease your M€ production 2 steps and increase your energy production 3 steps."
func TestNuclearPower_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Nuclear Power")
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
		shared.ResourceCreditProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Nuclear Power should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-2, prodAfter.Credits,
		"Credit production should decrease by 2")
	testutil.AssertEqual(t, prodBefore.Energy+3, prodAfter.Energy,
		"Energy production should increase by 3")
}

// --- Lightning Harvest (046) ---
// "Increase your energy production and your M€ production 1 step each."
func TestLightningHarvest_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Lightning Harvest")
	// Lightning Harvest requires 3 science tags
	sci1 := gamecards.Card{ID: "sci1", Name: "Sci1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	sci2 := gamecards.Card{ID: "sci2", Name: "Sci2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	sci3 := gamecards.Card{ID: "sci3", Name: "Sci3", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{sci1, sci2, sci3})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("sci1", "Sci1", "automated", []string{"science"})
	p.PlayedCards().AddCard("sci2", "Sci2", "automated", []string{"science"})
	p.PlayedCards().AddCard("sci3", "Sci3", "automated", []string{"science"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Lightning Harvest should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits,
		"Credit production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy,
		"Energy production should increase by 1")
}

// --- Algae (047) ---
// "Gain 1 plant and increase your plant production 2 steps."
func TestAlgae_PlantAndPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Algae")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 5), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Algae should play successfully")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+1, plantsAfter, "Should gain 1 plant")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants,
		"Plant production should increase by 2")
}

// --- Adapted Lichen (048) ---
// "Increase your plant production 1 step."
func TestAdaptedLichen_PlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Adapted Lichen")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Adapted Lichen should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants,
		"Plant production should increase by 1")
}

// --- Tardigrades (049) ---
// "Action: Add 1 microbe to this card."
func TestTardigrades_AddMicrobe(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-tardigrades"
	p.PlayedCards().AddCard(cardID, "Tardigrades", "active", []string{"microbe"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Tardigrades",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Tardigrades add microbe should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID),
		"Tardigrades should have 1 microbe after action")
}
func TestTardigrades_AccumulateMicrobes(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-tardigrades"
	p.PlayedCards().AddCard(cardID, "Tardigrades", "active", []string{"microbe"})
	p.Resources().AddToStorage(cardID, 3) // Start with 3 microbes
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "self-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Tardigrades",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Tardigrades add microbe should succeed")
	testutil.AssertEqual(t, 4, p.Resources().GetCardStorage(cardID),
		"Tardigrades should have 4 microbes (3 + 1)")
}

// --- Fish (052) ---
// Auto: "Decrease any plant production 1 step."
// Action: "Add 1 animal to this card."
func TestFish_DecreasePlantProductionOnPlay(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Fish")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, 2), "set temperature")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourcePlantProduction: 3,
	})
	attacker.Hand().AddCard(card.ID)
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Fish should play successfully")
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, targetProdBefore.Plants-1, targetProdAfter.Plants,
		"Target plant production should decrease by 1")
}
func TestFish_ActionAddAnimal(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-fish"
	p.PlayedCards().AddCard(cardID, "Fish", "active", []string{"animal"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Fish",
			BehaviorIndex: 1,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Fish add animal action should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID),
		"Fish should have 1 animal after action")
}

// --- Comet (010) ---
// "Raise temperature 1 step and place an ocean tile. Remove up to 3 plants from any player."
func TestComet_TempOceanAndRemovePlants(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Comet")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 5,
	})
	attacker.Hand().AddCard(card.ID)
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Comet should play successfully")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+2, tempAfter,
		"Temperature should increase by 1 step (2 degrees)")
	targetResources := target.Resources().Get()
	testutil.AssertEqual(t, 2, targetResources.Plants, "Target should have 2 plants (5 - 3)")
}

// --- Asteroid (009) ---
// "Raise temperature 1 step and gain 2 titanium. Remove up to 3 plants from any player."
// (Testing the self-player titanium gain specifically)
func TestAsteroid_SelfPlayerGainTitanium(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Asteroid")
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
	p.Hand().AddCard(card.ID)
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 14)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Asteroid should play successfully in solo mode")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Titanium, "Should gain 2 titanium")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+2, tempAfter,
		"Temperature should increase by 1 step (2 degrees)")
}

// --- Interstellar Colony Ship (027) ---
// No behaviors - just 4 VP. Requires 5 science tags.
func TestInterstellarColonyShip_RequiresScienteTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Interstellar Colony Ship")
	// Register science helper cards in the registry so tag counting works
	additionalCards := []gamecards.Card{}
	for i := 0; i < 5; i++ {
		scienceCard := gamecards.Card{
			ID:        "card-science-" + string(rune('a'+i)),
			Name:      "Science Card",
			Type:      gamecards.CardTypeAutomated,
			GameModes: []string{"classic"},
			Cost:      0,
			Tags:      []shared.CardTag{shared.TagScience},
		}
		additionalCards = append(additionalCards, scienceCard)
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	// Without 5 science tags, should fail
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 24)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 5 additional tags", "Should fail without 5 science tags")
	// Add 5 science-tagged played cards
	for i := 0; i < 5; i++ {
		sciID := "card-science-" + string(rune('a'+i))
		p.PlayedCards().AddCard(sciID, "Science Card", "automated", []string{"science"})
	}
	// Re-add card to hand since failed play should not remove it
	if !p.Hand().HasCard(card.ID) {
		p.Hand().AddCard(card.ID)
	}
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Should succeed with 5 science tags")
}

// --- Comet (010) Solo Mode ---
// Verify that in solo mode, the any-player plant removal is skipped.
func TestComet_SoloMode_PlantRemovalSkipped(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Comet")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourcePlant:  10,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Comet should play successfully in solo mode")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 10, resources.Plants,
		"Player's plants should be unchanged in solo mode (any-player removal skipped)")
}

// --- Deimos Down (039) Solo Mode ---
// In solo mode, plant removal from any-player is skipped but steel gain and temperature work.
func TestDeimosDown_SoloMode_SteelAndTemp(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Deimos Down")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourcePlant:  10,
	})
	p.Hand().AddCard(card.ID)
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 31)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Deimos Down should play in solo mode")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 4, resources.Steel, "Should gain 4 steel")
	testutil.AssertEqual(t, 10, resources.Plants,
		"Plants should be unchanged in solo mode (any-player removal skipped)")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+6, tempAfter,
		"Temperature should increase by 3 steps (6 degrees)")
}

// --- Methane From Titan (018) - with oxygen requirement ---
// This card requires 2% oxygen. Test that it fails when oxygen is too low.
func TestMethaneFromTitan_FailsWithoutOxygenRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Methane From Titan")
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
	p.Hand().AddCard(card.ID)
	// Oxygen starts at 0%, requirement is min 2%
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 28)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "oxygen requirement not met", "Methane From Titan should fail at 0% oxygen (requires 2%)")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// --- Food Factory (041) - fails without plant production ---
// "Decrease your plant production 1 step" - should fail if player has 0 plant production.
func TestFoodFactory_FailsWithoutPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Food Factory")
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
	// No plant production - should fail
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient plant-production", "Food Factory should fail without plant production")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// --- Nuclear Power (045) - fails without enough credit production ---
// "Decrease your M€ production 2 steps" - should fail if player has < 2 credit production (below -5 floor).
func TestNuclearPower_FailsWithInsufficientCreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Nuclear Power")
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
	// Set credit production to -4 (near -5 floor). Decreasing by 2 would go to -6, below floor.
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceCreditProduction: -4,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient credit-production",
		"Nuclear Power should fail when credit production would go below -5")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// --- Carbonate Processing (043) - fails without energy production ---
// "Decrease your energy production 1 step" - should fail if player has 0 energy production.
func TestCarbonateProcessing_FailsWithoutEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Carbonate Processing")
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
	// No energy production
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient energy-production", "Carbonate Processing should fail without energy production")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// --- Colonizer Training Camp (001) - max oxygen requirement failure ---
// Should fail when oxygen is above 5%.
func TestColonizerTrainingCamp_FailsAboveMaxOxygen(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Colonizer Training Camp")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Increase oxygen above 5%
	for i := 0; i < 6; i++ {
		if _, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 1, ""); err != nil {
			t.Fatalf("Failed to increase oxygen: %v", err)
		}
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "oxygen requirement not met", "Colonizer Training Camp should fail when oxygen is above 5%")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// tagP returns a pointer to a CardTag (unique name to avoid redefinition conflicts).
// iPtr returns a pointer to an int (unique name to avoid redefinition conflicts).
// --- Lake Marineris (053) ---
// "Place 2 ocean tiles. Requires 0°C or warmer."
func TestLakeMarineris_PlacesTwoOceanTiles(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	lakeMarineris := testutil.GetCardByName("Lake Marineris")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Meet temperature requirement
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, 0), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(lakeMarineris.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), lakeMarineris.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Lake Marineris should play successfully")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending tile selection for ocean placement")
	testutil.AssertEqual(t, "ocean", selection.TileType, "Pending tile type should be ocean")
}
func TestLakeMarineris_FailsWithoutTemperatureRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	lakeMarineris := testutil.GetCardByName("Lake Marineris")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Temperature below requirement (default -30, need 0)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(lakeMarineris.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), lakeMarineris.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "temperature requirement not met", "Lake Marineris should fail without meeting temperature requirement")
	testutil.AssertTrue(t, p.Hand().HasCard(lakeMarineris.ID), "rejected card stays in hand")
}

// --- Small Animals (054) ---
// Active card: "Action: Add 1 animal to this card. Decrease any plant production 1 step. Requires 6% oxygen."
func TestSmallAnimals_AddAnimalAction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-small-animals"
	p.PlayedCards().AddCard(cardID, "Small Animals", "active", []string{"animal"})
	p.Resources().AddToStorage(cardID, 0)
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Small Animals",
			BehaviorIndex: 1,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, nil, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Small Animals add animal action should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(cardID), "Card should have 1 animal after action")
}

// --- Kelp Farming (055) ---
// "Increase your M€ production 2 steps and your plant production 3 steps. Gain 2 plants. Requires 6 oceans."
func TestKelpFarming_ProductionAndPlantGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	kelpFarming := testutil.GetCardByName("Kelp Farming")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 6), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(kelpFarming.ID)
	plantsBefore := p.Resources().Get().Plants
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 17)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), kelpFarming.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Kelp Farming should play successfully")
	plantsAfter := p.Resources().Get().Plants
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	testutil.AssertEqual(t, productionBefore.Credits+2, productionAfter.Credits, "Should gain 2 credit production")
	testutil.AssertEqual(t, productionBefore.Plants+3, productionAfter.Plants, "Should gain 3 plant production")
}

// --- Mine (056) ---
// "Increase your steel production 1 step."
func TestMine_SteelProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	mine := testutil.GetCardByName("Mine")
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
	p.Hand().AddCard(mine.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), mine.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mine should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Steel+1, productionAfter.Steel, "Should gain 1 steel production")
}

// --- Vesta Shipyard (057) ---
// "Increase your titanium production 1 step."
func TestVestaShipyard_TitaniumProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	vestaShipyard := testutil.GetCardByName("Vesta Shipyard")
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
	p.Hand().AddCard(vestaShipyard.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 15)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), vestaShipyard.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Vesta Shipyard should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Titanium+1, productionAfter.Titanium, "Should gain 1 titanium production")
}

// --- Beam From A Thorium Asteroid (058) ---
// "Increase your heat production and energy production 3 steps each. Requires jovian tag."
func TestBeamFromAThoriumAsteroid_HeatAndEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	beamFromAsteroid := testutil.GetCardByName("Beam From A Thorium Asteroid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	prerequisite := testutil.GetCardByName("Vesta Shipyard")
	p.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(beamFromAsteroid.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 32)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), beamFromAsteroid.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Beam From A Thorium Asteroid should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Heat+3, productionAfter.Heat, "Should gain 3 heat production")
	testutil.AssertEqual(t, productionBefore.Energy+3, productionAfter.Energy, "Should gain 3 energy production")
}

// --- Trees (060) ---
// "Requires -4°C or warmer. Increase your plant production 3 steps. Gain 1 plant."
func TestTrees_PlantProductionAndGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	trees := testutil.GetCardByName("Trees")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -4), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(trees.ID)
	plantsBefore := p.Resources().Get().Plants
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), trees.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Trees should play successfully")
	plantsAfter := p.Resources().Get().Plants
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+1, plantsAfter, "Should gain 1 plant")
	testutil.AssertEqual(t, productionBefore.Plants+3, productionAfter.Plants, "Should gain 3 plant production")
}
func TestTrees_FailsWithoutTemperatureRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	trees := testutil.GetCardByName("Trees")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Temperature below requirement (default -30, need -4)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(trees.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), trees.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "temperature requirement not met", "Trees should fail without meeting temperature requirement")
	testutil.AssertTrue(t, p.Hand().HasCard(trees.ID), "rejected card stays in hand")
}

// --- Mineral Deposit (062) ---
// "Gain 5 steel."
func TestMineralDeposit_GainSteel(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	mineralDeposit := testutil.GetCardByName("Mineral Deposit")
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
	p.Hand().AddCard(mineralDeposit.ID)
	steelBefore := p.Resources().Get().Steel
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), mineralDeposit.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mineral Deposit should play successfully")
	steelAfter := p.Resources().Get().Steel
	testutil.AssertEqual(t, steelBefore+5, steelAfter, "Should gain 5 steel")
}

// --- Mining Expedition (063) ---
// "Raise oxygen 1 step. Remove 2 plants from any player. Gain 2 steel."
func TestMiningExpedition_OxygenAndSteelAndRemovePlants(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	miningExpedition := testutil.GetCardByName("Mining Expedition")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(miningExpedition.ID)
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 5,
	})
	steelBefore := attacker.Resources().Get().Steel
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), miningExpedition.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Mining Expedition should play successfully")
	steelAfter := attacker.Resources().Get().Steel
	testutil.AssertEqual(t, steelBefore+2, steelAfter, "Should gain 2 steel")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+1, oxygenAfter, "Oxygen should increase by 1")
	targetPlants := target.Resources().Get().Plants
	testutil.AssertEqual(t, 3, targetPlants, "Target should have 3 plants after removing 2")
}

// --- Building Industries (065) ---
// "Decrease your energy production 1 step and increase your steel production 2 steps."
func TestBuildingIndustries_EnergyToSteelProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	buildingIndustries := testutil.GetCardByName("Building Industries")
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
	p.Hand().AddCard(buildingIndustries.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), buildingIndustries.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Building Industries should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Energy-1, productionAfter.Energy, "Energy production should decrease by 1")
	testutil.AssertEqual(t, productionBefore.Steel+2, productionAfter.Steel, "Steel production should increase by 2")
}

// --- Sponsors (068) ---
// "Increase your M€ production 2 steps."
func TestSponsors_CreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	sponsors := testutil.GetCardByName("Sponsors")
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
	p.Hand().AddCard(sponsors.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), sponsors.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Sponsors should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+2, productionAfter.Credits, "Should gain 2 credit production")
}

// --- Towing A Comet (075) ---
// "Gain 2 plants. Raise oxygen level 1 step and place an ocean tile."
func TestTowingAComet_PlantsOxygenOcean(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	towingAComet := testutil.GetCardByName("Towing A Comet")
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
	p.Hand().AddCard(towingAComet.ID)
	plantsBefore := p.Resources().Get().Plants
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 23)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), towingAComet.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Towing A Comet should play successfully")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+1, oxygenAfter, "Oxygen should increase by 1")
}

// --- Space Mirrors (076) ---
// Active card: "Action: Spend 7 M€ to increase your energy production 1 step."
func TestSpaceMirrors_SpendCreditsForEnergyProduction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-space-mirrors"
	p.PlayedCards().AddCard(cardID, "Space Mirrors", "active", []string{"power", "space"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 7, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceEnergyProduction, 1, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Space Mirrors",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	productionBefore := p.Resources().Production()
	creditsBefore := p.Resources().Get().Credits
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Space Mirrors action should succeed")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Energy+1, productionAfter.Energy, "Energy production should increase by 1")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-7, creditsAfter, "Should spend 7 credits")
}

// --- Solar Wind Power (077) ---
// "Increase your energy production 1 step and gain 2 titanium."
func TestSolarWindPower_EnergyProductionAndTitanium(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	solarWindPower := testutil.GetCardByName("Solar Wind Power")
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
	p.Hand().AddCard(solarWindPower.ID)
	titaniumBefore := p.Resources().Get().Titanium
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), solarWindPower.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Solar Wind Power should play successfully")
	titaniumAfter := p.Resources().Get().Titanium
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, titaniumBefore+2, titaniumAfter, "Should gain 2 titanium")
	testutil.AssertEqual(t, productionBefore.Energy+1, productionAfter.Energy, "Should gain 1 energy production")
}

// --- Ice Asteroid (078) ---
// "Place 2 ocean tiles."
func TestIceAsteroid_PlaceTwoOceanTiles(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	iceAsteroid := testutil.GetCardByName("Ice Asteroid")
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
	p.Hand().AddCard(iceAsteroid.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 23)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), iceAsteroid.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ice Asteroid should play successfully")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending tile selection for ocean placement")
	testutil.AssertEqual(t, "ocean", selection.TileType, "Pending tile type should be ocean")
}

// --- Callisto Penal Mines (082) ---
// "Increase your M€ production 3 steps."
func TestCallistoPenalMines_CreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	callistoPenalMines := testutil.GetCardByName("Callisto Penal Mines")
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
	p.Hand().AddCard(callistoPenalMines.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 24)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), callistoPenalMines.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Callisto Penal Mines should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+3, productionAfter.Credits, "Should gain 3 credit production")
}

// --- Giant Space Mirror (083) ---
// "Increase your energy production 3 steps."
func TestGiantSpaceMirror_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	giantSpaceMirror := testutil.GetCardByName("Giant Space Mirror")
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
	p.Hand().AddCard(giantSpaceMirror.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 17)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), giantSpaceMirror.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Giant Space Mirror should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Energy+3, productionAfter.Energy, "Should gain 3 energy production")
}

// --- Grass (087) ---
// "Requires -16°C or warmer. Increase your plant production 1 step. Gain 3 plants."
func TestGrass_PlantProductionAndGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	grass := testutil.GetCardByName("Grass")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -16), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(grass.ID)
	plantsBefore := p.Resources().Get().Plants
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), grass.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Grass should play successfully")
	plantsAfter := p.Resources().Get().Plants
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+3, plantsAfter, "Should gain 3 plants")
	testutil.AssertEqual(t, productionBefore.Plants+1, productionAfter.Plants, "Should gain 1 plant production")
}

// --- Heather (088) ---
// "Requires -14°C or warmer. Increase your plant production 1 step. Gain 1 plant."
func TestHeather_PlantProductionAndGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	heather := testutil.GetCardByName("Heather")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -14), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(heather.ID)
	plantsBefore := p.Resources().Get().Plants
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), heather.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Heather should play successfully")
	plantsAfter := p.Resources().Get().Plants
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+1, plantsAfter, "Should gain 1 plant")
	testutil.AssertEqual(t, productionBefore.Plants+1, productionAfter.Plants, "Should gain 1 plant production")
}
func TestHeather_FailsWithoutTemperature(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	heather := testutil.GetCardByName("Heather")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Default temperature is -30, need -14
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(heather.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), heather.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "temperature requirement not met", "Heather should fail without meeting temperature requirement")
	testutil.AssertTrue(t, p.Hand().HasCard(heather.ID), "rejected card stays in hand")
}

// --- Peroxide Power (089) ---
// "Decrease your M€ production 1 step and increase your energy production 2 steps."
func TestPeroxidePower_ProductionChange(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	peroxidePower := testutil.GetCardByName("Peroxide Power")
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
		shared.ResourceCreditProduction: 1,
	})
	p.Hand().AddCard(peroxidePower.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), peroxidePower.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Peroxide Power should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits-1, productionAfter.Credits, "Credit production should decrease by 1")
	testutil.AssertEqual(t, productionBefore.Energy+2, productionAfter.Energy, "Energy production should increase by 2")
}

// --- Research (090) ---
// "Counts as playing 2 science cards. Draw 2 cards."
func TestResearch_DrawTwoCards(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	research := testutil.GetCardByName("Research")
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
	p.Hand().AddCard(research.ID)
	handBefore := p.Hand().CardCount()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), research.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Research should play successfully")
	// Played 1 card (-1), drew 2 cards (+2) = net +1
	handAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handBefore+1, handAfter, "Hand should increase by 1 (played 1, drew 2)")
}

// --- Gene Repair (091) ---
// "Requires 3 science tags. Increase your M€ production 2 steps."
func TestGeneRepair_CreditProductionWithScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	geneRepair := testutil.GetCardByName("Gene Repair")
	sciCard1 := gamecards.Card{ID: "card-sci-1", Name: "Science Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	sciCard2 := gamecards.Card{ID: "card-sci-2", Name: "Science Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{sciCard1, sciCard2})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("card-sci-1", "Science Card 1", "automated", []string{"science"})
	p.PlayedCards().AddCard("card-sci-2", "Science Card 2", "automated", []string{"science"})
	prerequisite := testutil.GetCardByName("Search For Life")
	p.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(geneRepair.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), geneRepair.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Gene Repair should play successfully with 3 science tags")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+2, productionAfter.Credits, "Should gain 2 credit production")
}
func TestGeneRepair_FailsWithoutScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	geneRepair := testutil.GetCardByName("Gene Repair")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Only 1 science tag (from gene repair itself) -- not enough
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(geneRepair.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), geneRepair.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 3 additional tags", "Gene Repair should fail without 3 science tags")
	testutil.AssertTrue(t, p.Hand().HasCard(geneRepair.ID), "rejected card stays in hand")
}

// --- Io Mining Industries (092) ---
// "Increase your titanium production 2 steps and your M€ production 2 steps."
func TestIoMiningIndustries_TitaniumAndCreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	ioMining := testutil.GetCardByName("Io Mining Industries")
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
	p.Hand().AddCard(ioMining.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 41)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), ioMining.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Io Mining Industries should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+2, productionAfter.Credits, "Should gain 2 credit production")
	testutil.AssertEqual(t, productionBefore.Titanium+2, productionAfter.Titanium, "Should gain 2 titanium production")
}

// --- Bushes (093) ---
// "Requires -10°C or warmer. Increase your plant production 2 steps. Gain 2 plants."
func TestBushes_PlantProductionAndGain(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	bushes := testutil.GetCardByName("Bushes")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -10), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(bushes.ID)
	plantsBefore := p.Resources().Get().Plants
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), bushes.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Bushes should play successfully")
	plantsAfter := p.Resources().Get().Plants
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	testutil.AssertEqual(t, productionBefore.Plants+2, productionAfter.Plants, "Should gain 2 plant production")
}

// --- Tropical Resort (098) ---
// "Decrease your heat production 2 steps and increase your M€ production 3 steps."
func TestTropicalResort_ProductionChange(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	tropicalResort := testutil.GetCardByName("Tropical Resort")
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
		shared.ResourceHeatProduction: 2,
	})
	p.Hand().AddCard(tropicalResort.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), tropicalResort.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Tropical Resort should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+3, productionAfter.Credits, "Should gain 3 credit production")
	testutil.AssertEqual(t, productionBefore.Heat-2, productionAfter.Heat, "Heat production should decrease by 2")
}

// --- Toll Station (099) ---
// "Increase your M€ production 1 step for each space tag your opponents have."
func TestTollStation_CreditProductionPerOpponentSpaceTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	tollStation := testutil.GetCardByName("Toll Station")
	spaceCard1 := gamecards.Card{ID: "card-space-1", Name: "Space Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagSpace}}
	spaceCard2 := gamecards.Card{ID: "card-space-2", Name: "Space Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagSpace}}
	spaceCard3 := gamecards.Card{ID: "card-space-3", Name: "Space Card 3", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagSpace}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{spaceCard1, spaceCard2, spaceCard3})
	players := testGame.GetAllPlayers()
	attacker := players[0]
	opponent := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	opponent.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	wild := testutil.GetCardByName("Research Coordination")
	opponent.PlayedCards().AddCard(wild.ID, wild.Name, string(wild.Type), nil)

	// Opponent has 3 space tags
	opponent.PlayedCards().AddCard("card-space-1", "Space Card 1", "automated", []string{"space"})
	opponent.PlayedCards().AddCard("card-space-2", "Space Card 2", "automated", []string{"space"})
	opponent.PlayedCards().AddCard("card-space-3", "Space Card 3", "automated", []string{"space"})
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(tollStation.ID)
	productionBefore := attacker.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), tollStation.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Toll Station should play successfully")
	productionAfter := attacker.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits+3, productionAfter.Credits,
		"Should gain 3 credit production (1 per each of 3 opponent space tags)")
}

// strPtrB2 returns a pointer to a string (unique name to avoid redefinition conflicts).
// --- Fueled Generators (100) ---
// "Decrease your M€ production 1 step and increase your energy production 1 step."
func TestFueledGenerators_ProductionChange(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	fueledGenerators := testutil.GetCardByName("Fueled Generators")
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
		shared.ResourceCreditProduction: 1,
	})
	p.Hand().AddCard(fueledGenerators.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), fueledGenerators.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Fueled Generators should play successfully")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Credits-1, productionAfter.Credits, "Credit production should decrease by 1")
	testutil.AssertEqual(t, productionBefore.Energy+1, productionAfter.Energy, "Energy production should increase by 1")
}

// --- Ironworks (101) ---
// Active card: "Action: Spend 4 energy to gain 1 steel and increase oxygen 1 step."
func TestIronworks_SpendEnergyForSteelAndOxygen(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-ironworks"
	p.PlayedCards().AddCard(cardID, "Ironworks", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 6,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player"),
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Ironworks",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	steelBefore := p.Resources().Get().Steel
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ironworks action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Energy, "Should have 2 energy after spending 4")
	testutil.AssertEqual(t, steelBefore+1, resources.Steel, "Should gain 1 steel")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+1, oxygenAfter, "Oxygen should increase by 1")
}
func TestIronworks_FailsWithoutEnoughEnergy(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-ironworks"
	p.PlayedCards().AddCard(cardID, "Ironworks", "active", []string{"building"})
	// Only 2 energy, need 4
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 2,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceSteel, 1, "self-player"),
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Ironworks",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	before := p.Resources().Get()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Ironworks should fail without enough energy")
	testutil.AssertEqual(t, before, p.Resources().Get(), "rejected action spends nothing")
}

// --- Power Grid (102) ---
// "Increase your energy production 1 step for each power tag you have, including this."
func TestPowerGrid_EnergyProductionPerPowerTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	powerGrid := testutil.GetCardByName("Power Grid")
	powerCard1 := gamecards.Card{ID: "card-power-1", Name: "Power Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagPower}}
	powerCard2 := gamecards.Card{ID: "card-power-2", Name: "Power Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagPower}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{powerCard1, powerCard2})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Give player 2 existing power tags
	p.PlayedCards().AddCard("card-power-1", "Power Card 1", "automated", []string{"power"})
	p.PlayedCards().AddCard("card-power-2", "Power Card 2", "automated", []string{"power"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(powerGrid.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), powerGrid.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Grid should play successfully")
	productionAfter := p.Resources().Production()
	// 2 existing power tags + 1 from Power Grid itself = 3
	testutil.AssertEqual(t, productionBefore.Energy+3, productionAfter.Energy,
		"Should gain 3 energy production (1 per each of 2 existing + 1 self power tag)")
}
func TestPowerGrid_NoPreviousPowerTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	powerGrid := testutil.GetCardByName("Power Grid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// No power tags except from Power Grid itself
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(powerGrid.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), powerGrid.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Grid should play successfully with no prior power tags")
	productionAfter := p.Resources().Production()
	// Only Power Grid itself = 1 power tag
	testutil.AssertEqual(t, productionBefore.Energy+1, productionAfter.Energy,
		"Should gain 1 energy production (only self power tag)")
}

// --- Steelworks (103) ---
// Active card: "Action: Spend 4 energy to gain 2 steel and increase oxygen 1 step."
func TestSteelworks_SpendEnergyForSteelAndOxygen(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-steelworks"
	p.PlayedCards().AddCard(cardID, "Steelworks", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 8,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceSteel, 2, "self-player"),
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Steelworks",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	steelBefore := p.Resources().Get().Steel
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Steelworks action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 4, resources.Energy, "Should have 4 energy after spending 4")
	testutil.AssertEqual(t, steelBefore+2, resources.Steel, "Should gain 2 steel")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+1, oxygenAfter, "Oxygen should increase by 1")
}

// --- Ore Processor (104) ---
// Active card: "Action: Spend 4 energy to gain 1 titanium and increase oxygen 1 step."
func TestOreProcessor_SpendEnergyForTitaniumAndOxygen(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-ore-processor"
	p.PlayedCards().AddCard(cardID, "Ore Processor", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 5,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceTitanium, 1, "self-player"),
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Ore Processor",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	titaniumBefore := p.Resources().Get().Titanium
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Ore Processor action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 1, resources.Energy, "Should have 1 energy after spending 4")
	testutil.AssertEqual(t, titaniumBefore+1, resources.Titanium, "Should gain 1 titanium")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+1, oxygenAfter, "Oxygen should increase by 1")
}
func TestOreProcessor_FailsWithoutEnoughEnergy(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-ore-processor"
	p.PlayedCards().AddCard(cardID, "Ore Processor", "active", []string{"building"})
	// Only 3 energy, need 4
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 3,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 4, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceTitanium, 1, "self-player"),
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Ore Processor",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	before := p.Resources().Get()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Ore Processor should fail without enough energy")
	testutil.AssertEqual(t, before, p.Resources().Get(), "rejected action spends nothing")
}

// --- Mass Converter (094) ---
// "Requires 5 science tags. Increase your energy production 6 steps."
func TestMassConverter_EnergyProductionWithScienceRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	massConverter := testutil.GetCardByName("Mass Converter")
	sciCard1 := gamecards.Card{ID: "card-sci-1", Name: "Science Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	sciCard2 := gamecards.Card{ID: "card-sci-2", Name: "Science Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	sciCard3 := gamecards.Card{ID: "card-sci-3", Name: "Science Card 3", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	sciCard4 := gamecards.Card{ID: "card-sci-4", Name: "Science Card 4", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{sciCard1, sciCard2, sciCard3, sciCard4})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("card-sci-1", "Science Card 1", "automated", []string{"science"})
	p.PlayedCards().AddCard("card-sci-2", "Science Card 2", "automated", []string{"science"})
	p.PlayedCards().AddCard("card-sci-3", "Science Card 3", "automated", []string{"science"})
	p.PlayedCards().AddCard("card-sci-4", "Science Card 4", "automated", []string{"science"})
	prerequisite := testutil.GetCardByName("Search For Life")
	p.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(massConverter.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), massConverter.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mass Converter should play successfully with 5 science tags")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Energy+6, productionAfter.Energy, "Should gain 6 energy production")
}
func TestMassConverter_FailsWithoutScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	massConverter := testutil.GetCardByName("Mass Converter")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Only 1 science tag from Mass Converter itself, need 5
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(massConverter.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), massConverter.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 5 additional tags", "Mass Converter should fail without 5 science tags")
	testutil.AssertTrue(t, p.Hand().HasCard(massConverter.ID), "rejected card stays in hand")
}

// --- Quantum Extractor (079) ---
// "Requires 4 science tags. Increase your energy production 4 steps."
func TestQuantumExtractor_EnergyProductionWithScienceRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	quantumExtractor := testutil.GetCardByName("Quantum Extractor")
	sciCard1 := gamecards.Card{ID: "card-sci-1", Name: "Science Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	sciCard2 := gamecards.Card{ID: "card-sci-2", Name: "Science Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	sciCard3 := gamecards.Card{ID: "card-sci-3", Name: "Science Card 3", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagScience}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{sciCard1, sciCard2, sciCard3})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("card-sci-1", "Science Card 1", "automated", []string{"science"})
	p.PlayedCards().AddCard("card-sci-2", "Science Card 2", "automated", []string{"science"})
	p.PlayedCards().AddCard("card-sci-3", "Science Card 3", "automated", []string{"science"})
	prerequisite := testutil.GetCardByName("Search For Life")
	p.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(quantumExtractor.ID)
	productionBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), quantumExtractor.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Quantum Extractor should play successfully with 4 science tags")
	productionAfter := p.Resources().Production()
	testutil.AssertEqual(t, productionBefore.Energy+4, productionAfter.Energy, "Should gain 4 energy production")
}

// --- Giant Ice Asteroid (080) ---
// "Raise temperature 2 steps and place 2 ocean tiles. Remove up to 6 plants from any player."
func TestGiantIceAsteroid_TemperatureAndOceans(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	giantIceAsteroid := testutil.GetCardByName("Giant Ice Asteroid")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(giantIceAsteroid.ID)
	target.Resources().Add(map[shared.ResourceType]int{
		shared.ResourcePlant: 10,
	})
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 36)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), giantIceAsteroid.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Giant Ice Asteroid should play successfully")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+4, tempAfter, "Temperature should increase by 2 steps (4 degrees)")
	targetPlants := target.Resources().Get().Plants
	testutil.AssertEqual(t, 4, targetPlants, "Target should have 4 plants after removing 6")
}

// --- Trans-Neptune Probe (084) ---
// No behaviors -- just tags (science, space). Tests that a card with no behaviors can be played.
func TestTransNeptuneProbe_PlaysWithNoBehaviors(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	transNeptuneProbe := testutil.GetCardByName("Trans-Neptune Probe")
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
	p.Hand().AddCard(transNeptuneProbe.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), transNeptuneProbe.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Trans-Neptune Probe should play successfully with no behaviors")
	testutil.AssertFalse(t, p.Hand().HasCard(transNeptuneProbe.ID), "Card should be removed from hand")
}

// =============================================================================
// Card 106: Acquired Company
// "Increase your M€ production 3 steps."
// =============================================================================
func TestAcquiredCompany_CreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Acquired Company")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Acquired Company should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3")
}

// =============================================================================
// Card 107: Media Archives
// "Gain 1 M€ for each event ever played by all players."
// =============================================================================
func TestMediaArchives_GainCreditsPerEventTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Media Archives")
	eventCard1 := gamecards.Card{ID: "event1", Name: "Sabotage", Type: gamecards.CardTypeEvent, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEvent}}
	eventCard2 := gamecards.Card{ID: "event2", Name: "Virus", Type: gamecards.CardTypeEvent, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEvent}}
	eventCard3 := gamecards.Card{ID: "event3", Name: "Asteroid", Type: gamecards.CardTypeEvent, GameModes: []string{"classic"}, Cost: 1, Tags: []shared.CardTag{shared.TagEvent}}
	additionalCards := []gamecards.Card{eventCard1, eventCard2, eventCard3}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	other := players[1]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.PlayedCards().AddCard("event1", "Sabotage", "event", []string{"event"})
	p.PlayedCards().AddCard("event2", "Virus", "event", []string{"event"})
	other.PlayedCards().AddCard("event3", "Asteroid", "event", []string{"event"})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Media Archives should play successfully")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-8+3, creditsAfter,
		"Should gain 3 credits (1 per event tag, 3 events total from all players) minus 8 cost")
}

// =============================================================================
// Card 108: Open City
// "Gain 2 plants. Increase your M€ production 4 steps and decrease energy production 1 step. Place a city tile."
// Requires 12% oxygen.
// =============================================================================
func TestOpenCity_PlantsProductionAndCity(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Open City")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 12), "set oxygen")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 23)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Open City should play successfully at 12% oxygen")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+4, prodAfter.Credits,
		"Credit production should increase by 4")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}
func TestOpenCity_FailsBelowOxygenRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Open City")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 23)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "oxygen requirement not met", "Open City should fail when oxygen is below 12%")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 109: Media Group
// "Effect: After you play an event card, you gain 3 M€."
// =============================================================================
func TestMediaGroup_Gain3Credits(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Media Group")
	eventCard := gamecards.Card{ID: "test-event", Name: "Test Event", Type: gamecards.CardTypeEvent, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagEvent}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{eventCard})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 4), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Media Group should play successfully")
	// Now play an event card to trigger the passive effect
	p.Hand().AddCard("test-event")
	creditsBefore := p.Resources().Get().Credits
	payment = shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "test-event", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Event card should play successfully")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore+3, creditsAfter,
		"Should gain 3 credits from Media Group passive effect after playing event card")
}

// =============================================================================
// Card 110: Business Network
// Auto: "Decrease your M€ production 1 step."
// Action: "Look at the top card and buy or discard it."
// =============================================================================
func TestBusinessNetwork_DecreaseCreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Business Network")
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
		shared.ResourceCreditProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Business Network should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-1, prodAfter.Credits,
		"Credit production should decrease by 1")
}

// =============================================================================
// Card 112: Bribed Committee
// "Gain 2 TR."
// =============================================================================
func TestBribedCommittee_Gain2TR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Bribed Committee")
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
	p.Hand().AddCard(card.ID)
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Bribed Committee should play successfully")
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+2, trAfter, "TR should increase by 2")
}

// =============================================================================
// Card 113: Solar Power
// "Increase your energy production 1 step."
// =============================================================================
func TestSolarPower_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Solar Power")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Solar Power should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy,
		"Energy production should increase by 1")
}

// =============================================================================
// Card 114: Breathing Filters
// No behaviors, just VP. Requires 7% oxygen.
// =============================================================================
func TestBreathingFilters_PlaysWithOxygenRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Breathing Filters")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 7), "set oxygen")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Breathing Filters should play successfully at 7% oxygen")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Breathing Filters should be in played cards")
}
func TestBreathingFilters_FailsBelowOxygen(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Breathing Filters")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "oxygen requirement not met", "Breathing Filters should fail with oxygen below 7%")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 117: Geothermal Power
// "Increase your energy production 2 steps."
// =============================================================================
func TestGeothermalPower_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Geothermal Power")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Geothermal Power should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+2, prodAfter.Energy,
		"Energy production should increase by 2")
}

// =============================================================================
// Card 118: Farming
// "Gain 2 plants. Increase your M€ production 2 steps and plant production 2 steps."
// Requires 4 C or warmer.
// =============================================================================
func TestFarming_PlantsAndProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Farming")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, 4), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Farming should play successfully at 4C")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits,
		"Credit production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants,
		"Plant production should increase by 2")
}

// =============================================================================
// Card 119: Dust Seals
// No behaviors, just VP. Requires 3 oceans or less.
// =============================================================================
func TestDustSeals_PlaysWithMaxOceanRequirement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Dust Seals")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 2)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Dust Seals should play successfully with 0 oceans")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Dust Seals should be in played cards")
}

// =============================================================================
// Card 122: Moss
// "Lose 1 plant. Increase plant production 1 step." Requires 3 oceans.
// =============================================================================
func TestMoss_LosePlantGainPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Moss")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 3), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourcePlant:  5,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Moss should play successfully with 3 oceans")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore-1, plantsAfter, "Should lose 1 plant")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants,
		"Plant production should increase by 1")
}

// =============================================================================
// Card 123: Industrial Center
// Action: "Spend 7 M€ to increase your steel production 1 step."
// =============================================================================

// =============================================================================
// Card 125: Hackers
// "Increase your M€ production 2 steps, decrease your energy production 1 step,
//
//	and decrease any player's M€ production 2 steps."
//
// =============================================================================
func TestHackers_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Hackers")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	target := players[1]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceCreditProduction: 5,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 3)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Hackers should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits,
		"Credit production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, targetProdBefore.Credits-2, targetProdAfter.Credits,
		"Target credit production should decrease by 2")
}

// =============================================================================
// Card 126: GHG Factories
// "Decrease your energy production 1 step and increase your heat production 4 steps."
// =============================================================================
func TestGHGFactories_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("GHG Factories")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "GHG Factories should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Heat+4, prodAfter.Heat,
		"Heat production should increase by 4")
}

// =============================================================================
// Card 127: Subterranean Reservoir
// "Place an ocean tile."
// =============================================================================
func TestSubterraneanReservoir_PlaceOcean(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Subterranean Reservoir")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Subterranean Reservoir should play successfully")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending ocean tile selection")
}

// =============================================================================
// Card 129: Zeppelins
// "Increase your M€ production 1 step for each city tile on Mars."
// Requires 5% oxygen.
// =============================================================================
func TestZeppelins_CreditProductionPerCityOnMars(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Zeppelins")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 5), "set oxygen")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Zeppelins should play successfully at 5% oxygen")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits, prodAfter.Credits,
		"Credit production should not change with 0 cities on Mars")
}

// =============================================================================
// Card 131: Decomposers
// "Effect: When you play an animal, plant, or microbe tag, including this,
// add a microbe to this card." Requires 3% oxygen.
// =============================================================================
func TestDecomposers_AddMicrobeOnPlay(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Decomposers")
	microbeCard := gamecards.Card{ID: "test-microbe", Name: "Test Microbe", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagMicrobe}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{microbeCard})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 4), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 3), "set oxygen")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Decomposers should play successfully at 3% oxygen")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "Decomposers includes its own microbe tag")
	// Play a microbe-tagged card to trigger the passive effect
	p.Hand().AddCard("test-microbe")
	payment = shared.Payment{Allocations: []shared.PaymentAllocation{}}
	err = playCardAction.Execute(ctx, testGame.ID(), p.ID(), "test-microbe", payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Microbe card should play successfully")
	microbeStorage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 2, microbeStorage, "Decomposers rewards its own and the later microbe tag")
}

// =============================================================================
// Card 132: Fusion Power
// "Increase your energy production 3 steps." Requires 2 power tags.
// =============================================================================
func TestFusionPower_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Fusion Power")
	powerCard1 := gamecards.Card{ID: "power1", Name: "Solar Power", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 11, Tags: []shared.CardTag{shared.TagBuilding, shared.TagPower}}
	powerCard2 := gamecards.Card{ID: "power2", Name: "Power Plant", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 4, Tags: []shared.CardTag{shared.TagBuilding, shared.TagPower}}
	additionalCards := []gamecards.Card{powerCard1, powerCard2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("power1", "Solar Power", "automated", []string{"building", "power"})
	p.PlayedCards().AddCard("power2", "Power Plant", "automated", []string{"building", "power"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 14)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Fusion Power should play successfully with 2 power tags")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+3, prodAfter.Energy,
		"Energy production should increase by 3")
}
func TestFusionPower_FailsWithInsufficientPowerTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Fusion Power")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 14)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 2 additional tags", "Fusion Power should fail without 2 power tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 133: Symbiotic Fungus
// Action: "Add 1 microbe to any card." Requires -14C or warmer.
// =============================================================================
func TestSymbioticFungus_ActionAddMicrobeToAnyCard(t *testing.T) {
	testGame, repo, _, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-symbiotic-fungus"
	targetCardID := "test-target-microbe-card"
	p.PlayedCards().AddCard(cardID, "Symbiotic Fungus", "active", []string{"microbe"})
	p.PlayedCards().AddCard(targetCardID, "Decomposers", "active", []string{"microbe"})
	p.Resources().AddToStorage(targetCardID, 0)
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{
		{ID: targetCardID, Name: "Decomposers", Type: gamecards.CardTypeActive, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceMicrobe}},
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardStorageCondition(shared.ResourceMicrobe, 1, "any-card"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Symbiotic Fungus",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, []string{targetCardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Symbiotic Fungus action should succeed")
	microbeStorage := p.Resources().GetCardStorage(targetCardID)
	testutil.AssertEqual(t, 1, microbeStorage, "Target card should have 1 microbe")
}

// =============================================================================
// Card 135: Advanced Ecosystems
// No behaviors, just VP. Requires 1 plant, 1 microbe, 1 animal tag.
// =============================================================================
func TestAdvancedEcosystems_PlaysWithTagRequirements(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Advanced Ecosystems")
	plantDummy := gamecards.Card{ID: "plant1", Name: "Moss", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 4, Tags: []shared.CardTag{shared.TagPlant}}
	microbeDummy := gamecards.Card{ID: "microbe1", Name: "Decomposers", Type: gamecards.CardTypeActive, GameModes: []string{"classic"}, Cost: 5, Tags: []shared.CardTag{shared.TagMicrobe}}
	animalDummy := gamecards.Card{ID: "animal1", Name: "Fish", Type: gamecards.CardTypeActive, GameModes: []string{"classic"}, Cost: 9, Tags: []shared.CardTag{shared.TagAnimal}}
	additionalCards := []gamecards.Card{plantDummy, microbeDummy, animalDummy}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("plant1", "Moss", "automated", []string{"plant"})
	p.PlayedCards().AddCard("microbe1", "Decomposers", "active", []string{"microbe"})
	p.PlayedCards().AddCard("animal1", "Fish", "active", []string{"animal"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Advanced Ecosystems should play successfully with required tags")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Advanced Ecosystems should be in played cards")
}

// =============================================================================
// Card 136: Great Dam
// "Increase your energy production 2 steps." Requires 4 oceans.
// =============================================================================
func TestGreatDam_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Great Dam")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 4), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Great Dam should play successfully with 4 oceans")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+2, prodAfter.Energy,
		"Energy production should increase by 2")
}

// =============================================================================
// Card 137: Cartel
// "Increase your M€ production 1 step for each Earth tag you have, including this."
// =============================================================================
func TestCartel_CreditProductionPerEarthTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Cartel")
	earthCard1 := gamecards.Card{ID: "earth1", Name: "Media Group", Type: gamecards.CardTypeActive, GameModes: []string{"classic"}, Cost: 6, Tags: []shared.CardTag{shared.TagEarth}}
	earthCard2 := gamecards.Card{ID: "earth2", Name: "Acquired Company", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 10, Tags: []shared.CardTag{shared.TagEarth}}
	additionalCards := []gamecards.Card{earthCard1, earthCard2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("earth1", "Media Group", "active", []string{"earth"})
	p.PlayedCards().AddCard("earth2", "Acquired Company", "automated", []string{"earth"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Cartel should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3 (2 existing earth tags + 1 from Cartel itself)")
}

// =============================================================================
// Card 138: Strip Mine
// "Increase your steel production 2 steps, titanium production 1 step,
//
//	decrease energy production 2 steps. Raise oxygen 2 steps."
//
// =============================================================================
func TestStripMine_ProductionAndOxygen(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Strip Mine")
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
		shared.ResourceEnergyProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 25)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Strip Mine should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Steel+2, prodAfter.Steel,
		"Steel production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium,
		"Titanium production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Energy-2, prodAfter.Energy,
		"Energy production should decrease by 2")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+2, oxygenAfter,
		"Oxygen should increase by 2 steps")
}

// =============================================================================
// Card 139: Wave Power
// "Increase your energy production 1 step." Requires 3 oceans.
// =============================================================================
func TestWavePower_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Wave Power")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 3), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Wave Power should play successfully with 3 oceans")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy,
		"Energy production should increase by 1")
}

// =============================================================================
// Card 140: Lava Flows
// "Raise temperature 2 steps."
// =============================================================================
func TestLavaFlows_RaiseTemperature(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Lava Flows")
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
	p.Hand().AddCard(card.ID)
	tempBefore := testGame.GlobalParameters().Temperature()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Lava Flows should play successfully")
	tempAfter := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, tempBefore+4, tempAfter,
		"Temperature should increase by 2 steps (4 degrees)")
}

// =============================================================================
// Card 141: Power Plant
// "Increase your energy production 1 step."
// =============================================================================
func TestPowerPlant_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Power Plant")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Power Plant should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy,
		"Energy production should increase by 1")
}

// =============================================================================
// Card 143: Large Convoy
// "Place an ocean tile. Draw 2 cards."
// =============================================================================
func TestLargeConvoy_OceanAndCardDraw(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Large Convoy")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 200,
	})
	p.Hand().AddCard(card.ID)
	handSizeBefore := p.Hand().CardCount()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 36)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, testutil.IntPtr(0), nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Large Convoy should play successfully")
	handSizeAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handSizeBefore-1+2, handSizeAfter,
		"Hand should have 2 more cards (minus 1 played + 2 drawn)")
}

// =============================================================================
// Card 144: Titanium Mine
// "Increase your titanium production 1 step."
// =============================================================================
func TestTitaniumMine_TitaniumProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Titanium Mine")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Titanium Mine should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium,
		"Titanium production should increase by 1")
}

// =============================================================================
// Card 145: Tectonic Stress Power
// "Increase your energy production 3 steps." Requires 2 science tags.
// =============================================================================
func TestTectonicStressPower_EnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Tectonic Stress Power")
	sciCard1 := gamecards.Card{ID: "science1", Name: "Research", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 11, Tags: []shared.CardTag{shared.TagScience}}
	sciCard2 := gamecards.Card{ID: "science2", Name: "Physics Complex", Type: gamecards.CardTypeActive, GameModes: []string{"classic"}, Cost: 12, Tags: []shared.CardTag{shared.TagScience, shared.TagBuilding}}
	additionalCards := []gamecards.Card{sciCard1, sciCard2}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("science1", "Research", "automated", []string{"science"})
	p.PlayedCards().AddCard("science2", "Physics Complex", "active", []string{"science", "building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 18)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Tectonic Stress Power should play successfully with 2 science tags")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy+3, prodAfter.Energy,
		"Energy production should increase by 3")
}

// =============================================================================
// Card 146: Nitrophilic Moss
// "Lose 2 plants. Increase your plant production 2 steps." Requires 3 oceans.
// =============================================================================
func TestNitrophilicMoss_LosePlantsGainPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Nitrophilic Moss")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOceans(ctx, 3), "set oceans")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourcePlant:  5,
	})
	p.Hand().AddCard(card.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Nitrophilic Moss should play successfully with 3 oceans")
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore-2, plantsAfter, "Should lose 2 plants")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants,
		"Plant production should increase by 2")
}

// =============================================================================
// Card 147: Herbivores
// "Add 1 animal to this card. Decrease any player's plant production 1 step."
// Requires 8% oxygen.
// =============================================================================
func TestHerbivores_AddAnimalAndDecreaseTargetPlantProd(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Herbivores")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	target := players[1]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 8), "set oxygen")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourcePlantProduction: 3,
	})
	p.Hand().AddCard(card.ID)
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Herbivores should play successfully at 8% oxygen")
	animalStorage := p.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 1, animalStorage, "Herbivores should have 1 animal")
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, targetProdBefore.Plants-1, targetProdAfter.Plants,
		"Target plant production should decrease by 1")
}

// =============================================================================
// Card 148: Insects
// "Increase your plant production 1 step for each plant tag you have."
// Requires 6% oxygen.
// =============================================================================
func TestInsects_PlantProductionPerPlantTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Insects")
	plantDummy1 := gamecards.Card{ID: "plant1", Name: "Moss", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 4, Tags: []shared.CardTag{shared.TagPlant}}
	plantDummy2 := gamecards.Card{ID: "plant2", Name: "Farming", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 16, Tags: []shared.CardTag{shared.TagPlant}}
	plantDummy3 := gamecards.Card{ID: "plant3", Name: "Nitrophilic Moss", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 8, Tags: []shared.CardTag{shared.TagPlant}}
	additionalCards := []gamecards.Card{plantDummy1, plantDummy2, plantDummy3}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 6), "set oxygen")
	p.PlayedCards().AddCard("plant1", "Moss", "automated", []string{"plant"})
	p.PlayedCards().AddCard("plant2", "Farming", "automated", []string{"plant"})
	p.PlayedCards().AddCard("plant3", "Nitrophilic Moss", "automated", []string{"plant"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Insects should play successfully at 6% oxygen")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+3, prodAfter.Plants,
		"Plant production should increase by 3 (1 per plant tag, 3 plant tags in play)")
}

// =============================================================================
// Card 151: Investment Loan
// "Gain 10 M€. Decrease your M€ production 1 step."
// =============================================================================
func TestInvestmentLoan_GainCreditsLoseProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Investment Loan")
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
		shared.ResourceCreditProduction: 2,
	})
	p.Hand().AddCard(card.ID)
	creditsBefore := p.Resources().Get().Credits
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 3)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Investment Loan should play successfully")
	creditsAfter := p.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore-3+10, creditsAfter,
		"Should gain 10 credits minus 3 cost = net +7")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits-1, prodAfter.Credits,
		"Credit production should decrease by 1")
}

// =============================================================================
// Card 154: Caretaker Contract
// Action: "Spend 8 heat to gain 1 TR." Requires 0C or warmer.
// =============================================================================
func TestCaretakerContract_ActionSpendHeatGainTR(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, 0), "set temperature")
	cardID := "test-caretaker-contract"
	p.PlayedCards().AddCard(cardID, "Caretaker Contract", "active", []string{})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceHeat: 15,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceHeat, 8, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceTR, 1, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Caretaker Contract",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	trBefore := p.Resources().TerraformRating()
	heatBefore := p.Resources().Get().Heat
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Caretaker Contract action should succeed")
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+1, trAfter, "TR should increase by 1")
	heatAfter := p.Resources().Get().Heat
	testutil.AssertEqual(t, heatBefore-8, heatAfter, "Should spend 8 heat")
}

// =============================================================================
// Card 155: Designed Microorganisms
// "Increase your plant production 2 steps." Requires -14C or colder.
// =============================================================================
func TestDesignedMicroorganisms_PlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Designed Microorganisms")
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
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Designed Microorganisms should play successfully at -30C (below -14C)")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants,
		"Plant production should increase by 2")
}
func TestDesignedMicroorganisms_FailsAboveMaxTemp(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Designed Microorganisms")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, 0), "set temperature")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "temperature requirement not met", "Designed Microorganisms should fail when temperature is above -14C")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 156: Standard Technology
// "Effect: After you pay for a standard project, except selling patents,
// you gain 3 M€."
// =============================================================================
func TestStandardTechnology_Gain3Credits(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Standard Technology")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Standard Technology should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Standard Technology should be in played cards")
	// Verify the passive effect was registered
	effects := p.Effects().List()
	foundEffect := false
	for _, effect := range effects {
		if effect.CardID == card.ID && effect.CardName == "Standard Technology" {
			foundEffect = true
			break
		}
	}
	testutil.AssertTrue(t, foundEffect,
		"Standard Technology should have registered its standard-project-played passive effect")
}

// =============================================================================
// Card 153: Adaptation Technology
// "Gain +2 global parameter requirement lenience."
// =============================================================================
func TestAdaptationTechnology_GlobalParameterLenience(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Adaptation Technology")
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
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Adaptation Technology should play successfully")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Adaptation Technology should be in played cards")
}

// =============================================================================
// Card 150: Anti-Gravity Technology
// "All cards cost 2 M€ less." Requires 7 science tags.
// =============================================================================
func TestAntiGravityTechnology_Discount(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Anti-Gravity Technology")
	scienceCards := make([]gamecards.Card, 7)
	for i := 0; i < 7; i++ {
		scienceCards[i] = gamecards.Card{
			ID:        "science-dummy-" + string(rune('a'+i)),
			Name:      "Science Dummy",
			Type:      gamecards.CardTypeAutomated,
			GameModes: []string{"classic"},
			Cost:      1,
			Tags:      []shared.CardTag{shared.TagScience},
		}
	}
	additionalCards := append([]gamecards.Card{card}, scienceCards...)
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards(additionalCards)
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	for i := 0; i < 7; i++ {
		cardIDStr := "science-dummy-" + string(rune('a'+i))
		p.PlayedCards().AddCard(cardIDStr, "Science Dummy", "automated", []string{"science"})
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 14)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Anti-Gravity Technology should play successfully with 7 science tags")
	testutil.AssertTrue(t, p.PlayedCards().Contains(card.ID),
		"Anti-Gravity Technology should be in played cards")
}
func TestAntiGravityTechnology_FailsWithInsufficientScienceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Anti-Gravity Technology")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("science-1", "Research", "automated", []string{"science"})
	p.PlayedCards().AddCard("science-2", "Physics Complex", "active", []string{"science"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 14)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "tag requirements need 7 additional tags", "Anti-Gravity Technology should fail without 7 science tags")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

// =============================================================================
// Card 158: Industrial Microbes
// "Increase your energy production and your steel production 1 step each."
// =============================================================================
func TestIndustrialMicrobes_IncreasesEnergyAndSteelProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	industrialMicrobes := testutil.GetCardByName("Industrial Microbes")
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
	p.Hand().AddCard(industrialMicrobes.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 12)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), industrialMicrobes.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Industrial Microbes should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Steel+1, prodAfter.Steel, "Steel production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy, "Energy production should increase by 1")
}

// =============================================================================
// Card 159: Lichen
// "Requires -24C or warmer. Increase your plant production 1 step."
// =============================================================================
func TestLichen_IncreasesPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	lichen := testutil.GetCardByName("Lichen")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Set temperature to -24 (3 steps from -30)
	if _, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 3, ""); err != nil {
		t.Fatalf("Failed to increase temperature: %v", err)
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(lichen.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), lichen.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Lichen should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
}

// =============================================================================
// Card 162: Imported GHG
// "Increase your heat production 1 step and gain 3 heat."
// =============================================================================
func TestImportedGHG_IncreasesHeatProductionAndGainsHeat(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	importedGHG := testutil.GetCardByName("Imported GHG")
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
	p.Hand().AddCard(importedGHG.ID)
	heatBefore := p.Resources().Get().Heat
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 7)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), importedGHG.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Imported GHG should play successfully")
	heatAfter := p.Resources().Get().Heat
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, heatBefore+3, heatAfter, "Should gain 3 heat")
	testutil.AssertEqual(t, prodBefore.Heat+1, prodAfter.Heat, "Heat production should increase by 1")
}

// =============================================================================
// Card 164: Micro-Mills
// "Increase your heat production 1 step."
// =============================================================================
func TestMicroMills_IncreasesHeatProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	microMills := testutil.GetCardByName("Micro-Mills")
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
	p.Hand().AddCard(microMills.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 3)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), microMills.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Micro-Mills should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+1, prodAfter.Heat, "Heat production should increase by 1")
}

// =============================================================================
// Card 165: Magnetic Field Generators
// "Decrease your energy production 4 steps and increase your plant production 2 steps. Raise your TR 3 steps."
// =============================================================================
func TestMagneticFieldGenerators_ProductionAndTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	magneticFieldGenerators := testutil.GetCardByName("Magnetic Field Generators")
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
		shared.ResourceEnergyProduction: 4,
	})
	p.Hand().AddCard(magneticFieldGenerators.ID)
	prodBefore := p.Resources().Production()
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 20)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), magneticFieldGenerators.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Magnetic Field Generators should play successfully")
	prodAfter := p.Resources().Production()
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, prodBefore.Energy-4, prodAfter.Energy, "Energy production should decrease by 4")
	testutil.AssertEqual(t, prodBefore.Plants+2, prodAfter.Plants, "Plant production should increase by 2")
	testutil.AssertEqual(t, trBefore+3, trAfter, "TR should increase by 3")
}

// =============================================================================
// Card 167: Import Of Advanced GHG
// "Increase your heat production 2 steps."
// =============================================================================
func TestImportOfAdvancedGHG_IncreasesHeatProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	importAdvGHG := testutil.GetCardByName("Import Of Advanced GHG")
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
	p.Hand().AddCard(importAdvGHG.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), importAdvGHG.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Import Of Advanced GHG should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat, "Heat production should increase by 2")
}

// =============================================================================
// Card 169: Tundra Farming
// "Requires -6C or warmer. Increase your plant production 1 step and your M$ production 2 steps. Gain 1 plant."
// =============================================================================
func TestTundraFarming_ProductionAndPlant(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	tundraFarming := testutil.GetCardByName("Tundra Farming")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Temperature needs to be -6 or warmer. Default is -30. Need 12 steps (each step is +2).
	if _, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 12, ""); err != nil {
		t.Fatalf("Failed to increase temperature: %v", err)
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(tundraFarming.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 16)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), tundraFarming.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Tundra Farming should play successfully")
	plantsAfter := p.Resources().Get().Plants
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+1, plantsAfter, "Should gain 1 plant")
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits, "Credit production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
}

// =============================================================================
// Card 170: Aerobraked Ammonia Asteroid
// "Add 2 microbes to another card. Increase your heat production 3 steps and your plant production 1 step."
// =============================================================================
func TestAerobrakedAmmoniaAsteroid_ProductionIncrease(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	aerobrakedAsteroid := testutil.GetCardByName("Aerobraked Ammonia Asteroid")
	microbeHost := gamecards.Card{
		ID: "card-microbe-host-b4", Name: "Microbe Host B4", Type: gamecards.CardTypeActive, Cost: 0,
		Tags:            []shared.CardTag{shared.TagMicrobe},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceMicrobe, Starting: 0},
	}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{microbeHost})

	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("card-microbe-host-b4", "Microbe Host B4", "active", []string{"microbe"})
	p.Resources().AddToStorage("card-microbe-host-b4", 0)
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(aerobrakedAsteroid.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 26)
	targetCardID := "card-microbe-host-b4"
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), aerobrakedAsteroid.ID, payment, nil, []string{targetCardID}, nil, nil, nil)
	testutil.AssertNoError(t, err, "Aerobraked Ammonia Asteroid should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+3, prodAfter.Heat, "Heat production should increase by 3")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	microbeStorage := p.Resources().GetCardStorage("card-microbe-host-b4")
	testutil.AssertEqual(t, 2, microbeStorage, "Should add 2 microbes to host card")
}

// =============================================================================
// Card 171: Magnetic Field Dome
// "Decrease your energy production 2 steps and increase your plant production 1 step. Raise your terraform rating 1 step."
// =============================================================================
func TestMagneticFieldDome_ProductionAndTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	magneticFieldDome := testutil.GetCardByName("Magnetic Field Dome")
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
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(magneticFieldDome.ID)
	prodBefore := p.Resources().Production()
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), magneticFieldDome.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Magnetic Field Dome should play successfully")
	prodAfter := p.Resources().Production()
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, prodBefore.Energy-2, prodAfter.Energy, "Energy production should decrease by 2")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
	testutil.AssertEqual(t, trBefore+1, trAfter, "TR should increase by 1")
}

// =============================================================================
// Card 175: Satellites
// "Increase your M$ production 1 step for each space tag you have, including this."
// =============================================================================
func TestSatellites_CreditProductionPerSpaceTag(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	satellites := testutil.GetCardByName("Satellites")
	spaceCard1 := gamecards.Card{ID: "card-space-1-b4", Name: "Space Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagSpace}, Cost: 0}
	spaceCard2 := gamecards.Card{ID: "card-space-2-b4", Name: "Space Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagSpace}, Cost: 0}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{satellites, spaceCard1, spaceCard2})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Play 2 space-tagged cards first
	p.PlayedCards().AddCard("card-space-1-b4", "Space Card 1", "automated", []string{"space"})
	p.PlayedCards().AddCard("card-space-2-b4", "Space Card 2", "automated", []string{"space"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(satellites.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), satellites.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Satellites should play successfully")
	prodAfter := p.Resources().Production()
	// 2 existing space tags + 1 from Satellites itself = 3 credit production
	testutil.AssertEqual(t, prodBefore.Credits+3, prodAfter.Credits,
		"Credit production should increase by 3 (1 per each of 2 existing + 1 self space tag)")
}
func TestSatellites_ZeroExistingSpaceTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	satellites := testutil.GetCardByName("Satellites")
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
	p.Hand().AddCard(satellites.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), satellites.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Satellites with no prior space tags should play successfully")
	prodAfter := p.Resources().Production()
	// Only 1 space tag from Satellites itself = 1 credit production
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits,
		"Credit production should increase by 1 (only self space tag)")
}

// =============================================================================
// Card 176: Noctis Farming
// "Requires -20C or warmer. Increase your M$ production 1 step and gain 2 plants."
// =============================================================================
func TestNoctisFarming_ProductionAndPlants(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	noctisFarming := testutil.GetCardByName("Noctis Farming")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Temperature needs to be -20 or warmer. Default is -30. Need 5 steps (each step is +2).
	if _, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 5, ""); err != nil {
		t.Fatalf("Failed to increase temperature: %v", err)
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(noctisFarming.ID)
	plantsBefore := p.Resources().Get().Plants
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), noctisFarming.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Noctis Farming should play successfully")
	plantsAfter := p.Resources().Get().Plants
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter, "Should gain 2 plants")
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits, "Credit production should increase by 1")
}

// =============================================================================
// Card 179: Soil Factory
// "Decrease your energy production 1 step and increase your plant production 1 step."
// =============================================================================
func TestSoilFactory_SwapsEnergyForPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	soilFactory := testutil.GetCardByName("Soil Factory")
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
	p.Hand().AddCard(soilFactory.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), soilFactory.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Soil Factory should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy, "Energy production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Plants+1, prodAfter.Plants, "Plant production should increase by 1")
}

// =============================================================================
// Card 180: Fuel Factory
// "Decrease your energy production 1 step and increase your titanium and your M$ production 1 step each."
// =============================================================================
func TestFuelFactory_SwapsEnergyForTitaniumAndCreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	fuelFactory := testutil.GetCardByName("Fuel Factory")
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
	p.Hand().AddCard(fuelFactory.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), fuelFactory.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Fuel Factory should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy, "Energy production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Titanium+1, prodAfter.Titanium, "Titanium production should increase by 1")
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits, "Credit production should increase by 1")
}

// =============================================================================
// Card 186: Rad-Suits
// "Requires 2 cities in play. Increase your M$ production 1 step."
// =============================================================================
func TestRadSuits_IncreasesCreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	radSuits := testutil.GetCardByName("Rad-Suits")
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
	p.Hand().AddCard(radSuits.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), radSuits.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Rad-Suits should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits, "Credit production should increase by 1")
}

// =============================================================================
// Card 196: Lagrange Observatory
// "Draw 1 card."
// =============================================================================
func TestLagrangeObservatory_DrawsCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	lagrange := testutil.GetCardByName("Lagrange Observatory")
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
	p.Hand().AddCard(lagrange.ID)
	handBefore := p.Hand().CardCount()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 9)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), lagrange.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Lagrange Observatory should play successfully")
	handAfter := p.Hand().CardCount()
	// Played 1 card (-1) and drew 1 card (+1) = net 0
	testutil.AssertEqual(t, handBefore, handAfter,
		"Hand size should remain same (played 1, drew 1)")
}

// =============================================================================
// Card 198: Immigration Shuttles
// "Increase your M$ production 5 steps."
// =============================================================================
func TestImmigrationShuttles_IncreasesCreditProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	immigrationShuttles := testutil.GetCardByName("Immigration Shuttles")
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
	p.Hand().AddCard(immigrationShuttles.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 31)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), immigrationShuttles.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Immigration Shuttles should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+5, prodAfter.Credits, "Credit production should increase by 5")
}

// =============================================================================
// Card 203: Soletta
// "Increase your heat production 7 steps."
// =============================================================================
func TestSoletta_IncreasesHeatProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	soletta := testutil.GetCardByName("Soletta")
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
	p.Hand().AddCard(soletta.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 35)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), soletta.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Soletta should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+7, prodAfter.Heat, "Heat production should increase by 7")
}

// =============================================================================
// Card 204: Technology Demonstration
// "Draw 2 cards."
// =============================================================================
func TestTechnologyDemonstration_DrawsTwoCards(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	techDemo := testutil.GetCardByName("Technology Demonstration")
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
	p.Hand().AddCard(techDemo.ID)
	handBefore := p.Hand().CardCount()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), techDemo.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Technology Demonstration should play successfully")
	handAfter := p.Hand().CardCount()
	// Played 1 card (-1) and drew 2 cards (+2) = net +1
	testutil.AssertEqual(t, handBefore+1, handAfter,
		"Hand size should increase by 1 (played 1, drew 2)")
}

// =============================================================================
// Card 205: Rad-Chem Factory
// "Decrease your energy production 1 step. Raise your terraform rating 2 steps."
// =============================================================================
func TestRadChemFactory_ProductionAndTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	radChemFactory := testutil.GetCardByName("Rad-Chem Factory")
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
	p.Hand().AddCard(radChemFactory.ID)
	prodBefore := p.Resources().Production()
	trBefore := p.Resources().TerraformRating()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 8)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), radChemFactory.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Rad-Chem Factory should play successfully")
	prodAfter := p.Resources().Production()
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy, "Energy production should decrease by 1")
	testutil.AssertEqual(t, trBefore+2, trAfter, "TR should increase by 2")
}

// =============================================================================
// Card 207: Medical Lab
// "Increase your M$ production 1 step for every 2 building tags you have, including this."
// =============================================================================
func TestMedicalLab_CreditProductionPerTwoBuildingTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	medicalLab := testutil.GetCardByName("Medical Lab")
	buildingCard1 := gamecards.Card{ID: "card-building-1-b4", Name: "Building Card 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagBuilding}, Cost: 0}
	buildingCard2 := gamecards.Card{ID: "card-building-2-b4", Name: "Building Card 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagBuilding}, Cost: 0}
	buildingCard3 := gamecards.Card{ID: "card-building-3-b4", Name: "Building Card 3", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagBuilding}, Cost: 0}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{medicalLab, buildingCard1, buildingCard2, buildingCard3})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Play 3 building-tagged cards first
	p.PlayedCards().AddCard("card-building-1-b4", "Building Card 1", "automated", []string{"building"})
	p.PlayedCards().AddCard("card-building-2-b4", "Building Card 2", "automated", []string{"building"})
	p.PlayedCards().AddCard("card-building-3-b4", "Building Card 3", "automated", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(medicalLab.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), medicalLab.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Medical Lab should play successfully")
	prodAfter := p.Resources().Production()
	// 3 existing building tags + 1 from Medical Lab itself = 4 building tags. 4 / 2 = 2 credit production
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits,
		"Credit production should increase by 2 (4 building tags / 2)")
}
func TestMedicalLab_OddNumberOfBuildingTags(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	medicalLab := testutil.GetCardByName("Medical Lab")
	buildingCard1 := gamecards.Card{ID: "card-building-odd-1", Name: "Building Card Odd 1", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagBuilding}, Cost: 0}
	buildingCard2 := gamecards.Card{ID: "card-building-odd-2", Name: "Building Card Odd 2", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{shared.TagBuilding}, Cost: 0}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{medicalLab, buildingCard1, buildingCard2})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Play 2 building-tagged cards first
	p.PlayedCards().AddCard("card-building-odd-1", "Building Card Odd 1", "automated", []string{"building"})
	p.PlayedCards().AddCard("card-building-odd-2", "Building Card Odd 2", "automated", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(medicalLab.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), medicalLab.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Medical Lab with odd building tags should play successfully")
	prodAfter := p.Resources().Production()
	// 2 existing + 1 from self = 3 building tags. 3 / 2 = 1 (floor division)
	testutil.AssertEqual(t, prodBefore.Credits+1, prodAfter.Credits,
		"Credit production should increase by 1 (3 building tags / 2 = 1, floor division)")
}

// =============================================================================
// Card 157: Nitrite Reducing Bacteria (active card with choices)
// "Action: Add 1 microbe to this card, or remove 3 microbes to increase your TR 1 step. Add 3 microbes to this card."
// =============================================================================
func TestNitriteReducingBacteria_OnPlay_AddsThreeMicrobes(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	nitriteReducing := testutil.GetCardByName("Nitrite Reducing Bacteria")
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
	p.Hand().AddCard(nitriteReducing.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), nitriteReducing.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Nitrite Reducing Bacteria should play successfully")
	microbeStorage := p.Resources().GetCardStorage(nitriteReducing.ID)
	testutil.AssertEqual(t, 3, microbeStorage, "Should have 3 microbes on card after playing")
}
func TestNitriteReducingBacteria_Action_AddMicrobe(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	nitriteReducing := testutil.GetCardByName("Nitrite Reducing Bacteria")
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
	p.Hand().AddCard(nitriteReducing.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), nitriteReducing.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Nitrite Reducing Bacteria should play successfully")
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(nitriteReducing.ID), "Should have 3 microbes after playing")
	choiceIndex := 0
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), nitriteReducing.ID, 1, &choiceIndex, []string{nitriteReducing.ID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add microbe) should succeed")
	testutil.AssertEqual(t, 4, p.Resources().GetCardStorage(nitriteReducing.ID), "Card should have 4 microbes after adding 1")
}
func TestNitriteReducingBacteria_Action_Remove3MicrobesForTR(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	nitriteReducing := testutil.GetCardByName("Nitrite Reducing Bacteria")
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
	p.Hand().AddCard(nitriteReducing.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 11)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), nitriteReducing.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Nitrite Reducing Bacteria should play successfully")
	// Card starts with 3 microbes from auto-trigger, add 2 more for the test
	p.Resources().AddToStorage(nitriteReducing.ID, 2)
	testutil.AssertEqual(t, 5, p.Resources().GetCardStorage(nitriteReducing.ID), "Should have 5 microbes before action")
	trBefore := p.Resources().TerraformRating()
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), nitriteReducing.ID, 1, &choiceIndex, []string{nitriteReducing.ID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Choice 1 (remove 3 microbes for TR) should succeed")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(nitriteReducing.ID), "Card should have 2 microbes after removing 3 from 5")
	trAfter := p.Resources().TerraformRating()
	testutil.AssertEqual(t, trBefore+1, trAfter, "TR should increase by 1")
}
func TestNitriteReducingBacteria_Action_FailsWithInsufficientMicrobes(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-nitrite-reducing-bacteria"
	p.PlayedCards().AddCard(cardID, "Nitrite Reducing Bacteria", "active", []string{"microbe"})
	p.Resources().AddToStorage(cardID, 2) // Only 2 microbes, need 3
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Nitrite Reducing Bacteria",
			BehaviorIndex: 1,
			Behavior:      nitriteReducingBacteriaBehavior(),
		},
	})
	choiceIndex := 1
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 1, &choiceIndex, []string{cardID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "insufficient resources on card", "Choice 1 should fail with only 2 microbes (need 3)")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(cardID), "microbes unchanged")
}

// =============================================================================
// Card 177: Water Splitting Plant (active card action)
// "Action: Spend 3 energy to raise oxygen 1 step. Requires 2 ocean tiles."
// =============================================================================
func TestWaterSplittingPlant_SpendEnergyToRaiseOxygen(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-water-splitting-plant"
	p.PlayedCards().AddCard(cardID, "Water Splitting Plant", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 5,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 3, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Water Splitting Plant",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	oxygenBefore := testGame.GlobalParameters().Oxygen()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Water Splitting Plant action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 2, resources.Energy, "Should have 2 energy after spending 3")
	oxygenAfter := testGame.GlobalParameters().Oxygen()
	testutil.AssertEqual(t, oxygenBefore+1, oxygenAfter, "Oxygen should increase by 1")
}
func TestWaterSplittingPlant_FailsWithoutEnoughEnergy(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-water-splitting-plant"
	p.PlayedCards().AddCard(cardID, "Water Splitting Plant", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceEnergy: 2, // Only 2, need 3
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceEnergy, 3, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewGlobalParameterCondition(shared.ResourceOxygen, 1, "none"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Water Splitting Plant",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	before := p.Resources().Get()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Water Splitting Plant should fail without enough energy")
	testutil.AssertEqual(t, before, p.Resources().Get(), "rejected action spends nothing")
}

// =============================================================================
// Card 184: Livestock (active card action)
// "Action: Add 1 animal to this card. Requires 9% oxygen."
// "Decrease your plant production 1 step and increase your M$ production 2 steps."
// =============================================================================
func TestLivestock_OnPlay_ChangesProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	livestock := testutil.GetCardByName("Livestock")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Oxygen needs to be at 9 for the requirement
	if _, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 9, ""); err != nil {
		t.Fatalf("Failed to increase oxygen: %v", err)
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourcePlantProduction: 1,
	})
	p.Hand().AddCard(livestock.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), livestock.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Livestock should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits, "Credit production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Plants-1, prodAfter.Plants, "Plant production should decrease by 1")
}
func TestLivestock_Action_AddAnimal(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	livestock := testutil.GetCardByName("Livestock")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Oxygen needs to be at 9 for the requirement (same as OnPlay test)
	if _, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 9, ""); err != nil {
		t.Fatalf("Failed to increase oxygen: %v", err)
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourcePlantProduction: 1,
	})
	p.Hand().AddCard(livestock.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), livestock.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Livestock should play successfully")
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), livestock.ID, 1, nil, []string{livestock.ID}, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Livestock action should succeed")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(livestock.ID), "Card should have 1 animal after action")
}

// =============================================================================
// Card 202: Underground Detonations (active card action)
// "Action: Spend 10 M$ to increase your heat production 2 steps."
// =============================================================================
func TestUndergroundDetonations_SpendCreditsForHeatProduction(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-underground-detonations"
	p.PlayedCards().AddCard(cardID, "Underground Detonations", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 20,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 10, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceHeatProduction, 2, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Underground Detonations",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	prodBefore := p.Resources().Production()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Underground Detonations action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 10, resources.Credits, "Should have 10 credits after spending 10")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Heat+2, prodAfter.Heat, "Heat production should increase by 2")
}
func TestUndergroundDetonations_FailsWithInsufficientCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-underground-detonations"
	p.PlayedCards().AddCard(cardID, "Underground Detonations", "active", []string{"building"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 5, // Only 5, need 10
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 10, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewProductionCondition(shared.ResourceHeatProduction, 2, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Underground Detonations",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	before := p.Resources().Get()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Underground Detonations should fail without enough credits")
	testutil.AssertEqual(t, before, p.Resources().Get(), "rejected action spends nothing")
}

// =============================================================================
// Card 208: AI Central (active card action)
// "Action: Draw 2 cards. Requires 3 science tags to play. Decrease your energy production 1 step."
// =============================================================================
func TestAICentral_OnPlay_DecreasesEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	aiCentral := testutil.GetCardByName("AI Central")
	sci1 := gamecards.Card{ID: "sci1", Name: "Sci1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	sci2 := gamecards.Card{ID: "sci2", Name: "Sci2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{sci1, sci2})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("sci1", "Sci1", "automated", []string{"science"})
	p.PlayedCards().AddCard("sci2", "Sci2", "automated", []string{"science"})
	prerequisite := testutil.GetCardByName("Search For Life")
	p.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(aiCentral.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), aiCentral.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "AI Central should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy, "Energy production should decrease by 1")
}
func TestAICentral_Action_DrawTwoCards(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	aiCentral := testutil.GetCardByName("AI Central")
	sci1 := gamecards.Card{ID: "sci1", Name: "Sci1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	sci2 := gamecards.Card{ID: "sci2", Name: "Sci2", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagScience}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{sci1, sci2})
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.PlayedCards().AddCard("sci1", "Sci1", "automated", []string{"science"})
	p.PlayedCards().AddCard("sci2", "Sci2", "automated", []string{"science"})
	prerequisite := testutil.GetCardByName("Search For Life")
	p.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	p.Hand().AddCard(aiCentral.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 21)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), aiCentral.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "AI Central should play successfully")
	handBefore := p.Hand().CardCount()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err = useAction.Execute(ctx, testGame.ID(), p.ID(), aiCentral.ID, 1, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "AI Central action should succeed")
	handAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handBefore+2, handAfter, "Hand should increase by 2 from drawing 2 cards")
}

// =============================================================================
// Card 160: Power Supply Consortium
// "Requires 2 power tags. Decrease any energy production 1 step and increase your own 1 step."
// =============================================================================
func TestPowerSupplyConsortium_StealEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	powerSupplyConsortium := testutil.GetCardByName("Power Supply Consortium")
	power1 := gamecards.Card{ID: "power1", Name: "Power1", Type: gamecards.CardTypeAutomated, GameModes: []string{"classic"}, Cost: 0, Tags: []shared.CardTag{shared.TagPower}}
	cardRegistry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{power1})
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.PlayedCards().AddCard("power1", "Power1", "automated", []string{"power"})
	prerequisite := testutil.GetCardByName("Power Plant")
	attacker.PlayedCards().AddCard(prerequisite.ID, prerequisite.Name, string(prerequisite.Type), nil)

	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(powerSupplyConsortium.ID)
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 3,
	})
	attackerProdBefore := attacker.Resources().Production()
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 5)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), powerSupplyConsortium.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Power Supply Consortium should play successfully")
	attackerProdAfter := attacker.Resources().Production()
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, attackerProdBefore.Energy+1, attackerProdAfter.Energy,
		"Attacker energy production should increase by 1")
	testutil.AssertEqual(t, targetProdBefore.Energy-1, targetProdAfter.Energy,
		"Target energy production should decrease by 1")
}

// =============================================================================
// Card 201: Energy Tapping
// "Decrease any energy production 1 step and increase your own 1 step."
// =============================================================================
func TestEnergyTapping_StealEnergyProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	energyTapping := testutil.GetCardByName("Energy Tapping")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(energyTapping.ID)
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 2,
	})
	attackerProdBefore := attacker.Resources().Production()
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 3)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), energyTapping.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Energy Tapping should play successfully")
	attackerProdAfter := attacker.Resources().Production()
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, attackerProdBefore.Energy+1, attackerProdAfter.Energy,
		"Attacker energy production should increase by 1")
	testutil.AssertEqual(t, targetProdBefore.Energy-1, targetProdAfter.Energy,
		"Target energy production should decrease by 1")
}
func TestEnergyTapping_SoloMode(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	energyTapping := testutil.GetCardByName("Energy Tapping")
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
	p.Hand().AddCard(energyTapping.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 3)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), energyTapping.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Energy Tapping should play successfully in solo mode")
	prodAfter := p.Resources().Production()
	// Self-player (+1) applied, any-player (-1) skipped in solo
	testutil.AssertEqual(t, prodBefore.Energy+1, prodAfter.Energy,
		"Energy production should increase by 1 in solo (any-player decrease skipped)")
}

// =============================================================================
// Card 178: Heat Trappers
// "Decrease any heat production 2 steps and increase your energy production 1 step."
// =============================================================================
func TestHeatTrappers_StealHeatProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	heatTrappers := testutil.GetCardByName("Heat Trappers")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(heatTrappers.ID)
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceHeatProduction: 5,
	})
	attackerProdBefore := attacker.Resources().Production()
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 6)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), heatTrappers.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Heat Trappers should play successfully")
	attackerProdAfter := attacker.Resources().Production()
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, attackerProdBefore.Energy+1, attackerProdAfter.Energy,
		"Attacker energy production should increase by 1")
	testutil.AssertEqual(t, targetProdBefore.Heat-2, targetProdAfter.Heat,
		"Target heat production should decrease by 2")
}

// =============================================================================
// Card 183: Biomass Combustors
// "Requires 6% oxygen. Decrease any plant production 1 step and increase your energy production 2 steps."
// =============================================================================
func TestBiomassCombustors_StealPlantProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	biomassCombustors := testutil.GetCardByName("Biomass Combustors")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	attacker := players[0]
	target := players[1]
	attacker.SetCorporationID(testutil.CardID("Tharsis Republic"))
	target.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, attacker.ID(), 2), "set current turn")
	// Oxygen needs to be 6
	if _, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 6, ""); err != nil {
		t.Fatalf("Failed to increase oxygen: %v", err)
	}
	attacker.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	attacker.Hand().AddCard(biomassCombustors.ID)
	target.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourcePlantProduction: 3,
	})
	attackerProdBefore := attacker.Resources().Production()
	targetProdBefore := target.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 4)
	targetID := target.ID()
	err := playCardAction.Execute(ctx, testGame.ID(), attacker.ID(), biomassCombustors.ID, payment, nil, nil, &targetID, nil, nil)
	testutil.AssertNoError(t, err, "Biomass Combustors should play successfully")
	attackerProdAfter := attacker.Resources().Production()
	targetProdAfter := target.Resources().Production()
	testutil.AssertEqual(t, attackerProdBefore.Energy+2, attackerProdAfter.Energy,
		"Attacker energy production should increase by 2")
	testutil.AssertEqual(t, targetProdBefore.Plants-1, targetProdAfter.Plants,
		"Target plant production should decrease by 1")
}

// =============================================================================
// Card 166: Shuttles (active card with discount effect)
// "Decrease your energy production 1 step and increase your M$ production 2 steps."
// "Effect: When you play a space card, you pay 2 M$ less for it."
// =============================================================================
func TestShuttles_OnPlay_ProductionChanges(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	shuttles := testutil.GetCardByName("Shuttles")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	// Oxygen needs to be 5 for the requirement
	if _, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 5, ""); err != nil {
		t.Fatalf("Failed to increase oxygen: %v", err)
	}
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceEnergyProduction: 1,
	})
	p.Hand().AddCard(shuttles.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), shuttles.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Shuttles should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Credits+2, prodAfter.Credits, "Credit production should increase by 2")
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy, "Energy production should decrease by 1")
}

// =============================================================================
// Card 199: Restricted Area (active card with action)
// "Action: Spend 2 M$ to draw a card."
// =============================================================================
func TestRestrictedArea_SpendCreditsToDrawCard(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-restricted-area"
	p.PlayedCards().AddCard(cardID, "Restricted Area", "active", []string{"science"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 10,
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Restricted Area",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	handBefore := p.Hand().CardCount()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Restricted Area action should succeed")
	resources := p.Resources().Get()
	testutil.AssertEqual(t, 8, resources.Credits, "Should have 8 credits after spending 2")
	handAfter := p.Hand().CardCount()
	testutil.AssertEqual(t, handBefore+1, handAfter, "Hand should increase by 1 from drawing 1 card")
}
func TestRestrictedArea_FailsWithoutEnoughCredits(t *testing.T) {
	testGame, repo, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	logger := testutil.TestLogger()
	ctx := context.Background()
	p, _ := testGame.GetPlayer(playerID)
	cardID := "test-restricted-area"
	p.PlayedCards().AddCard(cardID, "Restricted Area", "active", []string{"science"})
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 1, // Only 1, need 2
	})
	behavior := shared.CardBehavior{
		Triggers: []shared.Trigger{{Type: shared.TriggerTypeManual}},
		Inputs: []shared.BehaviorCondition{
			shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
		},
		Outputs: []shared.BehaviorCondition{
			shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
		},
	}
	p.Actions().SetActions([]shared.CardAction{
		{
			CardID:        cardID,
			CardName:      "Restricted Area",
			BehaviorIndex: 0,
			Behavior:      behavior,
		},
	})
	before := p.Resources().Get()
	useAction := cardAction.NewUseCardActionAction(repo, cardRegistry, nil, logger)
	err := useAction.Execute(ctx, testGame.ID(), playerID, cardID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "payment exceeds available source pool", "Restricted Area should fail without enough credits")
	testutil.AssertEqual(t, before, p.Resources().Get(), "rejected action spends nothing")
}

// --- Mining Rights (067) ---
// "Place this tile on an area with a steel or titanium placement bonus. Increase that production 1 step."
// This card should NOT require a choice index — the production increase is determined by the tile bonus.
func TestMiningRights_PlaceOnSteelBonus_IncreaseSteelProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Rights")
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)

	prodBefore := p.Resources().Production()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, card.Cost)

	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mining Rights should play without requiring a choice index")

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

func TestMiningRights_PlaceOnTitaniumBonus_IncreaseTitaniumProduction(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Mining Rights")
	cardRegistry := testutil.CreateTestCardRegistry()
	stateRepo := game.NewInMemoryGameStateRepository()

	p := testGame.GetAllPlayers()[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	p.Hand().AddCard(card.ID)

	prodBefore := p.Resources().Production()

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, card.Cost)

	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Mining Rights should play without requiring a choice index")

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

// --- Local Heat Trapping (190) ---
// "Spend 5 heat to either gain 4 plants, or to add 2 animals to another card."
// Should fail if player has less than 5 heat.
func TestLocalHeatTrapping_FailsWithInsufficientHeat(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Local Heat Trapping")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourceHeat:   1,
	})
	p.Hand().AddCard(card.ID)
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	choiceIndex := 0
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertErrorContains(t, err, "not enough heat", "Local Heat Trapping should fail with only 1 heat")
	testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "rejected card stays in hand")
}

func TestLocalHeatTrapping_PlaysWithSufficientHeat(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Local Heat Trapping")
	cardRegistry := testutil.CreateTestCardRegistry()
	players := testGame.GetAllPlayers()
	p := players[0]
	p.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.AssertNoError(t, testGame.UpdateStatus(ctx, shared.GameStatusActive), "update status")
	testutil.AssertNoError(t, testGame.UpdatePhase(ctx, shared.GamePhaseAction), "update phase")
	testutil.AssertNoError(t, testGame.SetCurrentTurn(ctx, p.ID(), 2), "set current turn")
	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
		shared.ResourceHeat:   5,
	})
	p.Hand().AddCard(card.ID)

	heatBefore := p.Resources().Get().Heat
	plantsBefore := p.Resources().Get().Plants

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 1)
	choiceIndex := 0
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, &choiceIndex, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Local Heat Trapping should play with 5 heat")

	heatAfter := p.Resources().Get().Heat
	plantsAfter := p.Resources().Get().Plants
	testutil.AssertEqual(t, heatBefore-5, heatAfter, "Heat should decrease by 5")
	testutil.AssertEqual(t, plantsBefore+4, plantsAfter, "Plants should increase by 4")
}

// --- Immigrant City (200) ---
// "Effect: Each time a city tile is placed, including this, increase your M€ production 1 step.
//
//	Decrease your energy production 1 step and decrease your M€ production 2 steps. Place a city tile."
func TestImmigrantCity_ProductionAndCityPlacement(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Immigrant City")
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
		shared.ResourceEnergyProduction: 3,
		shared.ResourceCreditProduction: 5,
	})
	p.Hand().AddCard(card.ID)
	prodBefore := p.Resources().Production()
	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.ResourceCredit, 13)
	err := playCardAction.Execute(ctx, testGame.ID(), p.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Immigrant City should play successfully")
	prodAfter := p.Resources().Production()
	testutil.AssertEqual(t, prodBefore.Energy-1, prodAfter.Energy,
		"Energy production should decrease by 1")
	testutil.AssertEqual(t, prodBefore.Credits-2, prodAfter.Credits,
		"Credit production should decrease by 2 (before city effect)")
	selection := testGame.GetPendingTileSelection(p.ID())
	testutil.AssertTrue(t, selection != nil, "Should have pending city tile selection")
}

// =============================================================================
// Flooding (188, event, base-game)
// Place an ocean. If there are tiles adjacent to this ocean, you may remove
// 4 M€ from the owner of one of those tiles.
// =============================================================================

var (
	floodingOceanHexA     = testutil.FormatHex(shared.HexPosition{Q: 4, R: -1, S: -3})
	floodingLandHexA      = testutil.FormatHex(shared.HexPosition{Q: 3, R: -1, S: -2})
	floodingLandHexB      = testutil.FormatHex(shared.HexPosition{Q: 3, R: 0, S: -3})
	floodingOceanIsolated = testutil.FormatHex(shared.HexPosition{Q: 0, R: 4, S: -4})
)

func playFloodingCard(ctx context.Context, t *testing.T, g *game.Game, repo game.GameRepository, playerID string) {
	t.Helper()
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()
	cr := testutil.CreateTestCardRegistry()

	p, _ := g.GetPlayer(playerID)
	card := testutil.GetCardByName("Flooding")
	p.Hand().AddCard(card.ID)

	playCard := cardAction.NewPlayCardAction(repo, cr, stateRepo, logger)
	payment := shared.NativePayment(shared.ResourceCredit, card.Cost)
	err := playCard.Execute(ctx, g.ID(), playerID, card.ID, payment, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("Flooding card should play: %v", err)
	}
}

func selectFloodingOceanTile(ctx context.Context, t *testing.T, g *game.Game, repo game.GameRepository, playerID string, oceanHex string) {
	t.Helper()
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()
	cr := testutil.CreateTestCardRegistry()

	selectTile := tileAction.NewSelectTileAction(repo, cr, stateRepo, logger)
	_, err := selectTile.Execute(ctx, g.ID(), playerID, oceanHex)
	if err != nil {
		t.Fatalf("Ocean tile placement should succeed: %v", err)
	}
}

func TestFlooding_AdjacentOpponentTile_RemovalOffered(t *testing.T) {
	ctx := context.Background()
	testGame, repo, cardRegistry, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	_ = cardRegistry
	p1ID := playerIDs[0]
	p2ID := playerIDs[1]

	p1, _ := testGame.GetPlayer(p1ID)
	testutil.SetPlayerCredits(ctx, p1, 100)
	p2, _ := testGame.GetPlayer(p2ID)
	testutil.SetPlayerCredits(ctx, p2, 100)

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p2ID, "city", floodingLandHexA)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)

	pending := testGame.GetPendingTileSelection(p1ID)
	testutil.AssertTrue(t, pending != nil, "Player should have pending ocean tile selection")

	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	removalSelection := p1.Selection().GetPendingResourceRemovalSelection()
	testutil.AssertTrue(t, removalSelection != nil, "Player should have pending removal target selection")
	testutil.AssertEqual(t, 4, removalSelection.Amount, "Removal amount should be 4")
	testutil.AssertTrue(t, testutil.ContainsHex(removalSelection.EligiblePlayerIDs, p2ID),
		"P2 should be in eligible removal targets")
	testutil.AssertTrue(t, !testutil.ContainsHex(removalSelection.EligiblePlayerIDs, p1ID),
		"P1 (self) should not be in eligible removal targets")
}

func TestFlooding_NoAdjacentOwnedTiles_NoPrompt(t *testing.T) {
	ctx := context.Background()
	testGame, repo, _, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	p1ID := playerIDs[0]

	p1, _ := testGame.GetPlayer(p1ID)
	testutil.SetPlayerCredits(ctx, p1, 100)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)

	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanIsolated)

	removalSelection := p1.Selection().GetPendingResourceRemovalSelection()
	testutil.AssertTrue(t, removalSelection == nil, "No removal target selection expected when no adjacent owned tiles")
}

func TestFlooding_MultipleAdjacentOpponents_AllEligible(t *testing.T) {
	ctx := context.Background()
	testGame, repo, _, playerIDs := testutil.SetupMultiPlayerGame(t, 3)
	p1ID := playerIDs[0]
	p2ID := playerIDs[1]
	p3ID := playerIDs[2]

	for _, pid := range playerIDs {
		p, _ := testGame.GetPlayer(pid)
		testutil.SetPlayerCredits(ctx, p, 100)
	}

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p2ID, "city", floodingLandHexA)
	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p3ID, "city", floodingLandHexB)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)
	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	p1, _ := testGame.GetPlayer(p1ID)
	removalSelection := p1.Selection().GetPendingResourceRemovalSelection()
	testutil.AssertTrue(t, removalSelection != nil, "Player should have pending removal target selection")
	testutil.AssertTrue(t, testutil.ContainsHex(removalSelection.EligiblePlayerIDs, p2ID), "P2 should be eligible")
	testutil.AssertTrue(t, testutil.ContainsHex(removalSelection.EligiblePlayerIDs, p3ID), "P3 should be eligible")
	testutil.AssertTrue(t, !testutil.ContainsHex(removalSelection.EligiblePlayerIDs, p1ID), "P1 should not be eligible")
}

func TestFlooding_ConfirmRemoval_CreditsDestroyed(t *testing.T) {
	ctx := context.Background()
	testGame, repo, cardRegistry, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()
	p1ID := playerIDs[0]
	p2ID := playerIDs[1]

	p1, _ := testGame.GetPlayer(p1ID)
	p2, _ := testGame.GetPlayer(p2ID)
	testutil.SetPlayerCredits(ctx, p1, 100)
	testutil.SetPlayerCredits(ctx, p2, 50)

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p2ID, "city", floodingLandHexA)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)
	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	p1CreditsBefore := testutil.GetPlayerCredits(p1)
	p2CreditsBefore := testutil.GetPlayerCredits(p2)

	confirmRemoval := confirmAction.NewConfirmResourceRemovalAction(repo, cardRegistry, stateRepo, logger)
	err = confirmRemoval.Execute(ctx, testGame.ID(), p1ID, p1.Selection().GetPendingResourceRemovalSelection().ID, p2ID, 4)
	testutil.AssertNoError(t, err, "Confirm removal should succeed")

	testutil.AssertEqual(t, p1CreditsBefore, testutil.GetPlayerCredits(p1), "Removing credits must not reward P1")
	testutil.AssertEqual(t, p2CreditsBefore-4, testutil.GetPlayerCredits(p2), "P2 should lose 4 credits")

	testutil.AssertTrue(t, p1.Selection().GetPendingResourceRemovalSelection() == nil,
		"Pending removal selection should be cleared after confirm")
}

func TestFlooding_PartialRemoval_TargetHasLessThan4Credits(t *testing.T) {
	ctx := context.Background()
	testGame, repo, cardRegistry, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()
	p1ID := playerIDs[0]
	p2ID := playerIDs[1]

	p1, _ := testGame.GetPlayer(p1ID)
	p2, _ := testGame.GetPlayer(p2ID)
	testutil.SetPlayerCredits(ctx, p1, 100)
	testutil.SetPlayerCredits(ctx, p2, 2)

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p2ID, "city", floodingLandHexA)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)
	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	p1CreditsBefore := testutil.GetPlayerCredits(p1)

	confirmRemoval := confirmAction.NewConfirmResourceRemovalAction(repo, cardRegistry, stateRepo, logger)
	err = confirmRemoval.Execute(ctx, testGame.ID(), p1ID, p1.Selection().GetPendingResourceRemovalSelection().ID, p2ID, 2)
	testutil.AssertNoError(t, err, "Confirm removal should succeed")

	testutil.AssertEqual(t, p1CreditsBefore, testutil.GetPlayerCredits(p1), "Removing credits must not reward P1")
	testutil.AssertEqual(t, 0, testutil.GetPlayerCredits(p2), "P2 should have 0 credits")
}

func TestFlooding_SkipRemoval_NoCreditsChange(t *testing.T) {
	ctx := context.Background()
	testGame, repo, cardRegistry, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()
	p1ID := playerIDs[0]
	p2ID := playerIDs[1]

	p1, _ := testGame.GetPlayer(p1ID)
	p2, _ := testGame.GetPlayer(p2ID)
	testutil.SetPlayerCredits(ctx, p1, 100)
	testutil.SetPlayerCredits(ctx, p2, 50)

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p2ID, "city", floodingLandHexA)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)
	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	p1CreditsBefore := testutil.GetPlayerCredits(p1)
	p2CreditsBefore := testutil.GetPlayerCredits(p2)

	confirmRemoval := confirmAction.NewConfirmResourceRemovalAction(repo, cardRegistry, stateRepo, logger)
	err = confirmRemoval.Execute(ctx, testGame.ID(), p1ID, p1.Selection().GetPendingResourceRemovalSelection().ID, "", 0)
	testutil.AssertNoError(t, err, "Skip removal should succeed")

	testutil.AssertEqual(t, p1CreditsBefore, testutil.GetPlayerCredits(p1), "P1 credits should be unchanged")
	testutil.AssertEqual(t, p2CreditsBefore, testutil.GetPlayerCredits(p2), "P2 credits should be unchanged")

	testutil.AssertTrue(t, p1.Selection().GetPendingResourceRemovalSelection() == nil,
		"Pending removal selection should be cleared after skip")
}

func TestFlooding_OwnTilesEligible(t *testing.T) {
	ctx := context.Background()
	testGame, repo, _, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	p1ID := playerIDs[0]

	p1, _ := testGame.GetPlayer(p1ID)
	testutil.SetPlayerCredits(ctx, p1, 100)

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p1ID, "city", floodingLandHexA)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)
	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	removalSelection := p1.Selection().GetPendingResourceRemovalSelection()
	testutil.AssertTrue(t, removalSelection != nil,
		"Own adjacent tiles permit optional self targeting")
}

func TestFlooding_InvalidTargetPlayer_Rejected(t *testing.T) {
	ctx := context.Background()
	testGame, repo, cardRegistry, playerIDs := testutil.SetupMultiPlayerGame(t, 2)
	logger := testutil.TestLogger()
	stateRepo := game.NewInMemoryGameStateRepository()
	p1ID := playerIDs[0]
	p2ID := playerIDs[1]

	p1, _ := testGame.GetPlayer(p1ID)
	testutil.SetPlayerCredits(ctx, p1, 100)
	p2, _ := testGame.GetPlayer(p2ID)
	testutil.SetPlayerCredits(ctx, p2, 100)

	testutil.PlaceTileForPlayer(ctx, t, testGame, repo, p2ID, "city", floodingLandHexA)

	err := testGame.SetCurrentTurn(ctx, p1ID, 2)
	testutil.AssertNoError(t, err, "Failed to set turn")
	playFloodingCard(ctx, t, testGame, repo, p1ID)
	selectFloodingOceanTile(ctx, t, testGame, repo, p1ID, floodingOceanHexA)

	confirmRemoval := confirmAction.NewConfirmResourceRemovalAction(repo, cardRegistry, stateRepo, logger)
	err = confirmRemoval.Execute(ctx, testGame.ID(), p1ID, p1.Selection().GetPendingResourceRemovalSelection().ID, "nonexistent-player", 4)
	testutil.AssertTrue(t, err != nil, "Should fail when targeting ineligible player")
}

func TestSearchForLife_RevealsAndDiscardsBeforeReward(t *testing.T) {
	for _, tc := range []struct {
		name    string
		top     string
		science int
	}{
		{"microbe", "Tardigrades", 1}, {"nonmatching", "Mine", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			card := testutil.GetCardByName("Search For Life")
			p.Hand().AddCard(card.ID)
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play Search For Life")
			g.InitDeck([]string{testutil.CardID(tc.top)}, nil, nil)
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 1), "last action")
			before := p.Resources().Get().Credits
			hand := p.Hand().CardCount()
			stateRepo := game.NewInMemoryGameStateRepository()
			use := cardAction.NewUseCardActionAction(repo, registry, stateRepo, testutil.TestLogger())
			testutil.AssertNoError(t, use.Execute(context.Background(), g.ID(), id, card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), "use Search For Life")
			testutil.AssertEqual(t, before-1, p.Resources().Get().Credits, "pay once")
			testutil.AssertEqual(t, tc.science, p.Resources().GetCardStorage(card.ID), "reward only a matching card")
			testutil.AssertEqual(t, hand, p.Hand().CardCount(), "revealed card never enters hand")
			testutil.AssertEqual(t, 0, g.Deck().GetAvailableCardCount(), "remove top card")
			testutil.AssertEqual(t, 1, len(g.Deck().DiscardPile()), "discard revealed card")
			testutil.AssertEqual(t, id, g.CurrentTurn().PlayerID(), "hold turn until acknowledgement")
			pending := p.Selection().GetPendingCardReveal()
			if pending == nil {
				t.Fatal("missing reveal acknowledgement")
			}
			testutil.AssertEqual(t, testutil.CardID(tc.top), pending.Cards[0].CardID, "revealed identity")
			testutil.AssertEqual(t, tc.science > 0, pending.Cards[0].Matched, "match result")
			snapshot := dto.ToGameDto(g, registry, id)
			if snapshot.CurrentPlayer.PendingCardReveal == nil {
				t.Fatal("reconnect DTO must retain reveal")
			}
			testutil.AssertErrorContains(t, baseaction.ValidateNoPendingSelections(g, id, testutil.TestLogger()), "pending selection", "pending reveal locks other actions")
			diffs, err := stateRepo.GetDiff(context.Background(), g.ID())
			testutil.AssertNoError(t, err, "get reveal log")
			if len(diffs) == 0 {
				t.Fatal("missing reveal log")
			}
			diff := &diffs[len(diffs)-1]
			if diff.DisplayData == nil || len(diff.DisplayData.RevealedCards) != 1 {
				t.Fatal("public log must contain revealed identity")
			}
			testutil.AssertEqual(t, testutil.CardID(tc.top), dto.ToStateDiffDto(diff).DisplayData.RevealedCards[0].CardID, "public log DTO")
			confirm := confirmAction.NewConfirmCardRevealAction(repo)
			testutil.AssertErrorContains(t, confirm.Execute(context.Background(), g.ID(), opponentID), "not your turn", "opponent cannot confirm")
			testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id), "acknowledge reveal")
			testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "advance after acknowledgement")
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 2), "restore owner turn")
			testutil.AssertErrorContains(t, confirm.Execute(context.Background(), g.ID(), id), "no pending card reveal", "cannot acknowledge twice")
			testutil.AssertEqual(t, tc.science, p.Resources().GetCardStorage(card.ID), "confirmation never rewards again")
			testutil.AssertEqual(t, before-1, p.Resources().Get().Credits, "confirmation never charges again")
			testutil.AssertErrorContains(t, use.Execute(context.Background(), g.ID(), id, card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), "action already played this generation", "cannot reuse this generation")

		})
	}
}

func TestSearchForLife_RejectedActionsDoNotMutate(t *testing.T) {
	for _, emptyDeck := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty-deck-%v", emptyDeck), func(t *testing.T) {
			g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			card := testutil.GetCardByName("Search For Life")
			p.Hand().AddCard(card.ID)
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play")
			g.InitDeck([]string{testutil.CardID("Tardigrades")}, nil, nil)
			if emptyDeck {
				g.InitDeck(nil, nil, nil)
			} else {
				testutil.SetPlayerCredits(context.Background(), p, 0)
			}
			credits, drawn, actions := p.Resources().Get().Credits, g.Deck().DrawnCardCount(), g.CurrentTurn().ActionsRemaining()
			use := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger())
			wantErr := "payment exceeds available source pool"
			if emptyDeck {
				wantErr = "not enough cards available to reveal"
			}
			testutil.AssertErrorContains(t, use.Execute(context.Background(), g.ID(), id, card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil), wantErr, "reject unavailable action")
			testutil.AssertEqual(t, credits, p.Resources().Get().Credits, "no charge")
			testutil.AssertEqual(t, drawn, g.Deck().DrawnCardCount(), "no draw")
			testutil.AssertEqual(t, actions, g.CurrentTurn().ActionsRemaining(), "no action consumed")
			testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(card.ID), "no reward")
			if p.Selection().GetPendingCardReveal() != nil {
				t.Fatal("rejected action created pending reveal")
			}
		})
	}
}

func checkRequirementStepBoundaries(t *testing.T, sources []string, allowedSteps [4]int) {
	t.Helper()
	tracks := []struct {
		name          string
		kind          gamecards.RequirementType
		current, step int
	}{
		{"temperature", gamecards.RequirementTemperature, -10, 2},
		{"oxygen", gamecards.RequirementOxygen, 7, 1},
		{"ocean", gamecards.RequirementOceans, 4, 1},
		{"venus", gamecards.RequirementVenus, 14, 2},
	}
	for i, track := range tracks {
		for _, maximum := range []bool{false, true} {
			for _, gap := range []int{allowedSteps[i], allowedSteps[i] + 1} {
				t.Run(fmt.Sprintf("%s/max=%t/gap=%d", track.name, maximum, gap), func(t *testing.T) {
					ctx := context.Background()
					requirement := gamecards.Requirement{Type: track.kind}
					boundary := track.current + gap*track.step
					if maximum {
						boundary = track.current - gap*track.step
						requirement.Max = &boundary
					} else {
						requirement.Min = &boundary
					}
					target := gamecards.Card{ID: "step-boundary", Name: "Step boundary", Type: gamecards.CardTypeAutomated, Cost: 5,
						Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{requirement}}}
					registry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{target})
					g, repo := testutil.CreateTestGameWithPlayers(t, 1)
					p := g.GetAllPlayers()[0]
					p.SetCorporationID(testutil.CardID("Tharsis Republic"))
					testutil.AssertNoError(t, g.UpdateStatus(ctx, shared.GameStatusActive), "Activate game")
					testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseAction), "Action phase")
					testutil.AssertNoError(t, g.SetCurrentTurn(ctx, p.ID(), 20), "Set turn")
					play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
					for _, name := range sources {
						source := testutil.GetCardByName(name)
						if source.Type == gamecards.CardTypeCorporation {
							testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), p.ID(), source.ID), "Set modifier corporation")
						} else {
							p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
							p.Hand().AddCard(source.ID)
							testutil.AssertNoError(t, play.Execute(ctx, g.ID(), p.ID(), source.ID, shared.NativePayment(shared.ResourceCredit, source.Cost), nil, nil, nil, nil, nil), "Play modifier card")
						}
					}
					p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
					_, err := g.GlobalParameters().IncreaseTemperature(ctx, 10, "")
					testutil.AssertNoError(t, err, "Set temperature")
					_, err = g.GlobalParameters().IncreaseOxygen(ctx, 7, "")
					testutil.AssertNoError(t, err, "Set oxygen")
					testutil.AssertNoError(t, g.GlobalParameters().SetOceans(ctx, 4), "Set oceans")
					_, err = g.GlobalParameters().IncreaseVenus(ctx, 7, "")
					testutil.AssertNoError(t, err, "Set Venus")
					p.Hand().AddCard(target.ID)
					testutil.AssertNoError(t, g.SetCurrentTurn(ctx, p.ID(), 2), "Set target turn")
					state := baseaction.CalculatePlayerCardState(&target, p, g, registry)
					allowed := gap == allowedSteps[i]
					if (len(state.Errors) == 0) != allowed {
						t.Errorf("availability allowed=%t, want %t: %+v", len(state.Errors) == 0, allowed, state.Errors)
					}
					before := p.Resources().Get()
					effectsBefore := len(p.Effects().List())
					err = play.Execute(ctx, g.ID(), p.ID(), target.ID, shared.NativePayment(shared.ResourceCredit, target.Cost), nil, nil, nil, nil, nil)
					if allowed {
						testutil.AssertNoError(t, err, "Play at exact allowance boundary")
						testutil.AssertTrue(t, p.PlayedCards().Contains(target.ID), "Target must be played")
					} else {
						testutil.AssertErrorContains(t, err, "requirement not met", "Reject one step outside allowance")
						testutil.AssertEqual(t, before, p.Resources().Get(), "Rejected play must not charge")
						testutil.AssertTrue(t, p.Hand().HasCard(target.ID), "Rejected card stays in hand")
						testutil.AssertEqual(t, effectsBefore, len(p.Effects().List()), "Rejected play preserves modifiers")
					}
				})
			}
		}
	}
}

func TestSpecialDesign_RequirementStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Special Design"}, [4]int{2, 2, 2, 2})
}

func TestAdaptationTechnology_RequirementStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Adaptation Technology"}, [4]int{2, 2, 2, 2})
}

func TestAdaptationTechnology_StacksWithSpecialDesignAtStepBoundaries(t *testing.T) {
	checkRequirementStepBoundaries(t, []string{"Adaptation Technology", "Special Design"}, [4]int{4, 4, 4, 4})
}

func TestSpecialDesign_LifetimePreservesPermanentModifiers(t *testing.T) {
	for _, mode := range []string{"requirement", "no-requirement", "generation-end"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			minimum := -22
			target := gamecards.Card{ID: "lifetime-target", Name: "Lifetime target", Type: gamecards.CardTypeAutomated, Cost: 5}
			if mode == "requirement" {
				target.Requirements = &gamecards.CardRequirements{Items: []gamecards.Requirement{{Type: gamecards.RequirementTemperature, Min: &minimum}}}
			}
			registry := testutil.CreateTestCardRegistryWithAdditionalCards([]gamecards.Card{target})
			g, repo := testutil.CreateTestGameWithPlayers(t, 1)
			p := g.GetAllPlayers()[0]
			testutil.AssertNoError(t, g.UpdateStatus(ctx, shared.GameStatusActive), "Activate game")
			testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseAction), "Action phase")
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, p.ID(), 10), "Set turn")
			testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), p.ID(), testutil.CardID("Inventrix")), "Set Inventrix")
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			special := testutil.GetCardByName("Special Design")
			p.Hand().AddCard(special.ID)
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), p.ID(), special.ID, shared.NativePayment(shared.ResourceCredit, special.Cost), nil, nil, nil, nil, nil), "Play Special Design")
			if mode == "generation-end" {
				testutil.AssertNoError(t, g.AdvanceGeneration(ctx), "Advance generation")
			} else {
				p.Hand().AddCard(target.ID)
				testutil.AssertNoError(t, play.Execute(ctx, g.ID(), p.ID(), target.ID, shared.NativePayment(shared.ResourceCredit, target.Cost), nil, nil, nil, nil, nil), "Consume Special Design")
			}
			for _, effect := range p.Effects().List() {
				if effect.CardID == special.ID {
					t.Error("Special Design must have expired")
				}
			}
			probe := gamecards.Card{ID: "probe", Requirements: &gamecards.CardRequirements{Items: []gamecards.Requirement{{Type: gamecards.RequirementTemperature, Min: testutil.IntPtr(-26)}}}}
			testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseAction), "Restore action phase")
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, p.ID(), 2), "Restore turn")
			p.Hand().AddCard(probe.ID)
			state := baseaction.CalculatePlayerCardState(&probe, p, g, registry)
			testutil.AssertEqual(t, 0, len(state.Errors), "Permanent Inventrix still allows two temperature steps")
			probe.Requirements.Items[0].Min = testutil.IntPtr(-24)
			state = baseaction.CalculatePlayerCardState(&probe, p, g, registry)
			testutil.AssertTrue(t, len(state.Errors) > 0, "Expired Special Design no longer extends Inventrix")
		})
	}
}

func TestEcologicalZone_RewardsEachTagExactlyOnce(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
	log := testutil.TestLogger()
	p, _ := g.GetPlayer(id)
	opponent, _ := g.GetPlayer(opponentID)
	testutil.AssertNoError(t, admin.NewSetCorporationAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, testutil.CardID("Arklight")), "set Arklight")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	testutil.AssertNoError(t, g.GlobalParameters().SetOxygen(ctx, 3), "oxygen")
	testutil.PlaceTileForPlayer(ctx, t, g, repo, id, "greenery", testutil.FindUnoccupiedLandHex(t, g))
	play := cardAction.NewPlayCardAction(repo, registry, nil, log)
	for _, name := range []string{"Decomposers", "Ecological Zone"} {
		c := testutil.GetCardByName(name)
		p.Hand().AddCard(c.ID)
		testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "play "+name)
	}
	ecoID, decomposerID, arkID := testutil.CardID("Ecological Zone"), testutil.CardID("Decomposers"), testutil.CardID("Arklight")
	testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(ecoID), "Ecological Zone includes both own tags exactly once")
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(decomposerID), "Decomposers includes own tag and both Ecological Zone tags")
	testutil.AssertEqual(t, 3, p.Resources().GetCardStorage(arkID), "Arklight includes own tag and both Ecological Zone tags")
	selection := g.GetPendingTileSelection(id)
	testutil.AssertTrue(t, selection != nil, "Ecological Zone still places a tile")
	_, err := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), log).Execute(ctx, g.ID(), id, selection.AvailableHexes[0])
	testutil.AssertNoError(t, err, "place Ecological Zone")
	// Advanced Ecosystems rewards animal/plant effects twice and Decomposers three times.
	advanced := testutil.GetCardByName("Advanced Ecosystems")
	tags := make([]string, len(advanced.Tags))
	for i, tag := range advanced.Tags {
		tags[i] = string(tag)
	}
	p.Hand().AddCard(advanced.ID)
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "next turn")
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, advanced.ID, shared.NativePayment(shared.ResourceCredit, advanced.Cost), nil, nil, nil, nil, nil), "play Advanced Ecosystems")
	testutil.AssertEqual(t, 4, p.Resources().GetCardStorage(ecoID), "two later animal/plant tags")
	testutil.AssertEqual(t, 6, p.Resources().GetCardStorage(decomposerID), "all three later ecosystem tags")
	testutil.AssertEqual(t, 5, p.Resources().GetCardStorage(arkID), "two later animal/plant tags")
	opponent.PlayedCards().AddCard(advanced.ID, advanced.Name, string(advanced.Type), tags)
	p.PlayedCards().AddCard("science-only", "Science only", "automated", []string{"science"})
	testutil.AssertEqual(t, 4, p.Resources().GetCardStorage(ecoID), "ignore opponent and unrelated tags")
	testutil.AssertEqual(t, 6, p.Resources().GetCardStorage(decomposerID), "ignore opponent and unrelated tags")
	testutil.AssertEqual(t, 5, p.Resources().GetCardStorage(arkID), "ignore opponent and unrelated tags")
}

func TestHerbivores_GreeneryPlacement(t *testing.T) {
	for _, tc := range []struct {
		name   string
		oxygen int
		final  bool
	}{
		{"normal", 8, false}, {"maximum oxygen", 14, false}, {"final greenery", 14, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
			log := testutil.TestLogger()
			p, _ := g.GetPlayer(id)
			opponent, _ := g.GetPlayer(opponentID)
			p.Resources().Set(shared.Resources{Credits: 100, Plants: 16})
			opponent.Resources().AddProduction(map[shared.ResourceType]int{shared.ResourcePlantProduction: 1})
			testutil.AssertNoError(t, g.GlobalParameters().SetOxygen(ctx, tc.oxygen), "oxygen")
			c := testutil.GetCardByName("Herbivores")
			p.Hand().AddCard(c.ID)
			testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, &opponentID, nil, nil), "play Herbivores")
			testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(c.ID), "initial animal")
			testutil.PlaceTileForPlayer(ctx, t, g, repo, opponentID, "greenery", testutil.FindUnoccupiedLandHex(t, g))
			testutil.PlaceTileForPlayer(ctx, t, g, repo, id, "city", testutil.FindUnoccupiedLandHex(t, g))
			testutil.PlaceTileForPlayer(ctx, t, g, repo, id, "ocean", testutil.FindUnoccupiedOceanHex(t, g))
			testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(c.ID), "ignore opponent greenery, cities and oceans")
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "turn")
			if tc.final {
				testutil.AssertNoError(t, g.UpdatePhase(ctx, shared.GamePhaseFinalPhase), "final phase")
			}
			testutil.AssertNoError(t, resconv.NewConvertPlantsToGreeneryAction(repo, registry, nil, log).Execute(ctx, g.ID(), id, shared.NativePayment(shared.ResourcePlant, 8)), "convert plants")
			selection := g.GetPendingTileSelection(id)
			_, err := tileAction.NewSelectTileAction(repo, registry, game.NewInMemoryGameStateRepository(), log).Execute(ctx, g.ID(), id, selection.AvailableHexes[0])
			testutil.AssertNoError(t, err, "place owned greenery")
			testutil.AssertEqual(t, 2, p.Resources().GetCardStorage(c.ID), "exactly one animal per owned greenery, even without oxygen increase")
		})
	}
}

func TestFlooding_OptionalRemovalEdges(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		self, claim, maxOceans bool
		credits, remove        int
	}{
		{name: "self partial", self: true, credits: 100, remove: 2},
		{name: "opponent partial", credits: 20, remove: 1},
		{name: "empty claim", claim: true, credits: 20},
		{name: "no credits"},
		{name: "oceans exhausted", maxOceans: true, credits: 20},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			targetID := otherID
			if tc.self {
				targetID = id
			}
			target, _ := g.GetPlayer(targetID)
			p.Resources().Set(shared.Resources{Credits: 100})
			target.Resources().Set(shared.Resources{Credits: tc.credits})
			if tc.claim {
				testutil.AssertNoError(t, g.Board().ReserveTile(ctx, shared.HexPosition{Q: 3, R: -1, S: -2}, targetID), "reserve adjacent land")
			} else {
				testutil.PlaceTileForPlayer(ctx, t, g, repo, targetID, "city", floodingLandHexA)
			}
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 2), "turn")
			if tc.maxOceans {
				testutil.AssertNoError(t, g.GlobalParameters().SetOceans(ctx, 9), "max oceans")
			}
			playFloodingCard(ctx, t, g, repo, id)
			if !tc.maxOceans {
				selectFloodingOceanTile(ctx, t, g, repo, id, floodingOceanHexA)
			}
			pending := p.Selection().GetPendingResourceRemovalSelection()
			if tc.remove == 0 {
				testutil.AssertTrue(t, pending == nil, "no actionable removal")
				return
			}
			testutil.AssertTrue(t, pending != nil, "pending removal")
			before := target.Resources().Get().Credits
			testutil.AssertNoError(t, confirmAction.NewConfirmResourceRemovalAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, pending.ID, targetID, tc.remove), "chosen amount")
			testutil.AssertEqual(t, before-tc.remove, target.Resources().Get().Credits, "exact partial removal")
		})
	}
}

// Real-card regression: failed attacks must not consume payment or the card.
func TestPets_BlocksVirusFromEitherPlayer(t *testing.T) {
	for _, own := range []bool{false, true} {
		t.Run(fmt.Sprint(own), func(t *testing.T) {
			g, repo, registry, ownerID, opponentID := testutil.SetupTwoPlayerGame(t)
			ctx := context.Background()
			owner, _ := g.GetPlayer(ownerID)
			pets := testutil.GetCardByName("Pets")
			owner.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			owner.Hand().AddCard(pets.ID)
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), ownerID, pets.ID, shared.NativePayment(shared.ResourceCredit, pets.Cost), nil, nil, nil, nil, nil), "play Pets")
			actorID := opponentID
			if own {
				actorID = ownerID
			}
			actor, _ := g.GetPlayer(actorID)
			virus := testutil.GetCardByName("Virus")
			actor.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
			actor.Hand().AddCard(virus.ID)
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, actorID, 2), "turn")
			before := actor.Resources().Get()
			choice := 0
			err := play.Execute(ctx, g.ID(), actorID, virus.ID, shared.NativePayment(shared.ResourceCredit, virus.Cost), &choice, []string{pets.ID}, nil, nil, nil)
			if err == nil {
				t.Fatal("Virus must reject protected Pets")
			}
			testutil.AssertTrue(t, actor.Hand().HasCard(virus.ID), "rejected card remains in hand")
			testutil.AssertEqual(t, before, actor.Resources().Get(), "payment unchanged")
			testutil.AssertEqual(t, 1, owner.Resources().GetCardStorage(pets.ID), "Pets animal retained")
		})
	}
}

func TestPets_BlocksSpendingAndTransfersAcrossGenerations(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, ownerID, otherID := testutil.SetupTwoPlayerGame(t)
	owner, _ := g.GetPlayer(ownerID)
	pets := testutil.GetCardByName("Pets")
	owner.Hand().AddCard(pets.ID)
	owner.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, play.Execute(ctx, g.ID(), ownerID, pets.ID, shared.NativePayment(shared.ResourceCredit, pets.Cost), nil, nil, nil, nil, nil), "play Pets")
	owner.Effects().RemoveTemporaryEffects(shared.TemporaryGenerationEnd)
	owner.Effects().RemoveTemporaryEffects(shared.TemporaryNextCard)
	applier := gamecards.NewBehaviorApplier(owner, g, "Pets", testutil.TestLogger()).WithSourceCardID(pets.ID).WithCardRegistry(registry)
	testutil.AssertErrorContains(t, applier.ApplyInputs(ctx, []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"), shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card")}), "animal are protected", "cannot spend Pets")
	testutil.AssertEqual(t, 90, owner.Resources().Get().Credits, "other input not spent")
	testutil.AssertErrorContains(t, applier.ApplyOutputs(ctx, []shared.BehaviorCondition{shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"), shared.NewCardStorageCondition(shared.ResourceAnimal, -1, "self-card")}), "card resources are protected", "cannot remove Pets")
	testutil.AssertEqual(t, 90, owner.Resources().Get().Credits, "other output not applied")
	owner.Resources().AddPaymentSubstitute(shared.PaymentSubstitute{Source: shared.PaymentSource{Target: "self-card", CardID: pets.ID, Resource: shared.ResourceAnimal}, ConversionRate: 1, TargetResource: shared.ResourceCredit})
	virus := testutil.GetCardByName("Virus")
	owner.Hand().AddCard(virus.ID)
	choice := 1
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, ownerID, 2), "turn")
	testutil.AssertErrorContains(t, play.Execute(ctx, g.ID(), ownerID, virus.ID, shared.Payment{Allocations: []shared.PaymentAllocation{{Source: shared.PaymentSource{Target: "self-card", Resource: "animal", CardID: pets.ID},
		TargetResource: shared.ResourceCredit, Amount: 1}},
	}, &choice, nil, nil, nil, nil), "ineligible payment source for credit", "Pets cannot pay")
	testutil.AssertTrue(t, owner.Hand().HasCard(virus.ID), "payment rejection retains card")
	for _, actorID := range []string{ownerID, otherID} {
		actor, _ := g.GetPlayer(actorID)
		transfer := gamecards.NewBehaviorApplier(actor, g, "Predators", testutil.TestLogger()).WithSourceCardID(testutil.CardID("Predators")).WithStealSourceCardID(pets.ID).WithCardRegistry(registry)
		testutil.AssertErrorContains(t, transfer.ApplyOutputs(ctx, []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "steal-from-any-card")}), "are protected", "Pets cannot transfer")
	}
	testutil.AssertEqual(t, 1, owner.Resources().GetCardStorage(pets.ID), "animal retained")
	testutil.PlaceTileForPlayer(ctx, t, g, repo, otherID, "city", floodingLandHexA)
	testutil.AssertEqual(t, 2, owner.Resources().GetCardStorage(pets.ID), "city still adds an animal once")

}

func TestVirus_RemovesFromActualOwnerAndCanSkip(t *testing.T) {
	for _, skip := range []bool{false, true} {
		t.Run(fmt.Sprint(skip), func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, actorID, ownerID := testutil.SetupTwoPlayerGame(t)
			actor, _ := g.GetPlayer(actorID)
			owner, _ := g.GetPlayer(ownerID)
			birds := testutil.GetCardByName("Birds")
			owner.PlayedCards().AddCard(birds.ID, birds.Name, string(birds.Type), nil)
			owner.Resources().AddToStorage(birds.ID, 1)
			virus := testutil.GetCardByName("Virus")
			actor.Hand().AddCard(virus.ID)
			actor.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 10})
			choice := 0
			targets := []string{birds.ID}
			if skip {
				targets = nil
			}
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), actorID, virus.ID, shared.NativePayment(shared.ResourceCredit, virus.Cost), &choice, targets, nil, nil, nil)
			testutil.AssertNoError(t, err, "Virus play")
			expected := 0
			if skip {
				expected = 1
			}
			testutil.AssertEqual(t, expected, owner.Resources().GetCardStorage(birds.ID), "actual owner storage")
			testutil.AssertEqual(t, 0, actor.Resources().GetCardStorage(birds.ID), "actor does not acquire target storage")
		})
	}
}

func TestArtificialLake_OffersLandOnly(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Artificial Lake")
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	p.Hand().AddCard(card.ID)
	testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, -6), "temperature")
	err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "play Artificial Lake")
	pending := g.GetPendingTileSelection(id)
	if pending == nil || len(pending.AvailableHexes) == 0 {
		t.Fatal("Artificial Lake needs a land placement")
	}
	for _, hex := range pending.AvailableHexes {
		for _, tile := range g.Board().Tiles() {
			if tile.Coordinates.String() == hex {
				testutil.AssertEqual(t, shared.ResourceLandTile, tile.Type, "Artificial Lake must use land")
			}
		}
	}
}

func TestArtificialLake_CompletionUsesNormalOceanRewards(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Artificial Lake")
	p.Hand().AddCard(card.ID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, -6), "temperature")
	testutil.AssertNoError(t, cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play")
	pending := g.GetPendingTileSelection(id)
	if pending == nil {
		t.Fatal("missing placement")
	}
	var selected shared.HexPosition
	found := false
	expected := map[shared.ResourceType]int{}
	for _, hex := range pending.AvailableHexes {
		for _, tile := range g.Board().Tiles() {
			if tile.Coordinates.String() != hex {
				continue
			}
			suitable := true
			for _, bonus := range tile.Bonuses {
				switch bonus.Type {
				case shared.ResourceCredit, shared.ResourcePlant, shared.ResourceSteel, shared.ResourceTitanium, shared.ResourceEnergy, shared.ResourceHeat:
				default:
					suitable = false
				}
			}
			if !suitable || len(tile.Bonuses) == 0 {
				continue
			}
			selected = tile.Coordinates
			for _, bonus := range tile.Bonuses {
				expected[bonus.Type] += bonus.Amount
			}
			found = true
			break
		}
		if found {
			break
		}
	}
	if !found {
		t.Fatal("fixture needs land with basic resource bonus")
	}
	// An adjacent existing ocean adds the ordinary adjacency bonus.
	for _, n := range selected.GetNeighbors() {
		tile, err := g.Board().GetTile(n)
		if err == nil && tile.Type == shared.ResourceOceanSpace && tile.OccupiedBy == nil {
			testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, n, board.TileOccupant{Type: shared.ResourceOceanTile}, ""), "existing ocean")
			testutil.AssertNoError(t, g.GlobalParameters().SetOceans(ctx, 1), "existing ocean count")
			expected[shared.ResourceCredit] += 2
			break
		}
	}
	before := p.Resources().Get()
	tr := p.Resources().TerraformRating()
	oceans := g.GlobalParameters().Oceans()
	result, err := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, selected.String())
	testutil.AssertNoError(t, err, "place ocean on land")
	testutil.AssertEqual(t, true, result.OceanPlaced, "ordinary ocean completion")
	testutil.AssertEqual(t, oceans+1, g.GlobalParameters().Oceans(), "ocean count")
	testutil.AssertEqual(t, tr+1, p.Resources().TerraformRating(), "TR exactly once")
	for _, rt := range []shared.ResourceType{shared.ResourceCredit, shared.ResourcePlant, shared.ResourceSteel, shared.ResourceTitanium, shared.ResourceEnergy, shared.ResourceHeat} {
		testutil.AssertEqual(t, before.GetAmount(rt)+expected[rt], p.Resources().Get().GetAmount(rt), "placement bonus")
	}
	tile, err := g.Board().GetTile(selected)
	testutil.AssertNoError(t, err, "tile")
	testutil.AssertEqual(t, shared.ResourceLandTile, tile.Type, "underlying area remains land")
	testutil.AssertEqual(t, shared.ResourceOceanTile, tile.OccupiedBy.Type, "occupant is ocean")
}

func TestArtificialLake_NoLandVersusOceanMaximum(t *testing.T) {
	for _, capped := range []bool{false, true} {
		t.Run(fmt.Sprint(capped), func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			card := testutil.GetCardByName("Artificial Lake")
			p.Hand().AddCard(card.ID)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			testutil.AssertNoError(t, g.GlobalParameters().SetTemperature(ctx, -6), "temperature")
			for _, tile := range g.Board().Tiles() {
				if tile.Type == shared.ResourceLandTile {
					testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, tile.Coordinates, board.TileOccupant{Type: shared.ResourceGreeneryTile}, otherID), "fill land")
				}
			}
			if capped {
				g.GlobalParameters().ReduceMaxOceans(8)
				testutil.AssertNoError(t, g.GlobalParameters().SetOceans(ctx, g.GlobalParameters().GetMaxOceans()), "cap oceans")
			}
			tr := p.Resources().TerraformRating()
			err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
			if capped {
				testutil.AssertNoError(t, err, "capped oceans can be skipped")
				testutil.AssertEqual(t, 85, p.Resources().Get().Credits, "pay card only")
				vp := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, nil, nil)
				testutil.AssertEqual(t, 1, vp.CardVP, "retain fixed VP")
			} else {
				testutil.AssertErrorContains(t, err, "No ocean tiles remaining", "mandatory ocean has no legal land")
				testutil.AssertEqual(t, 100, p.Resources().Get().Credits, "no payment")
				testutil.AssertTrue(t, p.Hand().HasCard(card.ID), "card retained")
			}
			testutil.AssertTrue(t, g.GetPendingTileSelection(id) == nil, "no empty or extra ocean selection")
			testutil.AssertEqual(t, tr, p.Resources().TerraformRating(), "no TR without placement")
		})
	}
}

func assertWildTagRequirements(t *testing.T, name string, tags []shared.CardTag) {
	t.Helper()
	for _, tc := range []struct {
		name   string
		actual int
		wilds  int
		valid  bool
	}{
		{"one wild cannot cover three requirements", 0, 1, false},
		{"one wild cannot cover two missing requirements", 1, 1, false},
		{"one missing requirement", 2, 1, true},
		{"two wilds cover two missing requirements", 1, 2, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, id, _ := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			fixtures := []gamecards.Card{}
			for i := 0; i < tc.actual; i++ {
				c := gamecards.Card{ID: fmt.Sprintf("tag-%d", i), Name: "Tag fixture", Type: gamecards.CardTypeAutomated, Tags: []shared.CardTag{tags[i]}}
				fixtures = append(fixtures, c)
				p.PlayedCards().AddCard(c.ID, c.Name, string(c.Type), nil)
			}
			registry := testutil.CreateTestCardRegistryWithAdditionalCards(fixtures)
			for _, wildName := range []string{"Research Coordination", "Research Network"}[:tc.wilds] {
				c := testutil.GetCardByName(wildName)
				p.PlayedCards().AddCard(c.ID, c.Name, string(c.Type), nil)
			}
			card := testutil.GetCardByName(name)
			p.Hand().AddCard(card.ID)
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			g.InitDeck([]string{testutil.CardID("Sponsors"), testutil.CardID("Tardigrades")}, nil, nil)
			state := baseaction.CalculatePlayerCardState(&card, p, g, registry)
			testutil.AssertEqual(t, tc.valid, state.Available(), "availability")
			credits, hand, played, actions := p.Resources().Get().Credits, p.Hand().CardCount(), len(p.PlayedCards().Cards()), g.CurrentTurn().ActionsRemaining()
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			err := play.Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
			testutil.AssertEqual(t, tc.valid, err == nil, fmt.Sprintf("execute: %v", err))
			if !tc.valid {
				testutil.AssertEqual(t, credits, p.Resources().Get().Credits, "no charge")
				testutil.AssertEqual(t, hand, p.Hand().CardCount(), "hand unchanged")
				testutil.AssertEqual(t, played, len(p.PlayedCards().Cards()), "played unchanged")
				testutil.AssertEqual(t, actions, g.CurrentTurn().ActionsRemaining(), "actions unchanged")
			}
		})
	}
}

func TestAdvancedEcosystems_WildTagRequirements(t *testing.T) {
	assertWildTagRequirements(t, "Advanced Ecosystems", []shared.CardTag{shared.TagPlant, shared.TagMicrobe, shared.TagAnimal})
}

func TestGeneRepair_CannotSupplyOwnRequirement(t *testing.T) {
	g, repo, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	for _, name := range []string{"Search For Life", "Research Coordination"} {
		card := testutil.GetCardByName(name)
		p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	}
	card := testutil.GetCardByName("Gene Repair")
	p.Hand().AddCard(card.ID)
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
	testutil.AssertFalse(t, baseaction.CalculatePlayerCardState(&card, p, g, registry).Available(), "one science and one wild are insufficient")
	play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertErrorContains(t, play.Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "tag requirements need 2 additional tags", "own tag cannot meet requirement")
	third := testutil.GetCardByName("Trans-Neptune Probe")
	p.PlayedCards().AddCard(third.ID, third.Name, string(third.Type), nil)
	testutil.AssertTrue(t, baseaction.CalculatePlayerCardState(&card, p, g, registry).Available(), "two science and one wild suffice")
	testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), id, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play with existing tags")
}

func TestGanymedeColony_WildTagsNeverGrantVP(t *testing.T) {
	g, _, registry, id, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	p.SetCorporationID("")
	colony := testutil.GetCardByName("Ganymede Colony")
	p.PlayedCards().AddCard(colony.ID, colony.Name, string(colony.Type), nil)
	for _, name := range []string{"Research Coordination", "Research Network"} {
		card := testutil.GetCardByName(name)
		p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	}
	breakdown := gamecards.CalculatePlayerVP(p, g, nil, nil, g.GetAllPlayers(), registry, testutil.CreateTestAwardRegistry(), testutil.CreateTestMilestoneRegistry())
	testutil.AssertEqual(t, 1, breakdown.CardVP, "only Ganymede Colony's own Jovian tag scores")
	for _, granter := range p.VPGranters().GetAll() {
		if granter.CardID == colony.ID {
			testutil.AssertEqual(t, 1, granter.ComputedValue, "live VP matches final scoring")
			return
		}
	}
	t.Fatal("missing Ganymede Colony VP granter")
}

// placeCityCountTestTiles provides two Mars cities and one off-Mars city,
// split across players, plus a non-city tile that must not contribute.
func placeCityCountTestTiles(t *testing.T, g *game.Game, playerID, opponentID string) {
	t.Helper()
	for _, placement := range []struct {
		location board.TileLocation
		owner    string
		kind     shared.ResourceType
	}{
		{board.TileLocationMars, playerID, shared.ResourceCityTile},
		{board.TileLocationMars, opponentID, shared.ResourceCityTile},
		{board.TileLocationPhobos, opponentID, shared.ResourceCityTile},
		{board.TileLocationMars, playerID, shared.ResourceGreeneryTile},
	} {
		placed := false
		for _, tile := range g.Board().Tiles() {
			if tile.Location == placement.location && tile.OccupiedBy == nil {
				testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(context.Background(), tile.Coordinates,
					board.TileOccupant{Type: placement.kind}, placement.owner), "place counting fixture")
				placed = true
				break
			}
		}
		testutil.AssertTrue(t, placed, "fixture location exists")
	}
}

func TestGlobalCityCountCards_PreviewAndPlay(t *testing.T) {
	for _, tc := range []struct {
		name       string
		resource   shared.ResourceType
		production bool
		want       int
	}{
		{"Greenhouses", shared.ResourcePlant, false, 3},
		{"Zeppelins", shared.ResourceCreditProduction, true, 2},
		{"Energy Saving", shared.ResourceEnergyProduction, true, 3},
	} {
		for _, populated := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/populated=%t", tc.name, populated), func(t *testing.T) {
				ctx := context.Background()
				g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
				p, _ := g.GetPlayer(id)
				p.SetCorporationID("")
				card := testutil.GetCardByName(tc.name)
				p.Hand().AddCard(card.ID)
				testutil.SetPlayerCredits(ctx, p, 100)
				testutil.AssertNoError(t, g.GlobalParameters().SetOxygen(ctx, 5), "oxygen requirement")
				want := 0
				if populated {
					placeCityCountTestTiles(t, g, id, opponentID)
					want = tc.want
				}
				state := baseaction.CalculatePlayerCardState(&card, p, g, registry)
				testutil.AssertTrue(t, state.Available(), "card should be playable")
				testutil.AssertEqual(t, 1, len(state.ComputedValues), "one scaled behavior preview")
				testutil.AssertEqual(t, 1, len(state.ComputedValues[0].Outputs), "one scaled output")
				testutil.AssertEqual(t, string(tc.resource), state.ComputedValues[0].Outputs[0].ResourceType, "preview resource")
				testutil.AssertEqual(t, want, state.ComputedValues[0].Outputs[0].Amount, "preview includes correct cities")
				beforeResources, beforeProduction := p.Resources().Get(), p.Resources().Production()
				err := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID,
					shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil)
				testutil.AssertNoError(t, err, "play real database card")
				if tc.production {
					testutil.AssertEqual(t, beforeProduction.GetAmount(tc.resource)+want, p.Resources().Production().GetAmount(tc.resource), "production gain")
				} else {
					testutil.AssertEqual(t, beforeResources.GetAmount(tc.resource)+want, p.Resources().Get().GetAmount(tc.resource), "resource gain")
				}
			})
		}
	}
}

func TestMartianRails_ActionCountsOnlyMarsCities(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Martian Rails")
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), nil)
	p.Actions().SetActions([]shared.CardAction{{CardID: card.ID, CardName: card.Name, BehaviorIndex: 0, Behavior: card.Behaviors[0]}})
	p.Resources().Add(map[shared.ResourceType]int{shared.ResourceEnergy: 1})
	placeCityCountTestTiles(t, g, id, opponentID)
	before := p.Resources().Get()
	err := cardAction.NewUseCardActionAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, card.ID, 0, nil, nil, nil, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "use Martian Rails")
	testutil.AssertEqual(t, before.Credits+2, p.Resources().Get().Credits, "only two Mars cities earn credits")
	testutil.AssertEqual(t, before.Energy-1, p.Resources().Get().Energy, "pay energy once")
}

func TestResearchOutpost_AdjacencyAndPermanentDiscount(t *testing.T) {
	for _, adjacent := range []shared.ResourceType{"", shared.ResourceCityTile, shared.ResourceGreeneryTile, shared.ResourceOceanTile, shared.ResourceCommercialDistrictTile} {
		t.Run(string(adjacent), func(t *testing.T) {
			ctx := context.Background()
			g, repo, registry, id, otherID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(id)
			p.SetCorporationID("")
			p.Resources().Set(shared.Resources{Credits: 100})
			testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 10), "turn")
			var center, neighbor shared.HexPosition
			found := false
			for _, tile := range g.Board().Tiles() {
				if tile.Type != shared.ResourceLandTile || len(tile.Tags) > 0 || tile.OccupiedBy != nil {
					continue
				}
				for _, n := range tile.Coordinates.GetNeighbors() {
					nt, err := g.Board().GetTile(n)
					if err == nil && nt.OccupiedBy == nil {
						center = tile.Coordinates
						neighbor = n
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				t.Fatal("no adjacent spaces")
			}
			if adjacent != "" {
				testutil.AssertNoError(t, g.Board().UpdateTileOccupancy(ctx, neighbor, board.TileOccupant{Type: adjacent}, otherID), "adjacent occupied tile")
			}
			c := testutil.GetCardByName("Research Outpost")
			p.Hand().AddCard(c.ID)
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, c.ID, shared.NativePayment(shared.ResourceCredit, c.Cost), nil, nil, nil, nil, nil), "play outpost")
			selection := g.GetPendingTileSelection(id)
			if selection == nil {
				t.Fatal("missing placement")
			}
			testutil.AssertEqual(t, adjacent == "", testutil.ContainsHex(selection.AvailableHexes, center.String()), "no adjacent tile of any type")
			selectTile := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger())
			if adjacent != "" {
				_, err := selectTile.Execute(ctx, g.ID(), id, center.String())
				testutil.AssertErrorContains(t, err, "is not valid for placement", "invalid adjacency rejected")
			}
			_, err := selectTile.Execute(ctx, g.ID(), id, selection.AvailableHexes[0])
			testutil.AssertNoError(t, err, "legal placement")
			for _, name := range []string{"Power Plant", "Sponsors"} {
				next := testutil.GetCardByName(name)
				p.Hand().AddCard(next.ID)
				state := baseaction.CalculatePlayerCardState(&next, p, g, registry)
				testutil.AssertEqual(t, next.Cost-1, state.Cost["credit"], "discount in preview")
				before := p.Resources().Get().Credits
				testutil.AssertNoError(t, play.Execute(ctx, g.ID(), id, next.ID, shared.NativePayment(shared.ResourceCredit, next.Cost-1), nil, nil, nil, nil, nil), "discounted play")
				testutil.AssertEqual(t, before-next.Cost+1, p.Resources().Get().Credits, "permanent discount charged")
			}
		})
	}
}
