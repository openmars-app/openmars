package player

import (
	"log/slog"
	"time"

	"openmars/internal/events"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
)

// VPRecalculationContext provides data needed to recalculate VP
type VPRecalculationContext interface {
	GetCardStorage(playerID string, cardID string) int
	CountPlayerTagsByType(playerID string, tagType shared.CardTag) int
	CountAllTilesOfType(tileType shared.ResourceType) int
	CountPlayerTilesOfType(playerID string, tileType shared.ResourceType) int
	CountAdjacentTilesForCard(cardID string, tileType shared.ResourceType) int
	CountAdjacentTilesToTileType(playerID string, countType, adjacentToType shared.ResourceType) int
	CountAllColonies() int
	ModeRequirementsMet([]shared.StateRequirement) bool
	CountPermanentProductionTypes(playerID string) int
}

// VPGranters manages VP-granting cards.
type VPGranters struct {
	ds       *datastore.DataStore
	eventBus *events.EventBusImpl
	gameID   string
	playerID string
}

// NewVPGranters creates a new VPGranters view backed by the DataStore.
func NewVPGranters(ds *datastore.DataStore, eventBus *events.EventBusImpl, gameID, playerID string) *VPGranters {
	return &VPGranters{
		ds:       ds,
		eventBus: eventBus,
		gameID:   gameID,
		playerID: playerID,
	}
}

func (vg *VPGranters) update(fn func(s *datastore.PlayerState)) {
	if err := vg.ds.UpdatePlayer(vg.gameID, vg.playerID, fn); err != nil {
		slog.Default().Warn("Failed to update player state", slog.String("game_id", vg.gameID), slog.String("player_id", vg.playerID), slog.Any("error", err))
	}
}

func (vg *VPGranters) read(fn func(s *datastore.PlayerState)) {
	if err := vg.ds.ReadPlayer(vg.gameID, vg.playerID, fn); err != nil {
		slog.Default().Warn("Failed to read player state", slog.String("game_id", vg.gameID), slog.String("player_id", vg.playerID), slog.Any("error", err))
	}
}

func (vg *VPGranters) Add(granter shared.VPGranter) {
	vg.update(func(s *datastore.PlayerState) {
		s.VPGranters = append(s.VPGranters, granter)
	})
}

func (vg *VPGranters) Prepend(granter shared.VPGranter) {
	vg.update(func(s *datastore.PlayerState) {
		s.VPGranters = append([]shared.VPGranter{granter}, s.VPGranters...)
	})
}

// RemoveByCardID removes all VP granters for the given card ID.
func (vg *VPGranters) RemoveByCardID(cardID string) {
	vg.update(func(s *datastore.PlayerState) {
		filtered := s.VPGranters[:0]
		for _, g := range s.VPGranters {
			if g.CardID != cardID {
				filtered = append(filtered, g)
			}
		}
		s.VPGranters = filtered
	})
}

func (vg *VPGranters) GetAll() []shared.VPGranter {
	var result []shared.VPGranter
	vg.read(func(s *datastore.PlayerState) {
		result = make([]shared.VPGranter, len(s.VPGranters))
		copy(result, s.VPGranters)
	})
	return result
}

func (vg *VPGranters) TotalComputedVP() int {
	var total int
	vg.read(func(s *datastore.PlayerState) {
		for _, g := range s.VPGranters {
			total += g.ComputedValue
		}
	})
	return total
}

const vpTargetSelfCard = "self-card"

func (vg *VPGranters) RecalculateAll(ctx VPRecalculationContext) {
	var oldTotal, newTotal int
	vg.update(func(s *datastore.PlayerState) {
		for _, g := range s.VPGranters {
			oldTotal += g.ComputedValue
		}

		for i := range s.VPGranters {
			s.VPGranters[i].ComputedValue = evaluateGranterVP(&s.VPGranters[i], vg.playerID, ctx)
		}

		for _, g := range s.VPGranters {
			newTotal += g.ComputedValue
		}
	})

	if oldTotal != newTotal {
		events.Publish(vg.eventBus, events.VictoryPointsChangedEvent{
			GameID:    vg.gameID,
			PlayerID:  vg.playerID,
			OldPoints: oldTotal,
			NewPoints: newTotal,
			Source:    "vp-granters",
			Timestamp: time.Now(),
		})
	}
}

func evaluateGranterVP(granter *shared.VPGranter, playerID string, ctx VPRecalculationContext) int {
	total := 0
	for _, cond := range granter.VPConditions {
		total += evaluateVPCondition(cond, granter.CardID, playerID, ctx)
	}
	return total
}

func evaluateVPCondition(cond shared.VPCondition, cardID string, playerID string, ctx VPRecalculationContext) int {
	if !ctx.ModeRequirementsMet(cond.StateRequirements) {
		return 0
	}
	switch cond.Condition {
	case "fixed", "once":
		return cond.Amount

	case "per":
		if cond.Per == nil {
			return 0
		}
		count := countPerCondition(cond.Per, cardID, playerID, ctx)
		if cond.Per.Amount <= 0 {
			return 0
		}
		triggers := count / cond.Per.Amount

		if cond.MaxTrigger != nil && *cond.MaxTrigger >= 0 && triggers > *cond.MaxTrigger {
			triggers = *cond.MaxTrigger
		}

		return cond.Amount * triggers

	default:
		return 0
	}
}

func countPerCondition(per *shared.PerCondition, cardID string, playerID string, ctx VPRecalculationContext) int {
	if per.Target != nil && *per.Target == vpTargetSelfCard {
		return ctx.GetCardStorage(playerID, cardID)
	}

	if per.AdjacentToSelfTile {
		return ctx.CountAdjacentTilesForCard(cardID, per.ResourceType)
	}

	if per.AdjacentToTileType != nil {
		return ctx.CountAdjacentTilesToTileType(playerID, per.ResourceType, *per.AdjacentToTileType)
	}

	if per.Tag != nil {
		return ctx.CountPlayerTagsByType(playerID, *per.Tag)
	}

	switch per.ResourceType {
	case shared.ResourcePermanentProductionTypes:
		return ctx.CountPermanentProductionTypes(playerID)
	case shared.ResourceCityTile:
		if per.Target != nil && *per.Target == "self-player" {
			return ctx.CountPlayerTilesOfType(playerID, shared.ResourceCityTile)
		}
		return ctx.CountAllTilesOfType(per.ResourceType)
	case shared.ResourceGreeneryTile:
		if per.Target != nil && *per.Target == "self-player" {
			return ctx.CountPlayerTilesOfType(playerID, shared.ResourceGreeneryTile)
		}
		return ctx.CountAllTilesOfType(per.ResourceType)
	case shared.ResourceOceanTile:
		return ctx.CountAllTilesOfType(per.ResourceType)
	case shared.ResourceColony:
		return ctx.CountAllColonies()
	default:
		return ctx.CountPlayerTagsByType(playerID, shared.CardTag(per.ResourceType))
	}
}
