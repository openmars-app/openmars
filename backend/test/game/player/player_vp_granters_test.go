package player_test

import (
	"context"
	"testing"
	"time"

	"openmars/internal/events"
	"openmars/internal/game/board"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

type mockVPRecalculationContext struct {
	cardStorage              map[string]map[string]int
	tagCounts                map[string]map[shared.CardTag]int
	tileCounts               map[shared.ResourceType]int
	playerTileCounts         map[string]map[shared.ResourceType]int
	adjacentTilesForCard     map[string]map[shared.ResourceType]int
	adjacentTilesToTileType  int
	colonyCount              int
	modeValues               map[string]int
	permanentProductionTypes int
}

func newMockVPRecalculationContext() *mockVPRecalculationContext {
	return &mockVPRecalculationContext{
		cardStorage:          make(map[string]map[string]int),
		tagCounts:            make(map[string]map[shared.CardTag]int),
		tileCounts:           make(map[shared.ResourceType]int),
		playerTileCounts:     make(map[string]map[shared.ResourceType]int),
		adjacentTilesForCard: make(map[string]map[shared.ResourceType]int),
	}
}

func (m *mockVPRecalculationContext) GetCardStorage(playerID string, cardID string) int {
	if playerCards, ok := m.cardStorage[playerID]; ok {
		return playerCards[cardID]
	}
	return 0
}

func (m *mockVPRecalculationContext) CountPlayerTagsByType(playerID string, tagType shared.CardTag) int {
	if playerTags, ok := m.tagCounts[playerID]; ok {
		return playerTags[tagType]
	}
	return 0
}

func (m *mockVPRecalculationContext) CountAllTilesOfType(tileType shared.ResourceType) int {
	return m.tileCounts[tileType]
}

func (m *mockVPRecalculationContext) CountPlayerTilesOfType(playerID string, tileType shared.ResourceType) int {
	if playerTiles, ok := m.playerTileCounts[playerID]; ok {
		return playerTiles[tileType]
	}
	return 0
}

func (m *mockVPRecalculationContext) CountAdjacentTilesForCard(cardID string, tileType shared.ResourceType) int {
	if cardTiles, ok := m.adjacentTilesForCard[cardID]; ok {
		return cardTiles[tileType]
	}
	return 0
}

func (m *mockVPRecalculationContext) CountAdjacentTilesToTileType(_ string, _, _ shared.ResourceType) int {
	return m.adjacentTilesToTileType
}

func (m *mockVPRecalculationContext) CountAllColonies() int {
	return m.colonyCount
}

func (m *mockVPRecalculationContext) setCardStorage(playerID string, cardID string, count int) {
	if _, ok := m.cardStorage[playerID]; !ok {
		m.cardStorage[playerID] = make(map[string]int)
	}
	m.cardStorage[playerID][cardID] = count
}

func (m *mockVPRecalculationContext) setTagCount(playerID string, tag shared.CardTag, count int) {
	if _, ok := m.tagCounts[playerID]; !ok {
		m.tagCounts[playerID] = make(map[shared.CardTag]int)
	}
	m.tagCounts[playerID][tag] = count
}

func TestFixedVPConditions(t *testing.T) {
	tests := []struct {
		name       string
		cardID     string
		cardName   string
		vpAmount   int
		expectedVP int
	}{
		{
			name:       "Dust Seals awards 1 VP",
			cardID:     testutil.CardID("Dust Seals"),
			cardName:   "Dust Seals",
			vpAmount:   1,
			expectedVP: 1,
		},
		{
			name:       "Farming awards 2 VP",
			cardID:     testutil.CardID("Farming"),
			cardName:   "Farming",
			vpAmount:   2,
			expectedVP: 2,
		},
		{
			name:       "Anti-Gravity Technology awards 3 VP",
			cardID:     testutil.CardID("Anti-Gravity Technology"),
			cardName:   "Anti-Gravity Technology",
			vpAmount:   3,
			expectedVP: 3,
		},
		{
			name:       "Earth Elevator awards 4 VP",
			cardID:     testutil.CardID("Earth Elevator"),
			cardName:   "Earth Elevator",
			vpAmount:   4,
			expectedVP: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			granter := shared.VPGranter{
				CardID:   tt.cardID,
				CardName: tt.cardName,
				VPConditions: []shared.VPCondition{
					{
						Amount:    tt.vpAmount,
						Condition: shared.VPConditionFixed,
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "ComputedValue should match expected VP")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match expected VP")
		})
	}
}

func TestPerTagVPConditions(t *testing.T) {
	tests := []struct {
		name       string
		cardID     string
		cardName   string
		tag        shared.CardTag
		vpAmount   int
		perAmount  int
		tagCount   int
		expectedVP int
	}{
		{
			name:       "Ganymede Colony: 1 VP per 1 jovian tag, 3 jovian tags = 3 VP",
			cardID:     testutil.CardID("Ganymede Colony"),
			cardName:   "Ganymede Colony",
			tag:        shared.TagJovian,
			vpAmount:   1,
			perAmount:  1,
			tagCount:   3,
			expectedVP: 3,
		},
		{
			name:       "Water Import From Europa: 1 VP per 1 jovian tag, 0 tags = 0 VP",
			cardID:     testutil.CardID("Water Import From Europa"),
			cardName:   "Water Import From Europa",
			tag:        shared.TagJovian,
			vpAmount:   1,
			perAmount:  1,
			tagCount:   0,
			expectedVP: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			granter := shared.VPGranter{
				CardID:   tt.cardID,
				CardName: tt.cardName,
				VPConditions: []shared.VPCondition{
					{
						Amount:    tt.vpAmount,
						Condition: shared.VPConditionPer,
						Per: &shared.PerCondition{
							Tag:    &tt.tag,
							Amount: tt.perAmount,
						},
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			ctx.setTagCount(p.ID(), tt.tag, tt.tagCount)
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "ComputedValue should match expected VP")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match expected VP")
		})
	}
}

func TestPerTileVPConditions(t *testing.T) {
	tests := []struct {
		name       string
		cardID     string
		cardName   string
		tileType   shared.ResourceType
		vpAmount   int
		perAmount  int
		tileCount  int
		expectedVP int
		isColony   bool
	}{
		{
			name:       "Immigration Shuttles: 1 VP per 3 city tiles, 6 cities = 2 VP",
			cardID:     testutil.CardID("Immigration Shuttles"),
			cardName:   "Immigration Shuttles",
			tileType:   shared.ResourceCityTile,
			vpAmount:   1,
			perAmount:  3,
			tileCount:  6,
			expectedVP: 2,
		},
		{
			name:       "Space Port Colony: 1 VP per 2 colonies, 4 colonies = 2 VP",
			cardID:     testutil.CardID("Space Port Colony"),
			cardName:   "Space Port Colony",
			tileType:   shared.ResourceColony,
			vpAmount:   1,
			perAmount:  2,
			tileCount:  4,
			expectedVP: 2,
			isColony:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			granter := shared.VPGranter{
				CardID:   tt.cardID,
				CardName: tt.cardName,
				VPConditions: []shared.VPCondition{
					{
						Amount:    tt.vpAmount,
						Condition: shared.VPConditionPer,
						Per: &shared.PerCondition{
							ResourceType: tt.tileType,
							Amount:       tt.perAmount,
						},
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			if tt.isColony {
				ctx.colonyCount = tt.tileCount
			} else {
				ctx.tileCounts[tt.tileType] = tt.tileCount
			}
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "ComputedValue should match expected VP")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match expected VP")
		})
	}
}

func TestMaxTriggerCap(t *testing.T) {
	selfCard := "self-card"
	maxTrigger1 := 1

	tests := []struct {
		name         string
		cardID       string
		cardName     string
		vpAmount     int
		perAmount    int
		storageCount int
		maxTrigger   *int
		expectedVP   int
	}{
		{
			name:         "Search For Life: 3 VP per 1 science, maxTrigger 1, 1 science = 3 VP",
			cardID:       testutil.CardID("Search For Life"),
			cardName:     "Search For Life",
			vpAmount:     3,
			perAmount:    1,
			storageCount: 1,
			maxTrigger:   &maxTrigger1,
			expectedVP:   3,
		},
		{
			name:         "Search For Life: 3 VP per 1 science, maxTrigger 1, 3 science = 3 VP (capped)",
			cardID:       testutil.CardID("Search For Life"),
			cardName:     "Search For Life",
			vpAmount:     3,
			perAmount:    1,
			storageCount: 3,
			maxTrigger:   &maxTrigger1,
			expectedVP:   3,
		},
		{
			name:         "Search For Life: 3 VP per 1 science, maxTrigger 1, 0 science = 0 VP (not met)",
			cardID:       testutil.CardID("Search For Life"),
			cardName:     "Search For Life",
			vpAmount:     3,
			perAmount:    1,
			storageCount: 0,
			maxTrigger:   &maxTrigger1,
			expectedVP:   0,
		},
		{
			name:         "Zero resources on per-condition card = 0 VP",
			cardID:       testutil.CardID("Birds"),
			cardName:     "Birds",
			vpAmount:     1,
			perAmount:    1,
			storageCount: 0,
			maxTrigger:   nil,
			expectedVP:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			granter := shared.VPGranter{
				CardID:   tt.cardID,
				CardName: tt.cardName,
				VPConditions: []shared.VPCondition{
					{
						Amount:     tt.vpAmount,
						Condition:  shared.VPConditionPer,
						MaxTrigger: tt.maxTrigger,
						Per: &shared.PerCondition{
							ResourceType: shared.ResourceScience,
							Amount:       tt.perAmount,
							Target:       &selfCard,
						},
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			ctx.setCardStorage(p.ID(), tt.cardID, tt.storageCount)
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "ComputedValue should match expected VP")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match expected VP")
		})
	}
}

func TestVPOrderingAddThenGetAll(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	granterA := shared.VPGranter{
		CardID:   "card-a",
		CardName: "Card A",
		VPConditions: []shared.VPCondition{
			{Amount: 1, Condition: shared.VPConditionFixed},
		},
	}
	granterB := shared.VPGranter{
		CardID:   "card-b",
		CardName: "Card B",
		VPConditions: []shared.VPCondition{
			{Amount: 2, Condition: shared.VPConditionFixed},
		},
	}

	p.VPGranters().Add(granterA)
	p.VPGranters().Add(granterB)

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 2, len(granters), "Should have 2 VP granters")
	testutil.AssertEqual(t, "Card A", granters[0].CardName, "First granter should be Card A")
	testutil.AssertEqual(t, "Card B", granters[1].CardName, "Second granter should be Card B")
}

func TestVPOrderingPrependThenAdd(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	corpGranter := shared.VPGranter{
		CardID:   testutil.CardID("Arklight"),
		CardName: "Arklight",
		VPConditions: []shared.VPCondition{
			{Amount: 1, Condition: shared.VPConditionFixed},
		},
	}
	cardGranter := shared.VPGranter{
		CardID:   testutil.CardID("Birds"),
		CardName: "Birds",
		VPConditions: []shared.VPCondition{
			{Amount: 1, Condition: shared.VPConditionFixed},
		},
	}

	p.VPGranters().Add(cardGranter)
	p.VPGranters().Prepend(corpGranter)

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 2, len(granters), "Should have 2 VP granters")
	testutil.AssertEqual(t, "Arklight", granters[0].CardName, "Corporation should be first after Prepend")
	testutil.AssertEqual(t, "Birds", granters[1].CardName, "Card should be second")
}

func TestMultipleGrantersTotalComputedVP(t *testing.T) {
	selfCard := "self-card"

	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	birdsID := testutil.CardID("Birds")
	ganymedeID := testutil.CardID("Ganymede Colony")

	fixedGranter := shared.VPGranter{
		CardID:   testutil.CardID("Dust Seals"),
		CardName: "Dust Seals",
		VPConditions: []shared.VPCondition{
			{Amount: 2, Condition: shared.VPConditionFixed},
		},
	}
	perGranter := shared.VPGranter{
		CardID:   birdsID,
		CardName: "Birds",
		VPConditions: []shared.VPCondition{
			{
				Amount:    1,
				Condition: shared.VPConditionPer,
				Per: &shared.PerCondition{
					ResourceType: shared.ResourceAnimal,
					Amount:       1,
					Target:       &selfCard,
				},
			},
		},
	}
	tagGranter := shared.VPGranter{
		CardID:   ganymedeID,
		CardName: "Ganymede Colony",
		VPConditions: []shared.VPCondition{
			{
				Amount:    1,
				Condition: shared.VPConditionPer,
				Per: &shared.PerCondition{
					Tag:    ptrTag(shared.TagJovian),
					Amount: 1,
				},
			},
		},
	}

	p.VPGranters().Add(fixedGranter)
	p.VPGranters().Add(perGranter)
	p.VPGranters().Add(tagGranter)

	ctx := newMockVPRecalculationContext()
	ctx.setCardStorage(p.ID(), birdsID, 3)
	ctx.setTagCount(p.ID(), shared.TagJovian, 2)
	p.VPGranters().RecalculateAll(ctx)

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 3, len(granters), "Should have 3 VP granters")
	testutil.AssertEqual(t, 2, granters[0].ComputedValue, "Dust Seals should be 2 VP")
	testutil.AssertEqual(t, 3, granters[1].ComputedValue, "Birds should be 3 VP (3 animals)")
	testutil.AssertEqual(t, 2, granters[2].ComputedValue, "Ganymede Colony should be 2 VP (2 jovian tags)")
	testutil.AssertEqual(t, 7, p.VPGranters().TotalComputedVP(), "TotalComputedVP should be sum of all granters")
}

func ptrTag(tag shared.CardTag) *shared.CardTag {
	return &tag
}

func TestIntegrationCardPlayedRegistersVPGranterAndResourceRecalculates(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	birdsID := testutil.CardID("Birds")
	p.PlayedCards().AddCard(birdsID, "Birds", "active", []string{"animal"})

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 1, len(granters), "Playing Birds should register 1 VP granter")
	testutil.AssertEqual(t, "Birds", granters[0].CardName, "VP granter should be for Birds")
	testutil.AssertEqual(t, 0, granters[0].ComputedValue, "VP should be 0 with no animals stored")

	p.Resources().AddToStorage(birdsID, 3)
	events.Publish(testGame.EventBus(), events.ResourceStorageChangedEvent{
		GameID:    testGame.ID(),
		PlayerID:  p.ID(),
		CardID:    birdsID,
		Timestamp: time.Now(),
	})

	granters = p.VPGranters().GetAll()
	testutil.AssertEqual(t, 3, granters[0].ComputedValue, "VP should be 3 after adding 3 animals")
	testutil.AssertEqual(t, 3, p.VPGranters().TotalComputedVP(), "TotalComputedVP should be 3")
}

func TestIntegrationTagPlayedRecalculatesVP(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	ganymedeID := testutil.CardID("Ganymede Colony")
	p.PlayedCards().AddCard(ganymedeID, "Ganymede Colony", "automated", []string{"city", "jovian", "space"})

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 1, len(granters), "Playing Ganymede Colony should register 1 VP granter")

	initialVP := granters[0].ComputedValue

	ioMiningID := testutil.CardID("Io Mining Industries")
	p.PlayedCards().AddCard(ioMiningID, "Io Mining Industries", "automated", []string{"jovian", "space"})

	granters = p.VPGranters().GetAll()
	testutil.AssertTrue(t, granters[0].ComputedValue > initialVP,
		"VP should increase after playing another card with jovian tag")
	// Io Mining Industries also has a VP granter (1 VP per jovian tag),
	// so TotalComputedVP is the sum of both granters
	testutil.AssertEqual(t, 2, len(granters), "Should have 2 VP granters (Ganymede Colony + Io Mining Industries)")
	testutil.AssertEqual(t, 2, granters[0].ComputedValue, "Ganymede Colony should be 2 VP (2 jovian tags)")
	testutil.AssertEqual(t, 2, granters[1].ComputedValue, "Io Mining Industries should be 2 VP (2 jovian tags)")
	testutil.AssertEqual(t, 4, p.VPGranters().TotalComputedVP(),
		"TotalComputedVP should be sum of both granters")
}

