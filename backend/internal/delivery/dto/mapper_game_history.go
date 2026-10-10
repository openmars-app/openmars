package dto

import (
	"openmars/internal/game/cards"
	"openmars/internal/game/datastore"
	"openmars/internal/game/shared"
)

// ToGameHistoryEntryDtos converts a slice of history entries to DTOs.
func ToGameHistoryEntryDtos(entries []*datastore.GameStateHistoryEntry, registry cards.CardRegistry) []GameHistoryEntryDto {
	dtos := make([]GameHistoryEntryDto, len(entries))
	for i, entry := range entries {
		dtos[i] = toGameHistoryEntryDto(entry, registry)
	}
	return dtos
}

func toGameHistoryEntryDto(entry *datastore.GameStateHistoryEntry, registry cards.CardRegistry) GameHistoryEntryDto {
	s := entry.State
	if registry != nil && len(cards.ModeDefinitions(registry)) > 0 {
		effective, err := cards.Resolve(registry, s.Settings)
		if err == nil {
			registry = effective
		} else {
			registry = nil
		}
	}

	tileDtos := make([]TileDto, len(s.Tiles))
	for i, tile := range s.Tiles {
		tileDtos[i] = TileDto{
			Coordinates: HexPositionDto{
				Q: tile.Coordinates.Q,
				R: tile.Coordinates.R,
				S: tile.Coordinates.S,
			},
			Type:        string(tile.Type),
			OwnerID:     tile.OwnerID,
			Tags:        tile.Tags,
			Bonuses:     convertTileBonuses(tile.Bonuses),
			Location:    string(tile.Location),
			DisplayName: tile.DisplayName,
			ReservedBy:  tile.ReservedBy,
		}
		if tile.OccupiedBy != nil {
			tileDtos[i].OccupiedBy = &TileOccupantDto{
				DisplayName: tile.OccupiedBy.DisplayName,
				Type:        string(tile.OccupiedBy.Type),
				Tags:        tile.OccupiedBy.Tags,
				Visual:      toTileVisualDto(tile.OccupiedBy.Visual),
			}
		}
	}

	players := make(map[string]GameHistoryPlayerDto, len(s.Players))
	for id, p := range s.Players {
		// totalVP comes from the snapshot enricher in main.go, which calls
		// gameAction.ComputePlayerVPBreakdowns — the same helper FinalScoringAction
		// uses. If the enricher wasn't wired (legacy entries, tests), totalVP is 0.
		totalVP := 0
		if entry.VPBreakdowns != nil {
			if breakdown, ok := entry.VPBreakdowns[id]; ok {
				totalVP = breakdown.TotalVP
			}
		}

		dependencies := make(map[string]DependencyStatusDto, len(p.DependencyGroups))
		for key, group := range p.DependencyGroups {
			status := p.DependencyStatuses[key]
			dependencies[key] = historicalDependencyStatus(s.ModeRuntime, group, status)
		}
		if registry != nil {
			cardIDs := append(append([]string{}, p.HandCardIDs...), p.PlayedCardIDs...)
			if p.CorporationID != "" {
				cardIDs = append(cardIDs, p.CorporationID)
			}
			for _, cardID := range cardIDs {
				if card, err := registry.GetByID(cardID); err == nil {
					for _, group := range card.DependencyGroups {
						key := shared.DependencyKey(cardID, group.ID)
						status := p.DependencyStatuses[key]
						dependencies[key] = historicalDependencyStatus(s.ModeRuntime, group, status)
					}
				}
			}
		}
		players[id] = GameHistoryPlayerDto{
			PermanentProduction: toProductionDto(p.Production),
			DependencyStatuses:  dependencies,
			ID:                  p.ID,
			Name:                p.Name,
			Color:               p.Color,
			TerraformRating:     p.TerraformRating,
			Credits:             p.Resources.Credits,
			Steel:               p.Resources.Steel,
			Titanium:            p.Resources.Titanium,
			Plants:              p.Resources.Plants,
			Energy:              p.Resources.Energy,
			Heat:                p.Resources.Heat,
			PlayedCardCount:     len(p.PlayedCardIDs),
			Production:          toProductionDto(shared.EffectiveProduction(p.Production, p.ProductionContributions, p.DependencyStatuses, s.ModeRuntime)),
			PlayedCardIDs:       p.PlayedCardIDs,
			HandCardIDs:         p.HandCardIDs,
			CorporationID:       p.CorporationID,
			ResourceStorage:     p.ResourceStorage,
			TotalVP:             totalVP,
		}
	}

	milestones := make([]ClaimedMilestoneDto, len(s.ClaimedMilestones))
	for i, m := range s.ClaimedMilestones {
		milestones[i] = ClaimedMilestoneDto{
			Type:     string(m.Type),
			PlayerID: m.PlayerID,
		}
	}

	awards := make([]FundedAwardDto, len(s.FundedAwards))
	for i, a := range s.FundedAwards {
		awards[i] = FundedAwardDto{
			Type:     string(a.Type),
			PlayerID: a.FundedByPlayer,
		}
	}

	return GameHistoryEntryDto{
		ModeRuntime:  ModeRuntimeStateToDto(s.ModeRuntime),
		Sequence:     entry.Sequence,
		Timestamp:    entry.Timestamp,
		Generation:   s.Generation,
		Phase:        GamePhase(s.CurrentPhase),
		Temperature:  s.Temperature,
		Oxygen:       s.Oxygen,
		Oceans:       s.Oceans,
		Venus:        s.Venus,
		ActionNumber: s.GlobalActionCounter,
		Board:        BoardDto{Tiles: tileDtos},
		Players:      players,
		Milestones:   milestones,
		Awards:       awards,
		Settings: GameSettingsDto{
			AvailableGameModes: MapGameModes(cards.ModeDefinitions(registry)),
			MaxPlayers:         s.Settings.MaxPlayers,
			DevelopmentMode:    s.Settings.DevelopmentMode,
			DemoGame:           s.Settings.DemoGame,
			GameModeID:         s.Settings.GameModeID, GameModeParameters: s.Settings.GameModeParameters, Features: s.Settings.Features, Systems: s.Settings.Systems,
		},
	}
}

func historicalDependencyStatus(runtime shared.ModeRuntimeState, group shared.DependencyGroup, status shared.DependencyStatus) DependencyStatusDto {
	if !status.Removed && group.RetireOnState != nil {
		value, ok := runtime.Values[group.RetireOnState.State]
		status.Retired = status.Retired || (ok && value == group.RetireOnState.Equals)
	}
	return DependencyStatusDto{ID: group.ID, Label: group.Label, Active: shared.DependencyEnabled(runtime, group, status), Imported: status.Removed, Retired: status.Retired}
}
