package dto

import (
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/board"
	"openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	"openmars/internal/game/shared"
)

// ToSpectatorGameDto creates a GameDto for spectators where all players are shown
// as OtherPlayerDto (no hidden information like hand cards or pending selections).
func ToSpectatorGameDto(g *game.Game, cardRegistry cards.CardRegistry, awardRegistry award.AwardRegistry, milestoneRegistry milestone.MilestoneRegistry) GameDto {
	if g.ResumeLobby() != nil {
		return ToResumeGameDto(g, "", cardRegistry)
	}
	cardRegistry = cards.ForGame(cardRegistry, g)
	players := g.GetAllPlayers()

	otherPlayers := make([]OtherPlayerDto, 0, len(players))
	for _, p := range players {
		otherPlayers = append(otherPlayers, ToOtherPlayerDto(p, g, cardRegistry))
	}

	settings := g.Settings()
	settingsDto := GameSettingsDto{
		MaxPlayers:      settings.MaxPlayers,
		DevelopmentMode: settings.DevelopmentMode,
		DemoGame:        settings.DemoGame,
		AllowRandomBuy:  settings.AllowRandomBuy,
		GameModeID:      settings.GameModeID, GameModeParameters: settings.GameModeParameters, Features: settings.Features, Systems: settings.Systems, AvailableGameModes: MapGameModes(cards.ModeDefinitions(cardRegistry)),
		HasClaudeOAuthToken:   settings.ClaudeOAuthToken != "",
		BotSpendCapUSD:        settings.EffectiveBotSpendCapUSD(),
		BotSpendUSD:           g.BotSpendUSD(),
		AvailablePlayerColors: shared.PlayerColors,
	}

	globalParams := g.GlobalParameters()
	globalParamsDto := GlobalParametersDto{
		Temperature: globalParams.Temperature(),
		Oxygen:      globalParams.Oxygen(),
		Oceans:      globalParams.Oceans(),
		MaxOceans:   globalParams.GetMaxOceans(),
		Venus:       globalParams.Venus(),
		Bonuses:     buildGlobalParameterBonuses(settings.HasVenus()),
	}

	tiles := g.Board().Tiles()
	tileDtos := make([]TileDto, len(tiles))
	for i, tile := range tiles {
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

	paymentConstants := PaymentConstantsDto{
		SteelValue:    2,
		TitaniumValue: 3,
	}

	var finalScoreDtos []FinalScoreDto
	if g.Status() == shared.GameStatusCompleted {
		finalScores := g.GetFinalScores()
		if finalScores != nil {
			finalScoreDtos = make([]FinalScoreDto, len(finalScores))
			for i, fs := range finalScores {
				finalScoreDtos[i] = FinalScoreDto{
					PlayerID:    fs.PlayerID,
					PlayerName:  fs.PlayerName,
					VPBreakdown: ToVPBreakdownDto(fs.Breakdown),
					IsWinner:    fs.IsWinner,
					Placement:   fs.Placement,
				}
			}
		}
	}

	initPhaseDto := buildInitPhaseDto(g, players, cardRegistry)

	triggeredEffects := g.GetTriggeredEffects()
	var triggeredEffectDtos []TriggeredEffectDto
	if len(triggeredEffects) > 0 {
		triggeredEffectDtos = make([]TriggeredEffectDto, len(triggeredEffects))
		for i, effect := range triggeredEffects {
			triggeredEffectDtos[i] = ToTriggeredEffectDto(effect)
		}
	}

	return GameDto{
		ModeRuntime:      ToModeRuntimeDto(g),
		ID:               g.ID(),
		Status:           GameStatus(g.Status()),
		Settings:         settingsDto,
		HostPlayerID:     g.HostPlayerID(),
		CurrentPhase:     GamePhase(g.CurrentPhase()),
		GlobalParameters: globalParamsDto,
		CurrentPlayer:    PlayerDto{},
		OtherPlayers:     otherPlayers,
		ViewingPlayerID:  "",
		IsSpectator:      true,
		CurrentTurn:      getCurrentTurnPlayerID(g),
		Generation:       g.Generation(),
		TurnOrder:        g.TurnOrder(),
		Board: BoardDto{
			Tiles: tileDtos,
		},
		PaymentConstants:   paymentConstants,
		Milestones:         ToMilestonesDto(g, cardRegistry, milestoneRegistry),
		Awards:             ToAwardsDto(g, cardRegistry, awardRegistry),
		AwardResults:       ToAwardResultsDto(g, cardRegistry, awardRegistry),
		FinalScores:        finalScoreDtos,
		TriggeredEffects:   triggeredEffectDtos,
		PlaceableTileTypes: ToPlaceableTileTypeDtos(),
		InitPhase:          initPhaseDto,
		Spectators:         toSpectatorDtos(g),
		ChatMessages:       toChatMessageDtos(g),
	}
}

func convertTileBonusesForSpectator(bonuses []board.TileBonus) []TileBonusDto {
	return convertTileBonuses(bonuses)
}
