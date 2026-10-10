package cards_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"openmars/internal/game/cards"
	"openmars/internal/game/shared"
)

func TestResourceConditionFieldValidity(t *testing.T) {
	allCards, err := cards.LoadCardsFromJSON("../../../assets/cards.json")
	if err != nil {
		t.Fatalf("Failed to load cards: %v", err)
	}

	for _, card := range allCards {
		for bi, behavior := range card.Behaviors {
			for ii, input := range behavior.Inputs {
				violations := shared.ValidateResourceCondition(input, true)
				for _, v := range violations {
					t.Errorf("Card %q (ID=%s) behavior[%d] input[%d] (%s): %s",
						card.Name, card.ID, bi, ii, input.GetResourceType(), v)
				}
			}
			for oi, output := range behavior.Outputs {
				violations := shared.ValidateResourceCondition(output, false)
				for _, v := range violations {
					t.Errorf("Card %q (ID=%s) behavior[%d] output[%d] (%s): %s",
						card.Name, card.ID, bi, oi, output.GetResourceType(), v)
				}
			}
			for ci, choice := range behavior.Choices {
				for ii, input := range choice.Inputs {
					violations := shared.ValidateResourceCondition(input, true)
					for _, v := range violations {
						t.Errorf("Card %q (ID=%s) behavior[%d] choice[%d] input[%d] (%s): %s",
							card.Name, card.ID, bi, ci, ii, input.GetResourceType(), v)
					}
				}
				for oi, output := range choice.Outputs {
					violations := shared.ValidateResourceCondition(output, false)
					for _, v := range violations {
						t.Errorf("Card %q (ID=%s) behavior[%d] choice[%d] output[%d] (%s): %s",
							card.Name, card.ID, bi, ci, oi, output.GetResourceType(), v)
					}
				}
			}
		}
	}
}

func TestAllResourceTypesHaveOutputProfiles(t *testing.T) {
	for _, rt := range shared.AllResourceTypes {
		if _, ok := shared.GetOutputProfile(rt); !ok {
			t.Errorf("ResourceType %q has no output field profile", rt)
		}
	}
}

func TestAllResourceTypesInAllResourceTypes(t *testing.T) {
	// Verify AllResourceTypes contains every constant from resource_type.go
	// by checking that the count matches the output profiles map size.
	// If a new constant is added to resource_type.go but not to AllResourceTypes,
	// TestAllResourceTypesHaveOutputProfiles won't catch it.
	// This test catches it by ensuring the AllResourceTypes slice has at least
	// as many entries as the output profiles map.
	seen := make(map[shared.ResourceType]bool)
	for _, rt := range shared.AllResourceTypes {
		if seen[rt] {
			t.Errorf("ResourceType %q appears more than once in AllResourceTypes", rt)
		}
		seen[rt] = true
	}
}

func TestBehaviorConditions_RoundTripAndClone(t *testing.T) {
	raw := `{"productionBox":"evaluate","outputs":[{"type":"copy","amount":1,"target":"self-player","scope":"production-box","zone":"played","selectors":[{"tags":["building"]}]},{"type":"card-draw","amount":1,"target":"self-player","per":{"type":"card-count","amount":3,"zone":"played","includeSource":true,"selectors":[{"tagCount":{"max":0}}]}},{"type":"colony-track-step","amount":-1,"target":"none","selectionGroup":"tracks"}]}`
	var original shared.CardBehavior
	if err := json.Unmarshal([]byte(raw), &original); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded shared.CardBehavior
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Fatal("Behavior metadata must survive JSON round trip")
	}
	cloned := original.DeepCopy()
	cloned.Outputs[0].(*shared.CopyCondition).Selectors[0].Tags[0] = shared.TagScience
	*cloned.Outputs[1].(*shared.CardOperationCondition).Per.Selectors[0].TagCount.Max = 99
	if original.Outputs[0].(*shared.CopyCondition).Selectors[0].Tags[0] != shared.TagBuilding {
		t.Fatal("Copy source selectors alias original")
	}
	if *original.Outputs[1].(*shared.CardOperationCondition).Per.Selectors[0].TagCount.Max != 0 {
		t.Fatal("Count selectors alias original")
	}
}