func TestIntegrationTilePlacedRecalculatesAllPlayersVP(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 2)
	players := testGame.GetAllPlayers()

	player1 := players[0]
	player2 := players[1]

	immigrationID := testutil.CardID("Immigration Shuttles")
	player1.PlayedCards().AddCard(immigrationID, "Immigration Shuttles", "automated", []string{"earth", "space"})
	player2.PlayedCards().AddCard(immigrationID, "Immigration Shuttles", "automated", []string{"earth", "space"})

	testutil.AssertEqual(t, 0, player1.VPGranters().TotalComputedVP(), "Player 1 VP should be 0 before tiles")
	testutil.AssertEqual(t, 0, player2.VPGranters().TotalComputedVP(), "Player 2 VP should be 0 before tiles")

	ctx := context.Background()
	tiles := testGame.Board().Tiles()
	citiesPlaced := 0
	for _, tile := range tiles {
		if tile.Type == shared.ResourceLandTile && tile.OccupiedBy == nil && citiesPlaced < 3 {
			err := testGame.Board().UpdateTileOccupancy(ctx, tile.Coordinates, board.TileOccupant{
				Type: shared.ResourceCityTile,
			}, player1.ID())
			testutil.AssertNoError(t, err, "Should place city tile")
			citiesPlaced++
		}
		if citiesPlaced >= 3 {
			break
		}
	}

	testutil.AssertEqual(t, 1, player1.VPGranters().TotalComputedVP(),
		"Player 1 VP should be 1 (3 cities / 3 per VP = 1)")
	testutil.AssertEqual(t, 1, player2.VPGranters().TotalComputedVP(),
		"Player 2 VP should also be 1 (tile changes affect all players)")
}

