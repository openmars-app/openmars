package cards_test

import (
	"context"
	"testing"

	"openmars/internal/game/award"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/shared"
	"openmars/test/testutil"
)

func strPtr(s string) *string {
	return &s
}

func tagPtr(t shared.CardTag) *shared.CardTag {
	return &t
}

func landlordAwardDef() *award.AwardDefinition {
	return &award.AwardDefinition{
		ID:          "landlord",
		Name:        "Landlord",
		Description: "Most tiles on Mars",
		GameModes:   []string{"classic"},
		Quantifier: []shared.PerCondition{
			{ResourceType: shared.ResourceNonOceanTile, Target: strPtr("self-player")},
		},
		Rewards: []award.AwardReward{
			{Place: 1, Outputs: []award.RewardOutput{{Type: "vp", Amount: 5}}},
			{Place: 2, Outputs: []award.RewardOutput{{Type: "vp", Amount: 2}}},
		},
	}
}

// TestLandlordAwardExcludesOceans verifies that Landlord counts cities + greeneries only.
func TestLandlordAwardExcludesOceans(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	ctx := context.Background()

	// Place 1 city owned by player
	err := g.Board().UpdateTileOccupancy(ctx, shared.HexPosition{Q: 0, R: 0, S: 0}, board.TileOccupant{
		Type: shared.ResourceCityTile,
	}, playerID)
	if err != nil {
		t.Fatalf("Failed to place city: %v", err)
	}

	// Place 1 greenery owned by player
	err = g.Board().UpdateTileOccupancy(ctx, shared.HexPosition{Q: 2, R: -2, S: 0}, board.TileOccupant{
		Type: shared.ResourceGreeneryTile,
	}, playerID)
	if err != nil {
		t.Fatalf("Failed to place greenery: %v", err)
	}

	// Place 2 ocean tiles (should NOT count for Landlord)
	oceanPositions := []shared.HexPosition{
		{Q: 3, R: -3, S: 0},
		{Q: 4, R: -4, S: 0},
	}
	for _, pos := range oceanPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceOceanTile,
		}, playerID)
		if err != nil {
			t.Fatalf("Failed to place ocean: %v", err)
		}
	}

	score := gamecards.CalculateAwardScore(landlordAwardDef(), p, g.Board(), cardRegistry)

	// Should count 1 city + 1 greenery = 2, NOT 4 (which would include oceans)
	if score != 2 {
		t.Fatalf("expected Landlord score=2 (1 city + 1 greenery), got %d", score)
	}
}

// TestLandlordAwardZeroWithOnlyOceans verifies Landlord returns 0 when player only has oceans.
func TestLandlordAwardZeroWithOnlyOceans(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)
	ctx := context.Background()

	// Place 3 ocean tiles owned by player
	positions := []shared.HexPosition{
		{Q: 0, R: 0, S: 0},
		{Q: 1, R: -1, S: 0},
		{Q: 2, R: -2, S: 0},
	}
	for _, pos := range positions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceOceanTile,
		}, playerID)
		if err != nil {
			t.Fatalf("Failed to place ocean: %v", err)
		}
	}

	score := gamecards.CalculateAwardScore(landlordAwardDef(), p, g.Board(), cardRegistry)

	if score != 0 {
		t.Fatalf("expected Landlord score=0 with only oceans, got %d", score)
	}
}

func TestBankerAwardCountsCreditProduction(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	p.Resources().AddProduction(map[shared.ResourceType]int{
		shared.ResourceCreditProduction: 5,
	})

	def := &award.AwardDefinition{
		ID:          "banker",
		Name:        "Banker",
		Description: "Most credit production",
		GameModes:   []string{"classic"},
		Quantifier: []shared.PerCondition{
			{ResourceType: shared.ResourceCreditProduction, Target: strPtr("self-player")},
		},
	}

	score := gamecards.CalculateAwardScore(def, p, g.Board(), cardRegistry)
	if score != 5 {
		t.Fatalf("expected Banker score=5, got %d", score)
	}
}

