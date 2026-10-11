package dto_test

import (
	"encoding/json"
	"slices"
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/internal/game/cards"
	"openmars/test/testutil"
)

func TestCardDto_PreservesCustomStoredResourcePresentation(t *testing.T) {
	var card cards.Card
	testutil.AssertNoError(t, json.Unmarshal([]byte(`{
		"id": "custom-storage", "name": "Supply storage", "type": "active",
		"resourceStorage": {
			"type": "card-resource", "starting": 0, "capacity": 6,
			"icon": "reserve", "label": "Reserves", "description": "Reserve crates"
		}
	}`), &card), "decode authored storage metadata")
	storage := dto.ToCardDto(card).ResourceStorage
	if storage == nil {
		t.Fatal("storage metadata missing from card API")
	}
	testutil.AssertEqual(t, "card-resource", string(storage.Type), "visual metadata must not change the engine resource type")
	testutil.AssertEqual(t, "reserve", storage.Icon, "authored icon reaches card views")
	testutil.AssertEqual(t, "Reserves", storage.Label, "authored label reaches card views")
	if storage.Capacity == nil || *storage.Capacity != 6 {
		t.Fatal("custom presentation must retain storage capacity")
	}
}

func TestCardDto_PreservesCombinedTagCount(t *testing.T) {
	card := testutil.GetCardByName("Gyropolis")
	encoded, err := json.Marshal(dto.ToCardDto(card))
	testutil.AssertNoError(t, err, "marshal card")
	var result struct {
		Behaviors []struct {
			Outputs []struct {
				Type string `json:"type"`
				Per  struct {
					Tags []string `json:"tags"`
				} `json:"per"`
			} `json:"outputs"`
		} `json:"behaviors"`
	}
	testutil.AssertNoError(t, json.Unmarshal(encoded, &result), "decode API data")
	found := false
	for _, behavior := range result.Behaviors {
		for _, output := range behavior.Outputs {
			if output.Type == "credit-production" {
				if found {
					t.Fatal("production should use one combined count")
				}
				found = true
				testutil.AssertTrue(t, slices.Equal([]string{"earth", "venus"}, output.Per.Tags), "both icons reach frontend")
			}
		}
	}
	if !found {
		t.Fatal("missing production output")
	}
}