func TestPerAdjacentOceanTileVPGranter(t *testing.T) {
	tests := []struct {
		name           string
		adjacentOceans int
		totalOceans    int
		expectedVP     int
	}{
		{
			name:           "Capital: 0 adjacent oceans, 5 total = 0 VP",
			adjacentOceans: 0,
			totalOceans:    5,
			expectedVP:     0,
		},
		{
			name:           "Capital: 3 adjacent oceans, 9 total = 3 VP (not 9)",
			adjacentOceans: 3,
			totalOceans:    9,
			expectedVP:     3,
		},
		{
			name:           "Capital: 1 adjacent ocean, 1 total = 1 VP",
			adjacentOceans: 1,
			totalOceans:    1,
			expectedVP:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			capitalID := testutil.CardID("Capital")
			granter := shared.VPGranter{
				CardID:   capitalID,
				CardName: "Capital",
				VPConditions: []shared.VPCondition{
					{
						Amount:    1,
						Condition: shared.VPConditionPer,
						Per: &shared.PerCondition{
							ResourceType:       shared.ResourceOceanTile,
							Amount:             1,
							AdjacentToSelfTile: true,
						},
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			ctx.tileCounts[shared.ResourceOceanTile] = tt.totalOceans
			if _, ok := ctx.adjacentTilesForCard[capitalID]; !ok {
				ctx.adjacentTilesForCard[capitalID] = make(map[shared.ResourceType]int)
			}
			ctx.adjacentTilesForCard[capitalID][shared.ResourceOceanTile] = tt.adjacentOceans
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "Capital VP should count only adjacent oceans")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match")
		})
	}
}

func TestCorporationVPConditions(t *testing.T) {
	selfCard := "self-card"

	tests := []struct {
		name         string
		cardID       string
		cardName     string
		resourceType shared.ResourceType
		vpAmount     int
		perAmount    int
		storageCount int
		expectedVP   int
	}{
		{
			name:         "Arklight: 1 VP per 2 animals on self-card, 4 animals = 2 VP",
			cardID:       testutil.CardID("Arklight"),
			cardName:     "Arklight",
			resourceType: shared.ResourceAnimal,
			vpAmount:     1,
			perAmount:    2,
			storageCount: 4,
			expectedVP:   2,
		},
		{
			name:         "Celestic: 1 VP per 3 floaters on self-card, 9 floaters = 3 VP",
			cardID:       testutil.CardID("Celestic"),
			cardName:     "Celestic",
			resourceType: shared.ResourceFloater,
			vpAmount:     1,
			perAmount:    3,
			storageCount: 9,
			expectedVP:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			granter := shared.VPGranter{
				CardID:   tt.cardID,
				CardName: tt.cardName,
				VPConditions: []shared.VPCondition{
					{
						Amount:    tt.vpAmount,
						Condition: shared.VPConditionPer,
						Per: &shared.PerCondition{
							ResourceType: tt.resourceType,
							Amount:       tt.perAmount,
							Target:       &selfCard,
						},
					},
				},
			}

			p.VPGranters().Prepend(granter)

			ctx := newMockVPRecalculationContext()
			ctx.setCardStorage(p.ID(), tt.cardID, tt.storageCount)
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "ComputedValue should match expected VP")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match expected VP")
		})
	}
}