func TestScientistAwardCountsScienceTags(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	// Cards 005 (Search For Life), 006 (Inventors' Guild), 044 (Natural Preserve) all have science tags
	scienceCardIDs := []string{"005", "006", "044"}
	for _, cardID := range scienceCardIDs {
		p.PlayedCards().AddCard(cardID, cardID, "active", []string{"science"})
	}

	def := &award.AwardDefinition{
		ID:          "scientist",
		Name:        "Scientist",
		Description: "Most science tags",
		GameModes:   []string{"classic"},
		Quantifier: []shared.PerCondition{
			{ResourceType: shared.ResourceTag, Tag: tagPtr(shared.TagScience)},
		},
	}

	score := gamecards.CalculateAwardScore(def, p, g.Board(), cardRegistry)
	if score != 3 {
		t.Fatalf("expected Scientist score=3, got %d", score)
	}
}

func TestThermalistAwardCountsHeatResources(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceHeat: 7,
	})

	def := &award.AwardDefinition{
		ID:          "thermalist",
		Name:        "Thermalist",
		Description: "Most heat resources",
		GameModes:   []string{"classic"},
		Quantifier: []shared.PerCondition{
			{ResourceType: shared.ResourceHeat, Target: strPtr("self-player")},
		},
	}

	score := gamecards.CalculateAwardScore(def, p, g.Board(), cardRegistry)
	if score != 7 {
		t.Fatalf("expected Thermalist score=7, got %d", score)
	}
}

func TestMinerAwardCountsSteelAndTitanium(t *testing.T) {
	g, _, cardRegistry, playerID, _ := testutil.SetupTwoPlayerGame(t)
	p, _ := g.GetPlayer(playerID)

	p.Resources().Add(map[shared.ResourceType]int{
		shared.ResourceSteel:    3,
		shared.ResourceTitanium: 4,
	})

	def := &award.AwardDefinition{
		ID:          "miner",
		Name:        "Miner",
		Description: "Most steel and titanium resources",
		GameModes:   []string{"classic"},
		Quantifier: []shared.PerCondition{
			{ResourceType: shared.ResourceSteel, Target: strPtr("self-player")},
			{ResourceType: shared.ResourceTitanium, Target: strPtr("self-player")},
		},
	}

	score := gamecards.CalculateAwardScore(def, p, g.Board(), cardRegistry)
	if score != 7 {
		t.Fatalf("expected Miner score=7 (3 steel + 4 titanium), got %d", score)
	}
}

