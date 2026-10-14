package card_effects_test

import (
	"context"
	"testing"

	"openmars/internal/action"
	cardAction "openmars/internal/action/card"
	confirmAction "openmars/internal/action/confirmation"
	tileAction "openmars/internal/action/tile"
	"openmars/internal/events"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

// --- Rover Construction (038) ---
// "Effect: When any city tile is placed, gain 2 M€."

func TestRoverConstruction_GainCreditsOnAnyCityPlacement(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	roverCard := gamecards.Card{
		ID:   "card-rover-construction",
		Name: "Rover Construction",
		Type: gamecards.CardTypeActive,
		Cost: 8,
		Tags: []shared.CardTag{shared.TagBuilding},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{roverCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// Register Rover Construction passive effect
	effect := shared.CardEffect{
		CardID:        "card-rover-construction",
		CardName:      "Rover Construction",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)

	// Subscribe the passive effect to events
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	creditsBefore := owner.Resources().Get().Credits

	// Simulate a city tile placement by another player
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	creditsAfter := owner.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore+2, creditsAfter,
		"Rover Construction owner should gain 2 credits when any player places a city")
}

func TestRoverConstruction_TriggersOnSelfCityToo(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	roverCard := gamecards.Card{
		ID:   "card-rover-construction",
		Name: "Rover Construction",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{roverCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	effect := shared.CardEffect{
		CardID:        "card-rover-construction",
		CardName:      "Rover Construction",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	creditsBefore := owner.Resources().Get().Credits

	// Self places a city
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	creditsAfter := owner.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore+2, creditsAfter,
		"Rover Construction should also trigger when owner places a city")
}

func TestRoverConstruction_DoesNotTriggerOnGreenery(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	roverCard := gamecards.Card{
		ID:   "card-rover-construction",
		Name: "Rover Construction",
		Type: gamecards.CardTypeActive,
		Cost: 8,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{roverCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	effect := shared.CardEffect{
		CardID:        "card-rover-construction",
		CardName:      "Rover Construction",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourceCredit, 2, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	creditsBefore := owner.Resources().Get().Credits

	// Place a greenery (not a city)
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceGreeneryTile),
	})

	creditsAfter := owner.Resources().Get().Credits
	testutil.AssertEqual(t, creditsBefore, creditsAfter,
		"Rover Construction should NOT trigger on greenery placement")
}

// --- Olympus Conference (185) ---
// "Effect: When you play a science tag, including this, either add a science resource
//
//	to this card, or remove a science resource from this card to draw a card."

func TestOlympusConference_AddScienceOnScienceTagPlayed(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	olympusConference := gamecards.Card{
		ID:   "card-olympus-conference",
		Name: "Olympus Conference",
		Type: gamecards.CardTypeActive,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagBuilding, shared.TagEarth, shared.TagScience},
		ResourceStorage: &gamecards.ResourceStorage{
			Type:     shared.ResourceScience,
			Starting: 0,
		},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{olympusConference})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// Set up Olympus Conference as played with resource storage
	owner.PlayedCards().AddCard("card-olympus-conference", "Olympus Conference", "active", []string{"building", "earth", "science"})
	owner.Resources().AddToStorage("card-olympus-conference", 0)

	// Register tag-played passive effect (choice 0: add science to self-card)
	effect := shared.CardEffect{
		CardID:        "card-olympus-conference",
		CardName:      "Olympus Conference",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type: "tag-played",
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagScience}},
						},
					},
				},
			},
			// For testing: just the add-science output (choice 0)
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceScience, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	storageBefore := owner.Resources().GetCardStorage("card-olympus-conference")

	// Simulate playing a science tag
	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	storageAfter := owner.Resources().GetCardStorage("card-olympus-conference")
	testutil.AssertEqual(t, storageBefore+1, storageAfter,
		"Olympus Conference should add 1 science resource when a science tag is played")
}

func TestOlympusConference_DoesNotTriggerOnNonScienceTag(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	olympusConference := gamecards.Card{
		ID:   "card-olympus-conference",
		Name: "Olympus Conference",
		Type: gamecards.CardTypeActive,
		Cost: 10,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{olympusConference})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-olympus-conference", "Olympus Conference", "active", []string{"building", "earth", "science"})
	owner.Resources().AddToStorage("card-olympus-conference", 0)

	effect := shared.CardEffect{
		CardID:        "card-olympus-conference",
		CardName:      "Olympus Conference",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type: "tag-played",
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagScience}},
						},
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceScience, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	storageBefore := owner.Resources().GetCardStorage("card-olympus-conference")

	// Play a building tag (not science)
	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "building",
	})

	storageAfter := owner.Resources().GetCardStorage("card-olympus-conference")
	testutil.AssertEqual(t, storageBefore, storageAfter,
		"Olympus Conference should NOT trigger on non-science tag")
}