func TestCorporationVPGranterIsPrepended(t *testing.T) {
	selfCard := "self-card"

	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	cardGranter := shared.VPGranter{
		CardID:   testutil.CardID("Birds"),
		CardName: "Birds",
		VPConditions: []shared.VPCondition{
			{
				Amount:    1,
				Condition: shared.VPConditionPer,
				Per: &shared.PerCondition{
					ResourceType: shared.ResourceAnimal,
					Amount:       1,
					Target:       &selfCard,
				},
			},
		},
	}
	corpGranter := shared.VPGranter{
		CardID:   testutil.CardID("Arklight"),
		CardName: "Arklight",
		VPConditions: []shared.VPCondition{
			{
				Amount:    1,
				Condition: shared.VPConditionPer,
				Per: &shared.PerCondition{
					ResourceType: shared.ResourceAnimal,
					Amount:       2,
					Target:       &selfCard,
				},
			},
		},
	}

	p.VPGranters().Add(cardGranter)
	p.VPGranters().Prepend(corpGranter)

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 2, len(granters), "Should have 2 VP granters")
	testutil.AssertEqual(t, "Arklight", granters[0].CardName, "Corporation should be first after Prepend")
	testutil.AssertEqual(t, "Birds", granters[1].CardName, "Card should be second")
}