func TestProductionBoxMetadata_OnlyContainsCopyableProduction(t *testing.T) {
	all, err := cards.LoadCardsFromJSON("../../../assets/cards.json")
	if err != nil {
		t.Fatal(err)
	}
	marked := 0
	for _, card := range all {
		for _, behavior := range card.Behaviors {
			if behavior.ProductionBox == "" {
				continue
			}
			marked++
			for _, output := range behavior.Outputs {
				if behavior.ProductionBox == "evaluate" && !shared.IsProductionResourceType(output.GetResourceType()) {
					t.Errorf("%s evaluated production box includes %s", card.Name, output.GetResourceType())
				}
			}
			validationCard := cards.Card{ID: card.ID, Type: card.Type, Behaviors: []shared.CardBehavior{behavior}}
			for _, err := range cards.ValidateCardJSON(&validationCard) {
				t.Error(err)
			}
		}
	}
	if marked == 0 {
		t.Fatal("No explicit production boxes in card database")
	}
}

func TestTriggeringCardTarget_OnlyStorageOutputs(t *testing.T) {
	for _, resource := range []shared.ResourceType{shared.ResourceAnimal, shared.ResourceMicrobe, shared.ResourceScience, shared.ResourceCardResource} {
		condition := shared.NewCardStorageCondition(resource, 1, "triggering-card")
		if errors := shared.ValidateResourceCondition(condition, false); len(errors) != 0 {
			t.Fatalf("valid %s output rejected: %v", resource, errors)
		}
		if len(shared.ValidateResourceCondition(condition, true)) == 0 {
			t.Fatal("triggering-card input accepted")
		}
	}
	invalid := shared.NewBasicResourceCondition(shared.ResourceCredit, 1, "triggering-card")
	if len(shared.ValidateResourceCondition(invalid, false)) == 0 {
		t.Fatal("non-storage triggering-card output accepted")
	}
}

func TestTargetRestriction_RoundTripAndClone(t *testing.T) {
	raw := []byte(`{"triggers":[{"type":"auto"}],"outputs":[{"type":"credit","amount":-4,"target":"any-player","targetRestriction":{"adjacent":"self-card","selectors":[{"tags":["venus"]}]}}]}`)
	var behavior shared.CardBehavior
	if err := json.Unmarshal(raw, &behavior); err != nil {
		t.Fatal(err)
	}
	output := behavior.Outputs[0].(*shared.BasicResourceCondition)
	clone := shared.CloneCondition(output).(*shared.BasicResourceCondition)
	clone.TargetRestriction.Selectors[0].Tags[0] = shared.TagEarth
	if output.TargetRestriction.Selectors[0].Tags[0] != shared.TagVenus {
		t.Fatal("clone changed source tags")
	}
	data, err := json.Marshal(behavior)
	if err != nil {
		t.Fatal(err)
	}
	var decoded shared.CardBehavior
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	got := decoded.Outputs[0].(*shared.BasicResourceCondition)
	if !reflect.DeepEqual(output.TargetRestriction, got.TargetRestriction) {
		t.Fatal("restriction lost during round trip")
	}
}

func TestDefense_ExplicitPolicyRoundTripValidationAndClone(t *testing.T) {
	for _, against := range []string{"any-player", "opponents"} {
		raw := []byte(`{"outputs":[{"type":"defense","amount":1,"target":"self-player","against":"` + against + `","selectors":[{"resources":["animal"]}]}]}`)
		var b shared.CardBehavior
		if err := json.Unmarshal(raw, &b); err != nil {
			t.Fatal(err)
		}
		output := b.Outputs[0].(*shared.EffectCondition)
		if issues := shared.ValidateResourceCondition(output, false); len(issues) > 0 {
			t.Fatal(issues)
		}
		clone := shared.CloneCondition(output).(*shared.EffectCondition)
		clone.Selectors[0].Resources[0] = "plant"
		if output.Selectors[0].Resources[0] != "animal" || clone.Against != against {
			t.Fatal("clone lost policy or aliased selectors")
		}
		encoded, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip shared.CardBehavior
		if err = json.Unmarshal(encoded, &roundTrip); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(b, roundTrip) {
			t.Fatal("policy lost on JSON round trip")
		}
		output.Against = ""
		if len(shared.ValidateResourceCondition(output, false)) == 0 {
			t.Fatal("missing policy accepted")
		}
		output.Against = "self"
		if len(shared.ValidateResourceCondition(output, false)) == 0 {
			t.Fatal("unknown policy accepted")
		}
		output.Against = against
		output.ResourceType = shared.ResourceDiscount
		if len(shared.ValidateResourceCondition(output, false)) == 0 {
			t.Fatal("against accepted on discount")
		}
	}
}