// --- Olympus Conference (185) - Triggered Choice Tests ---

func olympusConferenceChoiceBehavior() shared.CardBehavior {
	return shared.CardBehavior{
		Triggers: []shared.Trigger{
			{
				Type: shared.TriggerTypeAuto,
				Condition: &shared.ResourceTriggerCondition{
					Type: "tag-played",
					Selectors: []shared.Selector{
						{Tags: []shared.CardTag{shared.TagScience}},
					},
				},
			},
		},
		Choices: []shared.Choice{
			{
				Outputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceScience, 1, "self-card"),
				},
			},
			{
				Inputs: []shared.BehaviorCondition{
					shared.NewCardStorageCondition(shared.ResourceScience, 1, "self-card"),
				},
				Outputs: []shared.BehaviorCondition{
					shared.NewCardOperationCondition(shared.ResourceCardDraw, 1, "self-player"),
				},
			},
		},
	}
}

func TestOlympusConference_TriggeredChoice_CreatesPendingSelection(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	olympusConference := gamecards.Card{
		ID:   "card-olympus-conference",
		Name: "Olympus Conference",
		Type: gamecards.CardTypeActive,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagBuilding, shared.TagEarth, shared.TagScience},
		ResourceStorage: &gamecards.ResourceStorage{
			Type:     shared.ResourceScience,
			Starting: 0,
		},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{olympusConference})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-olympus-conference", "Olympus Conference", "active", []string{"building", "earth", "science"})
	owner.Resources().AddToStorage("card-olympus-conference", 1)

	effect := shared.CardEffect{
		CardID:        "card-olympus-conference",
		CardName:      "Olympus Conference",
		BehaviorIndex: 0,
		Behavior:      olympusConferenceChoiceBehavior(),
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Should have no pending selection before event")

	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	selection := owner.Selection().GetPendingBehaviorResolutions()[0]
	testutil.AssertTrue(t, selection != nil, "Should have a pending behavior choice selection")
	testutil.AssertEqual(t, 2, len(selection.Choices), "Should have 2 choices")
	testutil.AssertEqual(t, "Olympus Conference", selection.Source, "Source should be Olympus Conference")
	testutil.AssertEqual(t, "card-olympus-conference", selection.SourceCardID, "SourceCardID should match")
}

func TestOlympusConference_Choice0_AddScience(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	olympusConference := gamecards.Card{
		ID:   "card-olympus-conference",
		Name: "Olympus Conference",
		Type: gamecards.CardTypeActive,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagBuilding, shared.TagEarth, shared.TagScience},
		ResourceStorage: &gamecards.ResourceStorage{
			Type:     shared.ResourceScience,
			Starting: 0,
		},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{olympusConference})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-olympus-conference", "Olympus Conference", "active", []string{"building", "earth", "science"})
	owner.Resources().AddToStorage("card-olympus-conference", 1)

	effect := shared.CardEffect{
		CardID:        "card-olympus-conference",
		CardName:      "Olympus Conference",
		BehaviorIndex: 0,
		Behavior:      olympusConferenceChoiceBehavior(),
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	storageBefore := owner.Resources().GetCardStorage("card-olympus-conference")

	confirmBehaviorChoice := confirmAction.NewConfirmBehaviorChoiceAction(repo, cardRegistry, nil, logger)
	err := confirmBehaviorChoice.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, 0, nil)
	testutil.AssertNoError(t, err, "Choice 0 (add science) should succeed")

	storageAfter := owner.Resources().GetCardStorage("card-olympus-conference")
	testutil.AssertEqual(t, storageBefore+1, storageAfter,
		"Should have 1 more science resource on card after choice 0")

	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Pending selection should be cleared after confirming")
}