func TestIntegrationCorporationSelectedRegistersVPGranterAndResourceRecalculates(t *testing.T) {
	testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
	players := testGame.GetAllPlayers()
	p := players[0]

	arklightID := testutil.CardID("Arklight")
	testGame.RegisterCorporationVPGranter(p.ID(), arklightID)

	granters := p.VPGranters().GetAll()
	testutil.AssertEqual(t, 1, len(granters), "Selecting Arklight should register 1 VP granter")
	testutil.AssertEqual(t, "Arklight", granters[0].CardName, "VP granter should be for Arklight")
	testutil.AssertEqual(t, 0, granters[0].ComputedValue, "VP should be 0 with no animals stored")

	p.Resources().AddToStorage(arklightID, 4)
	events.Publish(testGame.EventBus(), events.ResourceStorageChangedEvent{
		GameID:    testGame.ID(),
		PlayerID:  p.ID(),
		CardID:    arklightID,
		Timestamp: time.Now(),
	})

	granters = p.VPGranters().GetAll()
	testutil.AssertEqual(t, 2, granters[0].ComputedValue, "VP should be 2 after adding 4 animals (1 VP per 2 animals)")
	testutil.AssertEqual(t, 2, p.VPGranters().TotalComputedVP(), "TotalComputedVP should be 2")

	birdsID := testutil.CardID("Birds")
	p.PlayedCards().AddCard(birdsID, "Birds", "active", []string{"animal"})

	granters = p.VPGranters().GetAll()
	testutil.AssertEqual(t, 2, len(granters), "Should have 2 VP granters after playing Birds")
	testutil.AssertEqual(t, "Arklight", granters[0].CardName, "Corporation should remain first (prepended)")
	testutil.AssertEqual(t, "Birds", granters[1].CardName, "Card should be second (appended)")
}

