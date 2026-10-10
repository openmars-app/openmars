package player

import (
	"log/slog"
	"time"

	"openmars/internal/events"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
)

// PlayerResources manages player resources, production, and scoring.
type PlayerResources struct {
	ds       *datastore.DataStore
	eventBus *events.EventBusImpl
	gameID   string
	playerID string
}

func newResources(ds *datastore.DataStore, eventBus *events.EventBusImpl, gameID, playerID string) *PlayerResources {
	return &PlayerResources{
		ds:       ds,
		eventBus: eventBus,
		gameID:   gameID,
		playerID: playerID,
	}
}

func (r *PlayerResources) update(fn func(s *datastore.PlayerState)) {
	if err := r.ds.UpdatePlayer(r.gameID, r.playerID, fn); err != nil {
		slog.Default().Warn("Failed to update player state", slog.String("game_id", r.gameID), slog.String("player_id", r.playerID), slog.Any("error", err))
	}
}

func (r *PlayerResources) read(fn func(s *datastore.PlayerState)) {
	if err := r.ds.ReadPlayer(r.gameID, r.playerID, fn); err != nil {
		slog.Default().Warn("Failed to read player state", slog.String("game_id", r.gameID), slog.String("player_id", r.playerID), slog.Any("error", err))
	}
}

func (r *PlayerResources) Get() shared.Resources {
	var res shared.Resources
	r.read(func(s *datastore.PlayerState) {
		res = s.Resources
	})
	return res
}

func (r *PlayerResources) Production() shared.Production {
	var runtime shared.ModeRuntimeState
	_ = r.ds.ReadGame(r.gameID, func(s *datastore.GameState) { runtime = s.ModeRuntime })
	var prod shared.Production
	r.read(func(s *datastore.PlayerState) {
		prod = shared.EffectiveProduction(s.Production, s.ProductionContributions, s.DependencyStatuses, runtime)
	})
	return prod
}

func (r *PlayerResources) TerraformRating() int {
	var tr int
	r.read(func(s *datastore.PlayerState) {
		tr = s.TerraformRating
	})
	return tr
}

func (r *PlayerResources) Storage() map[string]int {
	var storageCopy map[string]int
	r.read(func(s *datastore.PlayerState) {
		storageCopy = make(map[string]int, len(s.ResourceStorage))
		for k, v := range s.ResourceStorage {
			storageCopy[k] = v
		}
	})
	return storageCopy
}

const (
	baseSteelValue    = 2
	baseTitaniumValue = 3
)

// ResourceValue returns the current credit value of an explicitly permitted material.
func (r *PlayerResources) ResourceValue(rt shared.ResourceType) int {
	base := 1
	if rt == shared.ResourceSteel {
		base = baseSteelValue
	}
	if rt == shared.ResourceTitanium {
		base = baseTitaniumValue
	}
	return base + r.GetValueModifier(rt)
}

// PaymentSubstitutes returns independent copies of registered payment rules.
func (r *PlayerResources) PaymentSubstitutes() []shared.PaymentSubstitute {
	var rules []shared.PaymentSubstitute
	r.read(func(s *datastore.PlayerState) {
		for _, rule := range s.PaymentSubstitutes {
			rule.Selectors = shared.CloneSelectors(rule.Selectors)
			rules = append(rules, rule)
		}
	})
	return rules
}

// AddPaymentSubstitute registers a capability granted by a card.
func (r *PlayerResources) AddPaymentSubstitute(rule shared.PaymentSubstitute) {
	rule.Selectors = shared.CloneSelectors(rule.Selectors)
	r.update(func(s *datastore.PlayerState) { s.PaymentSubstitutes = append(s.PaymentSubstitutes, rule) })
}

// RemovePaymentSubstitutes removes only rules granted by the specified card.
func (r *PlayerResources) RemovePaymentSubstitutes(cardID string) {
	r.update(func(s *datastore.PlayerState) {
		kept := s.PaymentSubstitutes[:0]
		for _, rule := range s.PaymentSubstitutes {
			if rule.GrantedByCardID != cardID {
				kept = append(kept, rule)
			}
		}
		s.PaymentSubstitutes = kept
	})
}

// ValueModifiers returns a copy of the value modifiers map
func (r *PlayerResources) ValueModifiers() map[shared.ResourceType]int {
	var modifiersCopy map[shared.ResourceType]int
	r.read(func(s *datastore.PlayerState) {
		modifiersCopy = make(map[shared.ResourceType]int, len(s.ValueModifiers))
		for k, v := range s.ValueModifiers {
			modifiersCopy[k] = v
		}
	})
	return modifiersCopy
}