func TestOlympusConference_Choice1_RemoveScienceToDrawCard(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	olympusConference := gamecards.Card{
		ID:   "card-olympus-conference",
		Name: "Olympus Conference",
		Type: gamecards.CardTypeActive,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagBuilding, shared.TagEarth, shared.TagScience},
		ResourceStorage: &gamecards.ResourceStorage{
			Type:     shared.ResourceScience,
			Starting: 0,
		},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{olympusConference})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-olympus-conference", "Olympus Conference", "active", []string{"building", "earth", "science"})
	owner.Resources().AddToStorage("card-olympus-conference", 2)

	effect := shared.CardEffect{
		CardID:        "card-olympus-conference",
		CardName:      "Olympus Conference",
		BehaviorIndex: 0,
		Behavior:      olympusConferenceChoiceBehavior(),
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	handBefore := owner.Hand().CardCount()

	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	confirmBehaviorChoice := confirmAction.NewConfirmBehaviorChoiceAction(repo, cardRegistry, nil, logger)
	err := confirmBehaviorChoice.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, 1, nil)
	testutil.AssertNoError(t, err, "Choice 1 (remove science, draw card) should succeed")

	storageAfter := owner.Resources().GetCardStorage("card-olympus-conference")
	testutil.AssertEqual(t, 1, storageAfter,
		"Should have 1 science resource after spending 1 from 2")

	handAfter := owner.Hand().CardCount()
	testutil.AssertEqual(t, handBefore+1, handAfter,
		"Should have drawn 1 card")

	testutil.AssertTrue(t, len(owner.Selection().GetPendingBehaviorResolutions()) == 0,
		"Pending selection should be cleared after confirming")
}

func TestOlympusConference_Choice1_FailsWithoutScience(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	olympusConference := gamecards.Card{
		ID:   "card-olympus-conference",
		Name: "Olympus Conference",
		Type: gamecards.CardTypeActive,
		Cost: 10,
		Tags: []shared.CardTag{shared.TagBuilding, shared.TagEarth, shared.TagScience},
		ResourceStorage: &gamecards.ResourceStorage{
			Type:     shared.ResourceScience,
			Starting: 0,
		},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{olympusConference})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-olympus-conference", "Olympus Conference", "active", []string{"building", "earth", "science"})
	owner.Resources().AddToStorage("card-olympus-conference", 1)

	effect := shared.CardEffect{
		CardID:        "card-olympus-conference",
		CardName:      "Olympus Conference",
		BehaviorIndex: 0,
		Behavior:      olympusConferenceChoiceBehavior(),
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	events.Publish(testGame.EventBus(), events.TagPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		Tag:      "science",
	})

	owner.Resources().AddToStorage("card-olympus-conference", -1)

	confirmBehaviorChoice := confirmAction.NewConfirmBehaviorChoiceAction(repo, cardRegistry, nil, logger)
	err := confirmBehaviorChoice.Execute(ctx, testGame.ID(), owner.ID(), owner.Selection().GetPendingBehaviorResolutions()[0].ID, 1, nil)
	testutil.AssertErrorContains(t, err, "Not enough resources on Olympus Conference", "Choice 1 should fail with 0 science resources on card")
}

// --- Viral Enhancers (074) ---
// "Effect: When you play a plant, microbe, or an animal tag, including this,
//
//	gain 1 plant or add 1 resource to that card."

func TestViralEnhancers_GainPlantOnPlantTagPlayed(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	viralEnhancers := gamecards.Card{
		ID:   "card-viral-enhancers",
		Name: "Viral Enhancers",
		Type: gamecards.CardTypeActive,
		Cost: 9,
		Tags: []shared.CardTag{shared.TagMicrobe, shared.TagScience},
	}
	plantCard := gamecards.Card{
		ID:   "card-plant-test",
		Name: "Plant Test Card",
		Type: gamecards.CardTypeAutomated,
		Cost: 0,
		Tags: []shared.CardTag{shared.TagPlant},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{viralEnhancers, plantCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-viral-enhancers", "Viral Enhancers", "active", []string{"microbe", "science"})

	// Register card-played passive effect (simplified to just gain plant)
	effect := shared.CardEffect{
		CardID:        "card-viral-enhancers",
		CardName:      "Viral Enhancers",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:   "card-played",
						Target: testutil.StrPtr("self-player"),
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagPlant}},
							{Tags: []shared.CardTag{shared.TagMicrobe}},
							{Tags: []shared.CardTag{shared.TagAnimal}},
						},
					},
				},
			},
			// Simplified: just the "gain 1 plant" choice output for testing
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourcePlant, 1, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	plantsBefore := owner.Resources().Get().Plants

	// Simulate playing a plant-tagged card
	events.Publish(testGame.EventBus(), events.CardPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		CardID:   "card-plant-test",
		CardName: "Plant Test Card",
	})

	plantsAfter := owner.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+1, plantsAfter,
		"Viral Enhancers should give 1 plant when a plant-tagged card is played")
}