func TestPerResourceOnSelfCardVPConditions(t *testing.T) {
	selfCard := "self-card"

	tests := []struct {
		name         string
		cardID       string
		cardName     string
		resourceType shared.ResourceType
		vpAmount     int
		perAmount    int
		storageCount int
		expectedVP   int
	}{
		{
			name:         "Birds: 1 VP per 1 animal, 5 animals = 5 VP",
			cardID:       testutil.CardID("Birds"),
			cardName:     "Birds",
			resourceType: shared.ResourceAnimal,
			vpAmount:     1,
			perAmount:    1,
			storageCount: 5,
			expectedVP:   5,
		},
		{
			name:         "Small Animals: 1 VP per 2 animals, 5 animals = 2 VP",
			cardID:       testutil.CardID("Small Animals"),
			cardName:     "Small Animals",
			resourceType: shared.ResourceAnimal,
			vpAmount:     1,
			perAmount:    2,
			storageCount: 5,
			expectedVP:   2,
		},
		{
			name:         "Ants: 1 VP per 2 microbes, 6 microbes = 3 VP",
			cardID:       testutil.CardID("Ants"),
			cardName:     "Ants",
			resourceType: shared.ResourceMicrobe,
			vpAmount:     1,
			perAmount:    2,
			storageCount: 6,
			expectedVP:   3,
		},
		{
			name:         "Decomposers: 1 VP per 3 microbes, 9 microbes = 3 VP",
			cardID:       testutil.CardID("Decomposers"),
			cardName:     "Decomposers",
			resourceType: shared.ResourceMicrobe,
			vpAmount:     1,
			perAmount:    3,
			storageCount: 9,
			expectedVP:   3,
		},
		{
			name:         "Tardigrades: 1 VP per 4 microbes, 8 microbes = 2 VP",
			cardID:       testutil.CardID("Tardigrades"),
			cardName:     "Tardigrades",
			resourceType: shared.ResourceMicrobe,
			vpAmount:     1,
			perAmount:    4,
			storageCount: 8,
			expectedVP:   2,
		},
		{
			name:         "Floating Habs: 1 VP per 2 floaters, 6 floaters = 3 VP",
			cardID:       testutil.CardID("Floating Habs"),
			cardName:     "Floating Habs",
			resourceType: shared.ResourceFloater,
			vpAmount:     1,
			perAmount:    2,
			storageCount: 6,
			expectedVP:   3,
		},
		{
			name:         "Physics Complex: 2 VP per science, 4 science = 8 VP",
			cardID:       testutil.CardID("Physics Complex"),
			cardName:     "Physics Complex",
			resourceType: shared.ResourceScience,
			vpAmount:     2,
			perAmount:    1,
			storageCount: 4,
			expectedVP:   8,
		},
		{
			name:         "Security Fleet: 1 VP per fighter, 3 fighters = 3 VP",
			cardID:       testutil.CardID("Security Fleet"),
			cardName:     "Security Fleet",
			resourceType: shared.ResourceFighter,
			vpAmount:     1,
			perAmount:    1,
			storageCount: 3,
			expectedVP:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			granter := shared.VPGranter{
				CardID:   tt.cardID,
				CardName: tt.cardName,
				VPConditions: []shared.VPCondition{
					{
						Amount:    tt.vpAmount,
						Condition: shared.VPConditionPer,
						Per: &shared.PerCondition{
							ResourceType: tt.resourceType,
							Amount:       tt.perAmount,
							Target:       &selfCard,
						},
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			ctx.setCardStorage(p.ID(), tt.cardID, tt.storageCount)
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "ComputedValue should match expected VP")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match expected VP")
		})
	}
}

func TestPerAdjacentToTileTypeVPGranter(t *testing.T) {
	worldTreeTileType := shared.ResourceWorldTreeTile

	tests := []struct {
		name               string
		adjacentGreeneries int
		totalGreeneries    int
		expectedVP         int
	}{
		{
			name:               "World Tree: 0 adjacent greeneries, 5 total = 0 VP",
			adjacentGreeneries: 0,
			totalGreeneries:    5,
			expectedVP:         0,
		},
		{
			name:               "World Tree: 3 adjacent greeneries, 9 total = 3 VP (not 9)",
			adjacentGreeneries: 3,
			totalGreeneries:    9,
			expectedVP:         3,
		},
		{
			name:               "World Tree: 2 adjacent greeneries, 2 total = 2 VP",
			adjacentGreeneries: 2,
			totalGreeneries:    2,
			expectedVP:         2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGame, _ := testutil.CreateTestGameWithPlayers(t, 1)
			players := testGame.GetAllPlayers()
			p := players[0]

			worldTreeID := testutil.CardID("The World Tree")
			granter := shared.VPGranter{
				CardID:   worldTreeID,
				CardName: "The World Tree",
				VPConditions: []shared.VPCondition{
					{
						Amount:    1,
						Condition: shared.VPConditionPer,
						Per: &shared.PerCondition{
							ResourceType:       shared.ResourceGreeneryTile,
							Amount:             1,
							AdjacentToTileType: &worldTreeTileType,
						},
					},
				},
			}

			p.VPGranters().Add(granter)

			ctx := newMockVPRecalculationContext()
			ctx.tileCounts[shared.ResourceGreeneryTile] = tt.totalGreeneries
			ctx.adjacentTilesToTileType = tt.adjacentGreeneries
			p.VPGranters().RecalculateAll(ctx)

			granters := p.VPGranters().GetAll()
			testutil.AssertEqual(t, 1, len(granters), "Should have exactly 1 VP granter")
			testutil.AssertEqual(t, tt.expectedVP, granters[0].ComputedValue, "World Tree VP should count only adjacent greeneries")
			testutil.AssertEqual(t, tt.expectedVP, p.VPGranters().TotalComputedVP(), "TotalComputedVP should match")
		})
	}
}