// AddValueModifier adds a value modifier for a resource type
func (r *PlayerResources) AddValueModifier(resourceType shared.ResourceType, amount int) {
	r.update(func(s *datastore.PlayerState) {
		if s.ValueModifiers == nil {
			s.ValueModifiers = make(map[shared.ResourceType]int)
		}
		s.ValueModifiers[resourceType] += amount
	})
}

// GetValueModifier returns the total value modifier for a resource type
func (r *PlayerResources) GetValueModifier(resourceType shared.ResourceType) int {
	var val int
	r.read(func(s *datastore.PlayerState) {
		val = s.ValueModifiers[resourceType]
	})
	return val
}

func (r *PlayerResources) Set(resources shared.Resources) {
	r.update(func(s *datastore.PlayerState) {
		s.Resources = resources
	})

	if r.eventBus != nil {
		events.Publish(r.eventBus, events.ResourcesChangedEvent{
			GameID:    r.gameID,
			PlayerID:  r.playerID,
			Changes:   make(map[string]int),
			Timestamp: time.Now(),
		})
	}
}

func (r *PlayerResources) SetProduction(production shared.Production) {
	var oldProduction, newProduction shared.Production
	r.update(func(s *datastore.PlayerState) {
		oldProduction = s.Production
		s.Production = production
		newProduction = s.Production
	})

	if r.eventBus != nil {
		resourceTypes := []struct {
			name     string
			oldValue int
			newValue int
		}{
			{"credits", oldProduction.Credits, newProduction.Credits},
			{"steel", oldProduction.Steel, newProduction.Steel},
			{"titanium", oldProduction.Titanium, newProduction.Titanium},
			{"plants", oldProduction.Plants, newProduction.Plants},
			{"energy", oldProduction.Energy, newProduction.Energy},
			{"heat", oldProduction.Heat, newProduction.Heat},
		}

		for _, rt := range resourceTypes {
			events.Publish(r.eventBus, events.ProductionChangedEvent{
				GameID:        r.gameID,
				PlayerID:      r.playerID,
				ResourceType:  rt.name,
				OldProduction: rt.oldValue,
				NewProduction: rt.newValue,
				Timestamp:     time.Now(),
			})
		}
	}
}

func (r *PlayerResources) SetTerraformRating(tr int) {
	var oldRating int
	r.update(func(s *datastore.PlayerState) {
		oldRating = s.TerraformRating
		s.TerraformRating = tr
	})

	if r.eventBus != nil {
		events.Publish(r.eventBus, events.TerraformRatingChangedEvent{
			GameID:    r.gameID,
			PlayerID:  r.playerID,
			OldRating: oldRating,
			NewRating: tr,
			Timestamp: time.Now(),
		})
	}
}

func (r *PlayerResources) Add(changes map[shared.ResourceType]int) {
	r.update(func(s *datastore.PlayerState) {
		for resourceType, amount := range changes {
			switch resourceType {
			case shared.ResourceCredit:
				s.Resources.Credits += amount
			case shared.ResourceSteel:
				s.Resources.Steel += amount
			case shared.ResourceTitanium:
				s.Resources.Titanium += amount
			case shared.ResourcePlant:
				s.Resources.Plants += amount
			case shared.ResourceEnergy:
				s.Resources.Energy += amount
			case shared.ResourceHeat:
				s.Resources.Heat += amount
			}
		}
	})

	if r.eventBus != nil {
		changesMap := make(map[string]int, len(changes))
		for resourceType, amount := range changes {
			changesMap[string(resourceType)] = amount
		}

		events.Publish(r.eventBus, events.ResourcesChangedEvent{
			GameID:    r.gameID,
			PlayerID:  r.playerID,
			Changes:   changesMap,
			Timestamp: time.Now(),
		})
	}
}