func TestViralEnhancers_DoesNotTriggerOnBuildingTag(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	viralEnhancers := gamecards.Card{
		ID:   "card-viral-enhancers",
		Name: "Viral Enhancers",
		Type: gamecards.CardTypeActive,
		Cost: 9,
		Tags: []shared.CardTag{shared.TagMicrobe, shared.TagScience},
	}
	buildingCard := gamecards.Card{
		ID:   "card-building-test",
		Name: "Building Test Card",
		Type: gamecards.CardTypeAutomated,
		Cost: 0,
		Tags: []shared.CardTag{shared.TagBuilding},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{viralEnhancers, buildingCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-viral-enhancers", "Viral Enhancers", "active", []string{"microbe", "science"})

	effect := shared.CardEffect{
		CardID:        "card-viral-enhancers",
		CardName:      "Viral Enhancers",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:   "card-played",
						Target: testutil.StrPtr("self-player"),
						Selectors: []shared.Selector{
							{Tags: []shared.CardTag{shared.TagPlant}},
							{Tags: []shared.CardTag{shared.TagMicrobe}},
							{Tags: []shared.CardTag{shared.TagAnimal}},
						},
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourcePlant, 1, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	plantsBefore := owner.Resources().Get().Plants

	// Play a building-tagged card (should NOT trigger)
	events.Publish(testGame.EventBus(), events.CardPlayedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		CardID:   "card-building-test",
		CardName: "Building Test Card",
	})

	plantsAfter := owner.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore, plantsAfter,
		"Viral Enhancers should NOT trigger on building tag")
}

// --- Arctic Algae (023) ---
// "Effect: When anyone places an ocean tile, gain 2 plants."

func TestArcticAlgae_GainPlantsOnAnyOceanPlacement(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	arcticAlgaeCard := gamecards.Card{
		ID:   "card-arctic-algae",
		Name: "Arctic Algae",
		Type: gamecards.CardTypeActive,
		Cost: 12,
		Tags: []shared.CardTag{shared.TagPlant},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{arcticAlgaeCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	effect := shared.CardEffect{
		CardID:        "card-arctic-algae",
		CardName:      "Arctic Algae",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:   "ocean-placed",
						Target: &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourcePlant, 2, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	plantsBefore := owner.Resources().Get().Plants

	// Another player places an ocean tile
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceOceanTile),
	})

	plantsAfter := owner.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter,
		"Arctic Algae owner should gain 2 plants when any player places an ocean")
}

func TestArcticAlgae_TriggersOnSelfOceanToo(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	arcticAlgaeCard := gamecards.Card{
		ID:   "card-arctic-algae",
		Name: "Arctic Algae",
		Type: gamecards.CardTypeActive,
		Cost: 12,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{arcticAlgaeCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	effect := shared.CardEffect{
		CardID:        "card-arctic-algae",
		CardName:      "Arctic Algae",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:   "ocean-placed",
						Target: &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourcePlant, 2, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	plantsBefore := owner.Resources().Get().Plants

	// Owner places an ocean tile
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceOceanTile),
	})

	plantsAfter := owner.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore+2, plantsAfter,
		"Arctic Algae should also trigger when owner places an ocean")
}