func TestScoreAwardPlacement(t *testing.T) {
	g, _, cardRegistry, player1ID, player2ID := testutil.SetupTwoPlayerGame(t)
	p1, _ := g.GetPlayer(player1ID)
	p2, _ := g.GetPlayer(player2ID)
	ctx := context.Background()

	// Player 1 places 3 cities + 2 greeneries = 5 tiles
	cityPositions := []shared.HexPosition{
		{Q: 0, R: 0, S: 0},
		{Q: 1, R: -1, S: 0},
		{Q: 2, R: -2, S: 0},
	}
	for _, pos := range cityPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceCityTile,
		}, player1ID)
		if err != nil {
			t.Fatalf("Failed to place city for player1: %v", err)
		}
	}
	greeneryPositions := []shared.HexPosition{
		{Q: 3, R: -3, S: 0},
		{Q: 4, R: -4, S: 0},
	}
	for _, pos := range greeneryPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceGreeneryTile,
		}, player1ID)
		if err != nil {
			t.Fatalf("Failed to place greenery for player1: %v", err)
		}
	}

	// Player 2 places 2 cities + 1 greenery = 3 tiles
	p2CityPositions := []shared.HexPosition{
		{Q: -1, R: 1, S: 0},
		{Q: -2, R: 2, S: 0},
	}
	for _, pos := range p2CityPositions {
		err := g.Board().UpdateTileOccupancy(ctx, pos, board.TileOccupant{
			Type: shared.ResourceCityTile,
		}, player2ID)
		if err != nil {
			t.Fatalf("Failed to place city for player2: %v", err)
		}
	}
	err := g.Board().UpdateTileOccupancy(ctx, shared.HexPosition{Q: -3, R: 3, S: 0}, board.TileOccupant{
		Type: shared.ResourceGreeneryTile,
	}, player2ID)
	if err != nil {
		t.Fatalf("Failed to place greenery for player2: %v", err)
	}

	def := landlordAwardDef()

	// Verify individual scores
	score1 := gamecards.CalculateAwardScore(def, p1, g.Board(), cardRegistry)
	if score1 != 5 {
		t.Fatalf("expected player1 score=5, got %d", score1)
	}
	score2 := gamecards.CalculateAwardScore(def, p2, g.Board(), cardRegistry)
	if score2 != 3 {
		t.Fatalf("expected player2 score=3, got %d", score2)
	}

	placements := gamecards.ScoreAward(def, g.GetAllPlayers(), g.Board(), cardRegistry)

	// Find placements for each player
	var p1Placement, p2Placement int
	for _, pl := range placements {
		if pl.PlayerID == player1ID {
			p1Placement = pl.Placement
		}
		if pl.PlayerID == player2ID {
			p2Placement = pl.Placement
		}
	}

	if p1Placement != 1 {
		t.Fatalf("expected player1 placement=1, got %d", p1Placement)
	}
	if p2Placement != 2 {
		t.Fatalf("expected player2 placement=2, got %d", p2Placement)
	}

	// Verify VP from definition
	vp1 := gamecards.GetAwardVP(def, 1)
	if vp1 != 5 {
		t.Fatalf("expected VP for 1st place=5, got %d", vp1)
	}
	vp2 := gamecards.GetAwardVP(def, 2)
	if vp2 != 2 {
		t.Fatalf("expected VP for 2nd place=2, got %d", vp2)
	}
}

func TestGetCostForFundedCount(t *testing.T) {
	def := &award.AwardDefinition{
		ID:   "test-cost",
		Name: "Test Cost Award",
		Costs: []award.AwardCost{
			{AwardsBought: 0, Cost: 8},
			{AwardsBought: 1, Cost: 14},
			{AwardsBought: 2, Cost: 20},
		},
	}

	if cost := def.GetCostForFundedCount(0); cost != 8 {
		t.Fatalf("expected cost=8 for fundedCount=0, got %d", cost)
	}
	if cost := def.GetCostForFundedCount(1); cost != 14 {
		t.Fatalf("expected cost=14 for fundedCount=1, got %d", cost)
	}
	if cost := def.GetCostForFundedCount(2); cost != 20 {
		t.Fatalf("expected cost=20 for fundedCount=2, got %d", cost)
	}
	if cost := def.GetCostForFundedCount(3); cost != 20 {
		t.Fatalf("expected cost=20 for fundedCount=3 (boundary), got %d", cost)
	}
}

func TestGetRewardVP(t *testing.T) {
	def := &award.AwardDefinition{
		ID:   "test-reward",
		Name: "Test Reward Award",
		Rewards: []award.AwardReward{
			{Place: 1, Outputs: []award.RewardOutput{{Type: "vp", Amount: 5}}},
			{Place: 2, Outputs: []award.RewardOutput{{Type: "vp", Amount: 2}}},
		},
	}

	if vp := def.GetRewardVP(1); vp != 5 {
		t.Fatalf("expected VP=5 for place=1, got %d", vp)
	}
	if vp := def.GetRewardVP(2); vp != 2 {
		t.Fatalf("expected VP=2 for place=2, got %d", vp)
	}
	if vp := def.GetRewardVP(3); vp != 0 {
		t.Fatalf("expected VP=0 for place=3 (no reward), got %d", vp)
	}
}