func (r *PlayerResources) AddProduction(changes map[shared.ResourceType]int) {
	var oldProduction, newProduction shared.Production
	r.update(func(s *datastore.PlayerState) {
		oldProduction = s.Production
		for resourceType, amount := range changes {
			switch resourceType {
			case shared.ResourceCreditProduction:
				s.Production.Credits += amount
				if s.Production.Credits < shared.MinCreditProduction {
					s.Production.Credits = shared.MinCreditProduction
				}
			case shared.ResourceSteelProduction:
				s.Production.Steel += amount
				if s.Production.Steel < shared.MinOtherProduction {
					s.Production.Steel = shared.MinOtherProduction
				}
			case shared.ResourceTitaniumProduction:
				s.Production.Titanium += amount
				if s.Production.Titanium < shared.MinOtherProduction {
					s.Production.Titanium = shared.MinOtherProduction
				}
			case shared.ResourcePlantProduction:
				s.Production.Plants += amount
				if s.Production.Plants < shared.MinOtherProduction {
					s.Production.Plants = shared.MinOtherProduction
				}
			case shared.ResourceEnergyProduction:
				s.Production.Energy += amount
				if s.Production.Energy < shared.MinOtherProduction {
					s.Production.Energy = shared.MinOtherProduction
				}
			case shared.ResourceHeatProduction:
				s.Production.Heat += amount
				if s.Production.Heat < shared.MinOtherProduction {
					s.Production.Heat = shared.MinOtherProduction
				}
			}
		}
		newProduction = s.Production
	})

	if r.eventBus != nil {
		for resourceType := range changes {
			var oldValue, newValue int
			resourceName := string(resourceType)

			switch resourceType {
			case shared.ResourceCreditProduction:
				oldValue = oldProduction.Credits
				newValue = newProduction.Credits
				resourceName = "credits"
			case shared.ResourceSteelProduction:
				oldValue = oldProduction.Steel
				newValue = newProduction.Steel
				resourceName = "steel"
			case shared.ResourceTitaniumProduction:
				oldValue = oldProduction.Titanium
				newValue = newProduction.Titanium
				resourceName = "titanium"
			case shared.ResourcePlantProduction:
				oldValue = oldProduction.Plants
				newValue = newProduction.Plants
				resourceName = "plants"
			case shared.ResourceEnergyProduction:
				oldValue = oldProduction.Energy
				newValue = newProduction.Energy
				resourceName = "energy"
			case shared.ResourceHeatProduction:
				oldValue = oldProduction.Heat
				newValue = newProduction.Heat
				resourceName = "heat"
			}

			events.Publish(r.eventBus, events.ProductionChangedEvent{
				GameID:        r.gameID,
				PlayerID:      r.playerID,
				ResourceType:  resourceName,
				OldProduction: oldValue,
				NewProduction: newValue,
				Timestamp:     time.Now(),
			})
		}
	}
}

func (r *PlayerResources) UpdateTerraformRating(delta int) {
	var oldRating, newRating int
	r.update(func(s *datastore.PlayerState) {
		oldRating = s.TerraformRating
		s.TerraformRating += delta
		newRating = s.TerraformRating
	})

	if r.eventBus != nil {
		events.Publish(r.eventBus, events.TerraformRatingChangedEvent{
			GameID:    r.gameID,
			PlayerID:  r.playerID,
			OldRating: oldRating,
			NewRating: newRating,
			Timestamp: time.Now(),
		})
	}
}

// AddToStorage adds resources to a specific card's storage
func (r *PlayerResources) AddToStorage(cardID string, amount int) {
	var oldAmount, newAmount int
	r.update(func(s *datastore.PlayerState) {
		if s.ResourceStorage == nil {
			s.ResourceStorage = make(map[string]int)
		}
		oldAmount = s.ResourceStorage[cardID]
		s.ResourceStorage[cardID] += amount
		newAmount = s.ResourceStorage[cardID]
	})

	if r.eventBus != nil {
		events.Publish(r.eventBus, events.ResourceStorageChangedEvent{
			GameID:    r.gameID,
			PlayerID:  r.playerID,
			CardID:    cardID,
			OldAmount: oldAmount,
			NewAmount: newAmount,
			Timestamp: time.Now(),
		})
	}
}

// GetCardStorage returns the amount of resources stored on a specific card
func (r *PlayerResources) GetCardStorage(cardID string) int {
	var val int
	r.read(func(s *datastore.PlayerState) {
		val = s.ResourceStorage[cardID]
	})
	return val
}

// RemoveCardStorage removes the storage entry for a specific card
func (r *PlayerResources) RemoveCardStorage(cardID string) {
	r.update(func(s *datastore.PlayerState) {
		delete(s.ResourceStorage, cardID)
	})
}

// ClearValueModifiers resets all value modifiers to zero
func (r *PlayerResources) ClearValueModifiers() {
	r.update(func(s *datastore.PlayerState) {
		s.ValueModifiers = make(map[shared.ResourceType]int)
	})
}

// RecordProductionBox remembers production determined by a source's one-time choice.
func (r *PlayerResources) RecordProductionBox(cardID string, output shared.ProductionCondition) {
	r.update(func(s *datastore.PlayerState) {
		if s.ResolvedProductionBoxes == nil {
			s.ResolvedProductionBoxes = map[string][]shared.ProductionCondition{}
		}
		s.ResolvedProductionBoxes[cardID] = append(s.ResolvedProductionBoxes[cardID], output)
	})
}

// ResolvedProductionBox returns the production originally resolved for a card.
func (r *PlayerResources) ResolvedProductionBox(cardID string) []shared.ProductionCondition {
	var result []shared.ProductionCondition
	r.read(func(s *datastore.PlayerState) { result = append(result, s.ResolvedProductionBoxes[cardID]...) })
	return result
}