func TestArcticAlgae_DoesNotTriggerOnCityPlacement(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	arcticAlgaeCard := gamecards.Card{
		ID:   "card-arctic-algae",
		Name: "Arctic Algae",
		Type: gamecards.CardTypeActive,
		Cost: 12,
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{arcticAlgaeCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	effect := shared.CardEffect{
		CardID:        "card-arctic-algae",
		CardName:      "Arctic Algae",
		BehaviorIndex: 0,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:   "ocean-placed",
						Target: &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewBasicResourceCondition(shared.ResourcePlant, 2, "self-player"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	plantsBefore := owner.Resources().Get().Plants

	// Place a city (not an ocean)
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	plantsAfter := owner.Resources().Get().Plants
	testutil.AssertEqual(t, plantsBefore, plantsAfter,
		"Arctic Algae should NOT trigger on city placement")
}

// --- Pets (172) ---
// "Effect: Add an animal to this card when any city is built."

func TestPets_GainsAnimalWhenOtherPlayerPlacesCity(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	petsCard := gamecards.Card{
		ID:              "card-pets",
		Name:            "Pets",
		Type:            gamecards.CardTypeActive,
		Cost:            10,
		Tags:            []shared.CardTag{shared.TagAnimal, shared.TagEarth},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{petsCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-pets", "Pets", "active", []string{"animal", "earth"})

	effect := shared.CardEffect{
		CardID:        "card-pets",
		CardName:      "Pets",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	animalsBefore := owner.Resources().GetCardStorage("card-pets")

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	animalsAfter := owner.Resources().GetCardStorage("card-pets")
	testutil.AssertEqual(t, animalsBefore+1, animalsAfter,
		"Pets should gain 1 animal when any player places a city")
}

func TestPets_GainsAnimalWhenSelfPlacesCity(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	petsCard := gamecards.Card{
		ID:              "card-pets",
		Name:            "Pets",
		Type:            gamecards.CardTypeActive,
		Cost:            10,
		Tags:            []shared.CardTag{shared.TagAnimal, shared.TagEarth},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{petsCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-pets", "Pets", "active", []string{"animal", "earth"})

	effect := shared.CardEffect{
		CardID:        "card-pets",
		CardName:      "Pets",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	animalsBefore := owner.Resources().GetCardStorage("card-pets")

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	animalsAfter := owner.Resources().GetCardStorage("card-pets")
	testutil.AssertEqual(t, animalsBefore+1, animalsAfter,
		"Pets should gain 1 animal when owner places a city")
}

func TestPets_DoesNotTriggerOnGreeneryPlacement(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	petsCard := gamecards.Card{
		ID:              "card-pets",
		Name:            "Pets",
		Type:            gamecards.CardTypeActive,
		Cost:            10,
		Tags:            []shared.CardTag{shared.TagAnimal, shared.TagEarth},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{petsCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-pets", "Pets", "active", []string{"animal", "earth"})

	effect := shared.CardEffect{
		CardID:        "card-pets",
		CardName:      "Pets",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	animalsBefore := owner.Resources().GetCardStorage("card-pets")

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceGreeneryTile),
	})

	animalsAfter := owner.Resources().GetCardStorage("card-pets")
	testutil.AssertEqual(t, animalsBefore, animalsAfter,
		"Pets should NOT gain animals on greenery placement")
}

func TestPets_DoesNotTriggerOnOceanPlacement(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	petsCard := gamecards.Card{
		ID:              "card-pets",
		Name:            "Pets",
		Type:            gamecards.CardTypeActive,
		Cost:            10,
		Tags:            []shared.CardTag{shared.TagAnimal, shared.TagEarth},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{petsCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-pets", "Pets", "active", []string{"animal", "earth"})

	effect := shared.CardEffect{
		CardID:        "card-pets",
		CardName:      "Pets",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	animalsBefore := owner.Resources().GetCardStorage("card-pets")

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceOceanTile),
	})

	animalsAfter := owner.Resources().GetCardStorage("card-pets")
	testutil.AssertEqual(t, animalsBefore, animalsAfter,
		"Pets should NOT gain animals on ocean placement")
}

func TestPets_AccumulatesAnimalsFromMultipleCities(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	petsCard := gamecards.Card{
		ID:              "card-pets",
		Name:            "Pets",
		Type:            gamecards.CardTypeActive,
		Cost:            10,
		Tags:            []shared.CardTag{shared.TagAnimal, shared.TagEarth},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{petsCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-pets", "Pets", "active", []string{"animal", "earth"})

	effect := shared.CardEffect{
		CardID:        "card-pets",
		CardName:      "Pets",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	animalsBefore := owner.Resources().GetCardStorage("card-pets")

	for i := 0; i < 5; i++ {
		playerID := owner.ID()
		if i%2 == 1 {
			playerID = other.ID()
		}
		events.Publish(testGame.EventBus(), events.TilePlacedEvent{
			GameID:   testGame.ID(),
			PlayerID: playerID,
			TileType: string(shared.ResourceCityTile),
		})
	}

	animalsAfter := owner.Resources().GetCardStorage("card-pets")
	testutil.AssertEqual(t, animalsBefore+5, animalsAfter,
		"Pets should accumulate 5 animals from 5 city placements")
}

func TestPets_DoesNotTriggerForWrongGameID(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()

	anyPlayerTarget := "any-player"
	petsCard := gamecards.Card{
		ID:              "card-pets",
		Name:            "Pets",
		Type:            gamecards.CardTypeActive,
		Cost:            10,
		Tags:            []shared.CardTag{shared.TagAnimal, shared.TagEarth},
		ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceAnimal, Starting: 0},
	}
	cardRegistry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{petsCard})

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.PlayedCards().AddCard("card-pets", "Pets", "active", []string{"animal", "earth"})

	effect := shared.CardEffect{
		CardID:        "card-pets",
		CardName:      "Pets",
		BehaviorIndex: 1,
		Behavior: shared.CardBehavior{
			Triggers: []shared.Trigger{
				{
					Type: shared.TriggerTypeAuto,
					Condition: &shared.ResourceTriggerCondition{
						Type:     "city-placed",
						Location: testutil.StrPtr("anywhere"),
						Target:   &anyPlayerTarget,
					},
				},
			},
			Outputs: []shared.BehaviorCondition{
				shared.NewCardStorageCondition(shared.ResourceAnimal, 1, "self-card"),
			},
		},
	}
	owner.Effects().AddEffect(effect)
	action.SubscribePassiveEffectToEvents(ctx, testGame, owner, effect, logger, cardRegistry)

	animalsBefore := owner.Resources().GetCardStorage("card-pets")

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   "wrong-game-id",
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	animalsAfter := owner.Resources().GetCardStorage("card-pets")
	testutil.AssertEqual(t, animalsBefore, animalsAfter,
		"Pets should NOT trigger for events from a different game")
}

// --- Pets End-to-End Integration Tests ---
// These test the full flow: PlayCardAction → passive effect registration → event trigger

func TestPets_EndToEnd_PassiveEffectRegisteredOnPlay(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Pets")
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	owner.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), owner.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Pets should play successfully")

	// Verify 1 initial animal from auto trigger
	storage := owner.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 1, storage, "Pets should have 1 initial animal after play")

	// Now another player places a city — passive effect should trigger
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	storageAfter := owner.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 2, storageAfter,
		"Pets should gain 1 animal when another player places a city (end-to-end)")
}

func TestPets_EndToEnd_MultipleCityPlacements(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Pets")
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	owner.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), owner.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Pets should play successfully")

	// Place 3 cities: 1 by owner, 2 by other player
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceCityTile),
	})
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceCityTile),
	})
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	// 1 initial + 3 from cities = 4
	storage := owner.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 4, storage,
		"Pets should have 4 animals (1 initial + 3 from city placements)")
}