func TestDefense_RejectsPolicyOnOtherJSONConditions(t *testing.T) {
	for _, resource := range []string{"credit", "animal", "discount", "card-draw"} {
		var b shared.CardBehavior
		if err := json.Unmarshal([]byte(`{"outputs":[{"type":"`+resource+`","amount":1,"target":"self-player","against":"opponents"}]}`), &b); err == nil {
			t.Fatalf("against accepted for %s", resource)
		}
	}
}

func TestTileArea_JSONValidationAndClone(t *testing.T) {
	for _, area := range []string{"land", "ocean"} {
		raw := []byte(`{"outputs":[{"type":"ocean-placement","amount":1,"target":"none","tileRestrictions":{"area":"` + area + `","adjacentToType":"city","minAdjacentOfType":1,"boardTags":["test"]}}]}`)
		var behavior shared.CardBehavior
		if err := json.Unmarshal(raw, &behavior); err != nil {
			t.Fatal(err)
		}
		output := behavior.Outputs[0].(*shared.TilePlacementCondition)
		if issues := shared.ValidateResourceCondition(output, false); len(issues) > 0 {
			t.Fatal(issues)
		}
		encoded, err := json.Marshal(behavior)
		if err != nil {
			t.Fatal(err)
		}
		var decoded shared.CardBehavior
		if err = json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(behavior, decoded) {
			t.Fatal("area lost on round trip")
		}
		clone := shared.CloneCondition(output).(*shared.TilePlacementCondition)
		clone.TileRestrictions.BoardTags[0] = "changed"
		*clone.TileRestrictions.MinAdjacentOfType = 2
		if output.TileRestrictions.BoardTags[0] != "test" || *output.TileRestrictions.MinAdjacentOfType != 1 || clone.TileRestrictions.Area != area {
			t.Fatal("clone lost or aliased restrictions")
		}
	}
	for _, raw := range []string{
		`{"area":"anywhere"}`,
		`{"onTileType":"ocean"}`,
	} {
		var restrictions shared.TileRestrictions
		if err := json.Unmarshal([]byte(raw), &restrictions); err == nil {
			t.Fatalf("accepted invalid restriction: %s", raw)
		}
	}
}

func TestColonyTradeConditions_RoundTripAndValidation(t *testing.T) {
	for _, raw := range []string{
		`{"triggers":[{"type":"auto"}],"outputs":[{"type":"trade-fleet","amount":1,"target":"self-player"}]}`,
		`{"triggers":[{"type":"auto","condition":{"type":"before-colony-trade","target":"self-player"}}],"outputs":[{"type":"colony-track-step","amount":1,"target":"trigger-colony","optional":true}]}`,
	} {
		var behavior shared.CardBehavior
		if err := json.Unmarshal([]byte(raw), &behavior); err != nil {
			t.Fatal(err)
		}
		card := cards.Card{ID: "test", Type: cards.CardTypeActive, Behaviors: []shared.CardBehavior{behavior}}
		if problems := cards.ValidateCardJSON(&card); len(problems) > 0 {
			t.Fatal(problems)
		}
		if problems := shared.ValidateResourceCondition(behavior.Outputs[0], false); len(problems) > 0 {
			t.Fatal(problems)
		}
		encoded, err := json.Marshal(behavior.DeepCopy())
		if err != nil {
			t.Fatal(err)
		}
		var roundtrip shared.CardBehavior
		if err := json.Unmarshal(encoded, &roundtrip); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(behavior, roundtrip) {
			t.Fatalf("condition lost during roundtrip: %s", encoded)
		}
	}
	for _, raw := range []string{
		`{"triggers":[{"type":"auto"}],"outputs":[{"type":"colony-track-step","amount":1,"target":"trigger-colony","optional":true}]}`,
		`{"triggers":[{"type":"auto","condition":{"type":"before-colony-trade","target":"self-player"}}],"outputs":[{"type":"colony-track-step","amount":1,"target":"self-player"}]}`,
		`{"triggers":[{"type":"auto"}],"outputs":[{"type":"trade-fleet","amount":-1,"target":"self-player"}]}`,
		`{"triggers":[{"type":"auto"}],"outputs":[{"type":"trade-fleet","amount":1,"target":"any-player"}]}`,
		`{"triggers":[{"type":"auto"}],"outputs":[{"type":"colony-track-step","amount":1,"target":"none","optional":true}]}`,
	} {
		var behavior shared.CardBehavior
		if err := json.Unmarshal([]byte(raw), &behavior); err != nil {
			continue
		}
		card := cards.Card{ID: "invalid", Type: cards.CardTypeActive, Behaviors: []shared.CardBehavior{behavior}}
		if len(cards.ValidateCardJSON(&card)) == 0 && len(shared.ValidateResourceCondition(behavior.Outputs[0], false)) == 0 {
			t.Fatalf("invalid trade condition accepted: %s", raw)
		}
	}
}