func (m *mockVPRecalculationContext) ModeRequirementsMet(requirements []shared.StateRequirement) bool {
	for _, r := range requirements {
		v, ok := m.modeValues[r.State]
		if !ok || v != r.Equals {
			return false
		}
	}
	return true
}
func (m *mockVPRecalculationContext) CountPermanentProductionTypes(_ string) int {
	return m.permanentProductionTypes
}

func TestLiveScenarioVPChangesWithStateAndPermanentProduction(t *testing.T) {
	g, _ := testutil.CreateTestGameWithPlayers(t, 1)
	p := g.GetAllPlayers()[0]
	cap := 5
	p.VPGranters().Add(shared.VPGranter{CardID: "independent-production", VPConditions: []shared.VPCondition{{Amount: 1, Condition: "per", MaxTrigger: &cap, Per: &shared.PerCondition{ResourceType: shared.ResourcePermanentProductionTypes, Amount: 1}, StateRequirements: []shared.StateRequirement{{State: "transition", Equals: 1}}}}})
	ctx := newMockVPRecalculationContext()
	ctx.permanentProductionTypes = 4
	p.VPGranters().RecalculateAll(ctx)
	testutil.AssertEqual(t, 0, p.VPGranters().TotalComputedVP(), "Missing state must fail closed")
	ctx.modeValues = map[string]int{"transition": 1}
	p.VPGranters().RecalculateAll(ctx)
	testutil.AssertEqual(t, 4, p.VPGranters().TotalComputedVP(), "State enables production scoring")
	ctx.permanentProductionTypes = 6
	p.VPGranters().RecalculateAll(ctx)
	testutil.AssertEqual(t, 5, p.VPGranters().TotalComputedVP(), "Cap applies to live scoring")
	ctx.permanentProductionTypes = 2
	p.VPGranters().RecalculateAll(ctx)
	testutil.AssertEqual(t, 2, p.VPGranters().TotalComputedVP(), "Lost permanent production reduces score")
}