func TestPets_EndToEnd_DoesNotTriggerOnOcean(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 1)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Pets")
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	owner := players[0]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	owner.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	owner.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), owner.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Pets should play successfully")

	storageBefore := owner.Resources().GetCardStorage(card.ID)

	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceOceanTile),
	})

	storageAfter := owner.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, storageBefore, storageAfter,
		"Pets should NOT gain animals on ocean placement (end-to-end)")
}

func TestPets_DoesNotGainAnimalsForPreexistingCities(t *testing.T) {
	testGame, repo := testutil.CreateTestGameWithPlayers(t, 2)
	logger := testutil.TestLogger()
	ctx := context.Background()
	card := testutil.GetCardByName("Pets")
	cardRegistry := testutil.CreateTestCardRegistry()

	players := testGame.GetAllPlayers()
	owner := players[0]
	other := players[1]
	owner.SetCorporationID(testutil.CardID("Tharsis Republic"))
	other.SetCorporationID(testutil.CardID("Tharsis Republic"))
	testutil.StartTestGame(t, testGame)

	// Place cities BEFORE Pets is played
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: other.ID(),
		TileType: string(shared.ResourceCityTile),
	})
	events.Publish(testGame.EventBus(), events.TilePlacedEvent{
		GameID:   testGame.ID(),
		PlayerID: owner.ID(),
		TileType: string(shared.ResourceCityTile),
	})

	// Now play Pets
	owner.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceCredit: 100,
	})
	owner.Hand().AddCard(card.ID)

	playCardAction := cardAction.NewPlayCardAction(repo, cardRegistry, nil, logger)
	payment := shared.NativePayment(shared.
		ResourceCredit, 10)
	err := playCardAction.Execute(ctx, testGame.ID(), owner.ID(), card.ID, payment, nil, nil, nil, nil, nil)
	testutil.AssertNoError(t, err, "Pets should play successfully")

	// Only 1 initial animal — no retroactive animals from preexisting cities
	storage := owner.Resources().GetCardStorage(card.ID)
	testutil.AssertEqual(t, 1, storage,
		"Pets should only have 1 initial animal, no retroactive animals for preexisting cities")
}

