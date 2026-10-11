package awards_test

import (
	"path/filepath"
	"runtime"
	"testing"

	"openmars/internal/game/award"
	"openmars/test/testutil"
)

func jsonPath() string {
	_, currentFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(currentFile), "..", "..", "assets", "awards.json")
}

func TestLoadAwardsFromJSON(t *testing.T) {
	defs, err := award.LoadAwardsFromJSON(jsonPath())
	testutil.AssertNoError(t, err, "Should load awards from JSON")
	testutil.AssertEqual(t, 16, len(defs), "Should have exactly 16 awards")

	testutil.AssertEqual(t, "landlord", defs[0].ID, "First award should be landlord")
	testutil.AssertEqual(t, "venuphile", defs[5].ID, "Sixth award should be venuphile")

	for _, def := range defs {
		testutil.AssertTrue(t, def.Name != "", "Award name should not be empty for "+def.ID)
		testutil.AssertTrue(t, def.Description != "", "Award description should not be empty for "+def.ID)
		testutil.AssertTrue(t, def.GameModes != nil, "Award game mode membership must be explicit for "+def.ID)
		testutil.AssertTrue(t, len(def.Costs) > 0, "Award should have costs for "+def.ID)
		testutil.AssertTrue(t, len(def.Rewards) > 0, "Award should have rewards for "+def.ID)
	}
}

func TestRegistryGetByID(t *testing.T) {
	defs, err := award.LoadAwardsFromJSON(jsonPath())
	testutil.AssertNoError(t, err, "Should load awards from JSON")
	registry := award.NewInMemoryAwardRegistry(defs)

	def, err := registry.GetByID("landlord")
	testutil.AssertNoError(t, err, "Should find landlord award")
	testutil.AssertEqual(t, "Landlord", def.Name, "Name should match")

	_, err = registry.GetByID("nonexistent")
	testutil.AssertErrorContains(t, err, "award not found", "Should fail for unknown ID")
}

func TestRegistryGetAllOrder(t *testing.T) {
	defs, err := award.LoadAwardsFromJSON(jsonPath())
	testutil.AssertNoError(t, err, "Should load awards from JSON")
	registry := award.NewInMemoryAwardRegistry(defs)

	all := registry.GetAll()
	testutil.AssertEqual(t, 16, len(all), "GetAll should return 16 awards")

	expectedOrder := []string{"landlord", "banker", "scientist", "thermalist", "miner", "venuphile"}
	for i, id := range expectedOrder {
		testutil.AssertEqual(t, id, all[i].ID, "Award at index should match expected order")
	}
}