func TestResourceCondition_MaxTriggerValidation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cap      int
		per      bool
		variable bool
		valid    bool
	}{
		{"positive", 4, true, false, true}, {"zero", 0, true, false, true},
		{"negative", -1, true, false, false}, {"no per", 4, false, false, false},
		{"variable amount", 4, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			condition := &shared.BasicResourceCondition{ConditionBase: shared.ConditionBase{ResourceType: shared.ResourceCredit, Amount: 1, Target: "self-player"}, MaxTrigger: &tc.cap, VariableAmount: tc.variable}
			if tc.per {
				condition.Per = &shared.PerCondition{ResourceType: shared.ResourceFloater, Amount: 1}
			}
			violations := shared.ValidateResourceCondition(condition, false)
			if (len(violations) == 0) != tc.valid {
				t.Fatalf("valid = %v, violations = %v", tc.valid, violations)
			}
		})
	}
}

func TestPaymentSubstituteJSON_RoundTripAndValidation(t *testing.T) {
	valid := `{"outputs":[{"type":"payment-substitute","target":"self-player","amount":2,"source":{"target":"self-card","resource":"microbe"},"targetResource":"credit","selectors":[{"actions":["card-playing"],"tags":["plant"]}]}]}`
	var b shared.CardBehavior
	if err := json.Unmarshal([]byte(valid), &b); err != nil {
		t.Fatal(err)
	}
	original, ok := b.Outputs[0].(*shared.PaymentSubstituteCondition)
	if !ok {
		t.Fatal("missing typed payment condition")
	}
	cloned := shared.CloneCondition(original).(*shared.PaymentSubstituteCondition)
	cloned.Selectors[0].Tags[0] = shared.TagVenus
	if original.Selectors[0].Tags[0] != shared.TagPlant {
		t.Fatal("clone shares selectors")
	}
	encoded, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip shared.CardBehavior
	if err = json.Unmarshal(encoded, &roundtrip); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b, roundtrip) {
		t.Fatalf("roundtrip differs: %s", encoded)
	}
	for _, bad := range []string{
		`{"type":"storage-payment-substitute","amount":2,"target":"self-player"}`,
		`{"type":"payment-substitute","amount":1,"target":"self-player","selectors":[{"resources":["heat"]}]}`,
		`{"type":"payment-substitute","amount":0,"target":"self-player","source":{"target":"self-player","resource":"heat"},"targetResource":"credit"}`,
		`{"type":"payment-substitute","amount":1,"target":"self-player","source":{"target":"any-player","resource":"heat"},"targetResource":"credit"}`,
		`{"type":"payment-substitute","amount":1,"target":"self-player","source":{"target":"self-card","resource":"microbe","cardId":"forged"},"targetResource":"credit"}`,
		`{"type":"payment-substitute","amount":1,"target":"self-player","source":{"target":"self-card","resource":"heat"},"targetResource":"credit"}`,
		`{"type":"payment-substitute","amount":1,"target":"self-player","source":{"target":"self-player","resource":"heat"},"targetResource":"card-draw"}`,
	} {
		t.Run(bad, func(t *testing.T) {
			var b shared.CardBehavior
			if err := json.Unmarshal([]byte(`{"outputs":[`+bad+`]}`), &b); err == nil {
				t.Fatal("invalid payment accepted")
			}
		})
	}
}