func TestOlympusConference_SelfTriggerUsesPostPlayResources(t *testing.T) {
	for _, tc := range []struct {
		name     string
		starting int
		onPlay   int
		pending  int
		stored   int
	}{
		{name: "empty storage auto adds science", stored: 1},
		{name: "starting science retains both choices", starting: 1, pending: 1, stored: 1},
		{name: "on-play science retains both choices", onPlay: 1, pending: 1, stored: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, repo, _, playerID, opponentID := testutil.SetupTwoPlayerGame(t)
			p, _ := g.GetPlayer(playerID)
			card := testutil.GetCardByName("Olympus Conference")
			storage := *card.ResourceStorage
			storage.Starting = tc.starting
			card.ResourceStorage = &storage
			if tc.onPlay > 0 {
				card.Behaviors = append(append([]shared.CardBehavior{}, card.Behaviors...), shared.CardBehavior{
					ID: "initial-science", Triggers: []shared.Trigger{{Type: shared.TriggerTypeAuto}},
					Outputs: []shared.BehaviorCondition{shared.NewCardStorageCondition(shared.ResourceScience, tc.onPlay, "self-card")},
				})
			}
			registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
			p.Resources().Add(map[shared.ResourceType]int{shared.ResourceCredit: 100})
			p.Hand().AddCard(card.ID)
			testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), playerID, 1), "set last action")
			play := cardAction.NewPlayCardAction(repo, registry, nil, testutil.TestLogger())
			testutil.AssertNoError(t, play.Execute(context.Background(), g.ID(), playerID, card.ID, shared.NativePayment(shared.ResourceCredit, card.Cost), nil, nil, nil, nil, nil), "play without a pre-play choice")
			testutil.AssertEqual(t, tc.pending, len(p.Selection().GetPendingBehaviorResolutions()), "only meaningful choices remain")
			testutil.AssertEqual(t, tc.stored, p.Resources().GetCardStorage(card.ID), "self trigger uses resources after initial gains")
			if tc.pending == 0 {
				testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "forced effect must not hold the turn")
			} else {
				testutil.AssertEqual(t, playerID, g.CurrentTurn().PlayerID(), "real choice holds the turn")
			}
		})
	}
}

func TestPassiveChoice_ForcedResolution(t *testing.T) {
	for _, triggerType := range []string{"tag-played", "card-played"} {
		for _, tc := range []struct {
			name              string
			stored            int
			onlySpend         bool
			chooseTarget      bool
			combinedCost      bool
			interactiveOutput bool
			earlierEffect     bool
			wantStored        int
			wantPending       int
			wantCards         int
		}{
			{name: "empty storage", wantStored: 1},
			{name: "stored science", stored: 1, wantStored: 1, wantPending: 1},
			{name: "no legal choice", onlySpend: true},
			{name: "forced spending choice", stored: 1, onlySpend: true, wantCards: 1},
			{name: "destination remains a choice", chooseTarget: true, wantPending: 1},
			{name: "combined costs determine affordability", combinedCost: true, wantStored: 1},
			{name: "multi-step output cannot spend automatically", interactiveOutput: true, onlySpend: true, stored: 1, wantStored: 1, wantPending: 1},
			{name: "earlier effect preserves ordering", earlierEffect: true, wantPending: 2},
		} {
			t.Run(triggerType+"/"+tc.name, func(t *testing.T) {
				g, _ := testutil.CreateTestGameWithPlayers(t, 1)
				p := g.GetAllPlayers()[0]
				p.SetCorporationID(testutil.CardID("Tharsis Republic"))
				testutil.StartTestGame(t, g)
				card := gamecards.Card{ID: "science-source", Name: "Science source", Type: gamecards.CardTypeActive, ResourceStorage: &gamecards.ResourceStorage{Type: shared.ResourceScience}}
				registry := gamecards.NewInMemoryCardRegistry([]gamecards.Card{card})
				p.PlayedCards().AddCard(card.ID, card.Name, "active", nil)
				p.Resources().AddToStorage(card.ID, tc.stored)
				behavior := olympusConferenceChoiceBehavior()
				behavior.Triggers[0].Condition.Type = triggerType
				if triggerType == "card-played" {
					behavior.Triggers[0].Condition.Selectors = nil
				}
				if tc.onlySpend {
					behavior.Choices = behavior.Choices[1:]
				}
				if tc.chooseTarget {
					behavior.Choices = []shared.Choice{{Outputs: []shared.BehaviorCondition{
						shared.NewCardStorageCondition(shared.ResourceScience, 1, "any-card"),
					}}}
				}
				if tc.combinedCost {
					p.Resources().Set(shared.Resources{Credits: 1})
					behavior.Choices[1].Inputs = []shared.BehaviorCondition{
						shared.NewBasicResourceCondition(shared.ResourceCredit, 1, "self-player"),
						shared.NewBasicResourceCondition(shared.ResourceCredit, 1, "self-player"),
					}
				}
				if tc.interactiveOutput {
					behavior.Choices[0].Outputs = append(behavior.Choices[0].Outputs, shared.NewTilePlacementCondition(shared.ResourceCityPlacement, 1, "none"))
				}
				effect := shared.CardEffect{CardID: card.ID, CardName: card.Name, Behavior: behavior}
				p.Effects().AddEffect(effect)
				action.SubscribePassiveEffectToEvents(context.Background(), g, p, effect, testutil.TestLogger(), registry)
				if tc.earlierEffect {
					p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard", Source: "Earlier effect"})
				}
				before := p.Hand().CardCount()
				if triggerType == "tag-played" {
					events.Publish(g.EventBus(), events.TagPlayedEvent{GameID: g.ID(), PlayerID: p.ID(), CardID: card.ID, Tag: "science"})
				} else {
					events.Publish(g.EventBus(), events.CardPlayedEvent{GameID: g.ID(), PlayerID: p.ID(), CardID: card.ID})
				}
				action.ResolveForcedPassiveChoices(g, testutil.TestLogger(), registry)
				testutil.AssertEqual(t, tc.wantStored, p.Resources().GetCardStorage(card.ID), "correct forced resource transfer")
				testutil.AssertEqual(t, tc.wantCards, p.Hand().CardCount()-before, "only a legal draw executes")
				if tc.combinedCost {
					testutil.AssertEqual(t, 1, p.Resources().Get().Credits, "unaffordable branch cannot pay partial costs")
				}
				testutil.AssertEqual(t, tc.wantPending, len(p.Selection().GetPendingBehaviorResolutions()), "preserve real decisions, remove impossible ones")
				action.ResolveForcedPassiveChoices(g, testutil.TestLogger(), registry)
				testutil.AssertEqual(t, tc.wantStored, p.Resources().GetCardStorage(card.ID), "forced choice resolves only once")
			})
		}
	}
}

func TestPassiveChoice_ResolvesAfterEarlierDiscard(t *testing.T) {
	g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Olympus Conference")
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), []string{"science"})
	discardID := p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "card-discard", Source: "Earlier effect", MinCards: 0, MaxCards: 1})
	p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "choice", Source: card.Name, SourceCardID: card.ID, Choices: olympusConferenceChoiceBehavior().Choices})
	testutil.AssertNoError(t, g.SetCurrentTurn(context.Background(), id, 0), "last action already consumed")
	confirm := confirmAction.NewConfirmCardDiscardAction(repo, registry, nil, testutil.TestLogger())
	testutil.AssertNoError(t, confirm.Execute(context.Background(), g.ID(), id, discardID, nil), "skip earlier discard")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "remaining forced choice resolves once")
	testutil.AssertEqual(t, 0, len(p.Selection().GetPendingBehaviorResolutions()), "no redundant choice prompt remains")
	testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "completion can release the turn")
}

func TestPassiveChoice_ResolvesAfterTileSelection(t *testing.T) {
	ctx := context.Background()
	g, repo, registry, id, opponentID := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(id)
	card := testutil.GetCardByName("Olympus Conference")
	p.PlayedCards().AddCard(card.ID, card.Name, string(card.Type), []string{"science"})
	p.Selection().AddPendingBehaviorResolution(&shared.PendingBehaviorResolution{Kind: "choice", Source: card.Name, SourceCardID: card.ID, Choices: olympusConferenceChoiceBehavior().Choices})
	hex := testutil.FindUnoccupiedLandHex(t, g)
	testutil.AssertNoError(t, g.SetPendingTileSelection(ctx, id, &shared.PendingTileSelection{TileType: "city", AvailableHexes: []string{hex}, Source: "Earlier effect"}), "set tile selection")
	testutil.AssertNoError(t, g.SetCurrentTurn(ctx, id, 0), "last action already consumed")
	action.ResolveForcedPassiveChoices(g, testutil.TestLogger(), registry)
	testutil.AssertEqual(t, 0, p.Resources().GetCardStorage(card.ID), "tile selection keeps effect ordering open")
	_, err := tileAction.NewSelectTileAction(repo, registry, nil, testutil.TestLogger()).Execute(ctx, g.ID(), id, hex)
	testutil.AssertNoError(t, err, "place city")
	testutil.AssertEqual(t, 1, p.Resources().GetCardStorage(card.ID), "remaining forced choice resolves after placement")
	testutil.AssertEqual(t, 0, len(p.Selection().GetPendingBehaviorResolutions()), "no redundant choice prompt remains")
	testutil.AssertEqual(t, opponentID, g.CurrentTurn().PlayerID(), "completion releases the turn")
}
