package dto

import (
	"fmt"
	"time"

	"slices"

	"openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/award"
	"openmars/internal/game/board"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/colony"
	"openmars/internal/game/milestone"
	"openmars/internal/game/player"
	pfDomain "openmars/internal/game/projectfunding"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
)

// ToGameDto converts Game to GameDto with personalized view
// The playerID parameter determines which player is "currentPlayer" vs "otherPlayers"
// Registries bundles optional expansion registries for DTO mapping
type Registries struct {
	ColonyRegistry          colony.ColonyRegistry
	ProjectFundingRegistry  pfDomain.ProjectFundingRegistry
	StandardProjectRegistry standardproject.StandardProjectRegistry
	AwardRegistry           award.AwardRegistry
	MilestoneRegistry       milestone.MilestoneRegistry
	AvailableMaps           []MapInfoDto
}

func ToGameDto(g *game.Game, cardRegistry gamecards.CardRegistry, playerID string, colonyRegistry ...colony.ColonyRegistry) GameDto {
	return ToGameDtoFull(g, cardRegistry, playerID, Registries{
		ColonyRegistry: firstOrNil(colonyRegistry),
	})
}

func firstOrNil(regs []colony.ColonyRegistry) colony.ColonyRegistry {
	if len(regs) > 0 {
		return regs[0]
	}
	return nil
}

// ToGameDtoFull converts Game to GameDto as playerID sees it. A viewer who is not a player
// in the game sees every player's public data only.
func ToGameDtoFull(g *game.Game, cardRegistry gamecards.CardRegistry, playerID string, registries Registries) GameDto {
	if g.ResumeLobby() != nil {
		return ToResumeGameDto(g, playerID, cardRegistry)
	}
	cardRegistry = gamecards.ForGame(cardRegistry, g)
	players := g.GetAllPlayers()

	var currentPlayer PlayerDto
	otherPlayers := make([]OtherPlayerDto, 0)

	var viewingPlayer *player.Player
	for _, p := range players {
		if p.ID() == playerID {
			viewingPlayer = p
			currentPlayer = ToPlayerDto(p, g, cardRegistry, registries.StandardProjectRegistry, registries.AwardRegistry, registries.MilestoneRegistry)
		} else {
			otherPlayers = append(otherPlayers, ToOtherPlayerDto(p, g, cardRegistry))
		}
	}

	settings := g.Settings()
	settingsDto := GameSettingsDto{
		MaxPlayers:      settings.MaxPlayers,
		MapID:           settings.MapID,
		DevelopmentMode: settings.DevelopmentMode,
		DemoGame:        settings.DemoGame,
		AllowRandomBuy:  settings.AllowRandomBuy,
		GameModeID:      settings.GameModeID, GameModeParameters: settings.GameModeParameters, Features: settings.Features, Systems: settings.Systems, AvailableGameModes: MapGameModes(gamecards.ModeDefinitions(cardRegistry)),
		HasClaudeOAuthToken:   settings.ClaudeOAuthToken != "",
		BotSpendCapUSD:        settings.EffectiveBotSpendCapUSD(),
		BotSpendUSD:           g.BotSpendUSD(),
		AvailablePlayerColors: shared.PlayerColors,
		AvailableMaps:         registries.AvailableMaps,
		Temperature:           settings.Temperature,
		Oxygen:                settings.Oxygen,
		Oceans:                settings.Oceans,
		Generation:            settings.Generation,
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

	board := g.Board()
	tiles := board.Tiles()
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
			occupant := &TileOccupantDto{
				DisplayName: tile.OccupiedBy.DisplayName,
				Type:        string(tile.OccupiedBy.Type),
				Tags:        tile.OccupiedBy.Tags,
				Visual:      toTileVisualDto(tile.OccupiedBy.Visual),
			}
			tileDtos[i].OccupiedBy = occupant
		}
	}

	paymentConstants := PaymentConstantsDto{
		SteelValue:    2, // Default steel value
		TitaniumValue: 3, // Default titanium value
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

	triggeredEffects := g.GetTriggeredEffects()
	var triggeredEffectDtos []TriggeredEffectDto
	if len(triggeredEffects) > 0 {
		triggeredEffectDtos = make([]TriggeredEffectDto, len(triggeredEffects))
		for i, effect := range triggeredEffects {
			triggeredEffectDtos[i] = ToTriggeredEffectDto(effect)
		}
	}

	initPhaseDto := buildInitPhaseDto(g, players, cardRegistry)

	result := GameDto{
		ModeRuntime:      ToModeRuntimeDto(g),
		ID:               g.ID(),
		Status:           GameStatus(g.Status()),
		Settings:         settingsDto,
		HostPlayerID:     g.HostPlayerID(),
		CurrentPhase:     GamePhase(g.CurrentPhase()),
		GlobalParameters: globalParamsDto,
		CurrentPlayer:    currentPlayer,
		OtherPlayers:     otherPlayers,
		ViewingPlayerID:  playerID,
		CurrentTurn:      getCurrentTurnPlayerID(g),
		Generation:       g.Generation(),
		PlayerOrder:      g.PlayerOrder(),
		TurnOrder:        g.TurnOrder(),
		Board: BoardDto{
			Tiles: tileDtos,
		},
		PaymentConstants:   paymentConstants,
		Milestones:         ToMilestonesDto(g, cardRegistry, registries.MilestoneRegistry),
		Awards:             ToAwardsDto(g, cardRegistry, registries.AwardRegistry),
		AwardResults:       ToAwardResultsDto(g, cardRegistry, registries.AwardRegistry),
		FinalScores:        finalScoreDtos,
		TriggeredEffects:   triggeredEffectDtos,
		PlaceableTileTypes: ToPlaceableTileTypeDtos(),
		InitPhase:          initPhaseDto,
		Spectators:         toSpectatorDtos(g),
		ChatMessages:       toChatMessageDtos(g),
	}

	result.IsLastRound = g.GlobalParameters().IsMaxed()

	if g.HasColonies() && registries.ColonyRegistry != nil {
		result.Colonies = toColonyDtos(g, registries.ColonyRegistry, cardRegistry, playerID)
		if selection := result.CurrentPlayer.PendingColonySelection; selection != nil && selection.AddTile {
			selection.TileOptions = unusedColonyDtos(g, registries.ColonyRegistry, cardRegistry, playerID)
		}

		fleets := make(map[string]TradeFleetDto)
		for _, p := range players {
			fleet := g.Colonies().TradeFleet(p.ID())
			fleets[p.ID()] = TradeFleetDto{Total: fleet.Capacity, Available: fleet.Available()}
		}
		result.TradeFleets = fleets
	}

	if g.HasProjectFunding() && registries.ProjectFundingRegistry != nil {
		result.ProjectFunding = toProjectFundingDtos(g, registries.ProjectFundingRegistry, playerID)
	}

	if viewingPlayer == nil {
		result.CurrentPlayer.CardReceipts = nil
	}
	return result
}

func toSpectatorDtos(g *game.Game) []SpectatorDto {
	spectators := g.GetAllSpectators()
	dtos := make([]SpectatorDto, len(spectators))
	for i, s := range spectators {
		dtos[i] = SpectatorDto{
			ID:    s.ID(),
			Name:  s.Name(),
			Color: s.Color(),
		}
	}
	return dtos
}

// ToChatMessageDto maps a chat message to its DTO. Messages without a kind are plain chat.
func ToChatMessageDto(msg shared.ChatMessage) ChatMessageDto {
	kind := msg.Kind
	if kind == "" {
		kind = shared.ChatMessageKindChat
	}
	return ChatMessageDto{
		SenderID:    msg.SenderID,
		SenderName:  msg.SenderName,
		SenderColor: msg.SenderColor,
		Message:     msg.Message,
		Timestamp:   msg.Timestamp.Format(time.RFC3339),
		IsSpectator: msg.IsSpectator,
		Kind:        string(kind),
	}
}

func toChatMessageDtos(g *game.Game) []ChatMessageDto {
	messages := g.GetChatMessages()
	dtos := make([]ChatMessageDto, len(messages))
	for i, msg := range messages {
		dtos[i] = ToChatMessageDto(msg)
	}
	return dtos
}

// ToPlaceableTileTypeDtos converts the board PlaceableTileTypes registry to DTOs
func ToPlaceableTileTypeDtos() []PlaceableTileTypeDto {
	dtos := make([]PlaceableTileTypeDto, len(board.PlaceableTileTypes))
	for i, pt := range board.PlaceableTileTypes {
		dtos[i] = PlaceableTileTypeDto{
			Type:  pt.Type,
			Label: pt.Label,
			Group: pt.Group,
		}
	}
	return dtos
}

// getCurrentTurnPlayerID extracts the player ID from the current turn
func getCurrentTurnPlayerID(g *game.Game) *string {
	turn := g.CurrentTurn()
	if turn == nil {
		return nil
	}
	playerID := turn.PlayerID()
	return &playerID
}

// convertTileBonuses converts TileBonus to DTO
func convertTileBonuses(bonuses []board.TileBonus) []TileBonusDto {
	dtos := make([]TileBonusDto, len(bonuses))
	for i, bonus := range bonuses {
		dtos[i] = TileBonusDto{
			Type:   string(bonus.Type),
			Amount: bonus.Amount,
		}
	}
	return dtos
}

// ToMilestonesDto converts all milestones to DTOs including claim status and per-player progress
func ToMilestonesDto(g *game.Game, cardRegistry gamecards.CardRegistry, milestoneRegistry milestone.MilestoneRegistry) []MilestoneDto {
	if milestoneRegistry == nil {
		return nil
	}
	gameMilestones := g.Milestones()
	players := g.GetAllPlayers()
	b := g.Board()

	filteredDefs := action.FilterMilestones(milestoneRegistry.GetAll(), g.SelectedMilestones(), g.Settings())

	dtos := make([]MilestoneDto, len(filteredDefs))
	for i, def := range filteredDefs {
		milestoneType := shared.MilestoneType(def.ID)
		var claimedBy *string
		isClaimed := gameMilestones.IsClaimed(milestoneType)
		if isClaimed {
			for _, claimed := range gameMilestones.ClaimedMilestones() {
				if claimed.Type == milestoneType {
					claimedBy = &claimed.PlayerID
					break
				}
			}
		}

		playerProgress := make(map[string]int, len(players))
		for _, p := range players {
			playerProgress[p.ID()] = gamecards.CalculateMilestoneProgress(&def, p, b, cardRegistry)
		}

		rewardDtos := buildMilestoneRewardDtos(def.Reward)

		var styleDtoPtr *StyleDto
		if def.Style.Color != "" || def.Style.Icon != "" {
			styleDtoPtr = &StyleDto{Color: def.Style.Color, Icon: def.Style.Icon}
		}

		dtos[i] = MilestoneDto{
			Type:           def.ID,
			Name:           def.Name,
			Description:    def.Description,
			IsClaimed:      isClaimed,
			ClaimedBy:      claimedBy,
			ClaimCost:      def.ClaimCost,
			Required:       def.GetRequired(),
			PlayerProgress: playerProgress,
			Reward:         rewardDtos,
			Style:          styleDtoPtr,
		}
	}
	return dtos
}

// buildMilestoneRewardDtos converts milestone reward outputs to AwardRewardDto slice
func buildMilestoneRewardDtos(rewards []award.RewardOutput) []AwardRewardDto {
	if len(rewards) == 0 {
		return nil
	}
	outputs := make([]any, len(rewards))
	for i, r := range rewards {
		outputs[i] = BasicResourceConditionDto{
			Type:   string(r.Type),
			Amount: r.Amount,
			Target: "self-player",
		}
	}
	return []AwardRewardDto{
		{
			Place:   1,
			Outputs: outputs,
		},
	}
}

// ToAwardsDto converts all awards to DTOs including funding status and per-player scores
func ToAwardsDto(g *game.Game, cardRegistry gamecards.CardRegistry, awardRegistry award.AwardRegistry) []AwardDto {
	if awardRegistry == nil {
		return nil
	}
	gameAwards := g.Awards()
	players := g.GetAllPlayers()
	b := g.Board()

	filteredDefs := action.FilterAwards(awardRegistry.GetAll(), g.SelectedAwards(), g.Settings())

	dtos := make([]AwardDto, len(filteredDefs))
	fundedCount := gameAwards.FundedCount()

	for i, def := range filteredDefs {
		awardType := shared.AwardType(def.ID)
		var fundedBy *string
		isFunded := gameAwards.IsFunded(awardType)
		fundingCost := def.GetCostForFundedCount(0)

		if isFunded {
			for _, funded := range gameAwards.FundedAwards() {
				if funded.Type == awardType {
					fundedBy = &funded.FundedByPlayer
					fundingCost = funded.FundingCost
					break
				}
			}
		} else {
			if fundedCount < game.MaxFundedAwards {
				fundingCost = def.GetCostForFundedCount(fundedCount)
			}
		}

		playerProgress := make(map[string]int, len(players))
		for _, p := range players {
			playerProgress[p.ID()] = gamecards.CalculateAwardScore(&def, p, b, cardRegistry)
		}

		rewardDtos := make([]AwardRewardDto, len(def.Rewards))
		for ri, r := range def.Rewards {
			outputs := make([]any, len(r.Outputs))
			for oi, o := range r.Outputs {
				outputs[oi] = BasicResourceConditionDto{
					Type:   string(o.Type),
					Amount: o.Amount,
					Target: "self-player",
				}
			}
			rewardDtos[ri] = AwardRewardDto{
				Place:   r.Place,
				Outputs: outputs,
			}
		}

		var styleDtoPtr *StyleDto
		if def.Style.Color != "" || def.Style.Icon != "" {
			styleDtoPtr = &StyleDto{Color: def.Style.Color, Icon: def.Style.Icon}
		}

		dtos[i] = AwardDto{
			Type:           def.ID,
			Name:           def.Name,
			Description:    def.Description,
			IsFunded:       isFunded,
			FundedBy:       fundedBy,
			FundingCost:    fundingCost,
			PlayerProgress: playerProgress,
			Rewards:        rewardDtos,
			Style:          styleDtoPtr,
		}
	}
	return dtos
}

// ToAwardResultsDto converts funded awards to placement results
func ToAwardResultsDto(g *game.Game, cardRegistry gamecards.CardRegistry, awardRegistry award.AwardRegistry) []AwardResultDto {
	if awardRegistry == nil {
		return nil
	}
	fundedAwards := g.Awards().FundedAwards()
	results := make([]AwardResultDto, 0, len(fundedAwards))

	for _, funded := range fundedAwards {
		def, err := awardRegistry.GetByID(string(funded.Type))
		if err != nil {
			continue
		}
		placements := gamecards.ScoreAward(def, g.GetAllPlayers(), g.Board(), cardRegistry)

		firstPlace := make([]string, 0)
		secondPlace := make([]string, 0)
		for _, p := range placements {
			if p.Placement == 1 {
				firstPlace = append(firstPlace, p.PlayerID)
			} else if p.Placement == 2 {
				secondPlace = append(secondPlace, p.PlayerID)
			}
		}

		results = append(results, AwardResultDto{
			AwardType:      string(funded.Type),
			FirstPlaceIds:  firstPlace,
			SecondPlaceIds: secondPlace,
		})
	}
	return results
}

// ToCardVPConditionDetailDto converts a card VP condition detail to DTO
func ToCardVPConditionDetailDto(detail shared.CardVPConditionDetail) CardVPConditionDetailDto {
	return CardVPConditionDetailDto{
		ConditionType:  detail.ConditionType,
		Amount:         detail.Amount,
		Count:          detail.Count,
		MaxTrigger:     detail.MaxTrigger,
		ActualTriggers: detail.ActualTriggers,
		TotalVP:        detail.TotalVP,
		Explanation:    detail.Explanation,
	}
}

// ToCardVPDetailDto converts a card VP detail to DTO
func ToCardVPDetailDto(detail shared.CardVPDetail) CardVPDetailDto {
	return CardVPDetailDto{
		CardID:     detail.CardID,
		CardName:   detail.CardName,
		Conditions: mapSlice(detail.Conditions, ToCardVPConditionDetailDto),
		TotalVP:    detail.TotalVP,
	}
}

// ToGreeneryVPDetailDto converts a greenery VP detail to DTO
func ToGreeneryVPDetailDto(detail shared.GreeneryVPDetail) GreeneryVPDetailDto {
	return GreeneryVPDetailDto{
		Coordinate: detail.Coordinate,
		VP:         detail.VP,
	}
}

// ToCityVPDetailDto converts a city VP detail to DTO
func ToCityVPDetailDto(detail shared.CityVPDetail) CityVPDetailDto {
	return CityVPDetailDto{
		CityCoordinate:     detail.CityCoordinate,
		AdjacentGreeneries: detail.AdjacentGreeneries,
		VP:                 detail.VP,
	}
}

// ToVPBreakdownDto converts a VP breakdown to DTO
func ToVPBreakdownDto(breakdown shared.VPBreakdown) VPBreakdownDto {
	return VPBreakdownDto{
		TerraformRating:   breakdown.TerraformRating,
		CardVP:            breakdown.CardVP,
		CardVPDetails:     mapSlice(breakdown.CardVPDetails, ToCardVPDetailDto),
		MilestoneVP:       breakdown.MilestoneVP,
		AwardVP:           breakdown.AwardVP,
		GreeneryVP:        breakdown.GreeneryVP,
		GreeneryVPDetails: mapSlice(breakdown.GreeneryVPDetails, ToGreeneryVPDetailDto),
		CityVP:            breakdown.CityVP,
		CityVPDetails:     mapSlice(breakdown.CityVPDetails, ToCityVPDetailDto),
		TotalVP:           breakdown.TotalVP,
	}
}

// ToFinalScoreDto creates a final score DTO for a player
func ToFinalScoreDto(playerID, playerName string, breakdown shared.VPBreakdown, isWinner bool, placement int) FinalScoreDto {
	return FinalScoreDto{
		PlayerID:    playerID,
		PlayerName:  playerName,
		VPBreakdown: ToVPBreakdownDto(breakdown),
		IsWinner:    isWinner,
		Placement:   placement,
	}
}

// toVPGranterDtos converts a slice of VPGranter to VPGranterDto slice with per-condition breakdown
func toVPGranterDtos(granters []shared.VPGranter) []VPGranterDto {
	if len(granters) == 0 {
		return []VPGranterDto{}
	}

	dtos := make([]VPGranterDto, len(granters))
	for i, g := range granters {
		conditions := make([]VPGranterConditionDto, len(g.VPConditions))
		for j, cond := range g.VPConditions {
			conditions[j] = toVPGranterConditionDto(cond)
		}
		dtos[i] = VPGranterDto{
			CardID:        g.CardID,
			CardName:      g.CardName,
			Description:   g.Description,
			ComputedValue: g.ComputedValue,
			Conditions:    conditions,
		}
	}
	return dtos
}

func toVPGranterConditionDto(cond shared.VPCondition) VPGranterConditionDto {
	dto := VPGranterConditionDto{
		Amount:        cond.Amount,
		ConditionType: string(cond.Condition),
	}

	switch cond.Condition {
	case shared.VPConditionFixed, shared.VPConditionOnce:
		dto.ComputedVP = cond.Amount
		dto.Explanation = fmt.Sprintf("%d VP", cond.Amount)

	case shared.VPConditionPer:
		if cond.Per != nil {
			perType := string(cond.Per.ResourceType)
			if cond.Per.Tag != nil {
				perType = string(*cond.Per.Tag)
			}
			if cond.Per.Target != nil && *cond.Per.Target == "self-card" {
				perType = string(cond.Per.ResourceType)
			}
			dto.PerType = &perType
			dto.PerAmount = &cond.Per.Amount
			dto.AdjacentToSelfTile = cond.Per.AdjacentToSelfTile
			dto.Explanation = fmt.Sprintf("%d VP per %d %s", cond.Amount, cond.Per.Amount, perType)
		}
	}

	return dto
}

// ToTriggeredEffectDto converts a triggered effect to DTO
func ToTriggeredEffectDto(effect shared.TriggeredEffect) TriggeredEffectDto {
	outputDtos := make([]any, len(effect.Outputs))
	for i, output := range effect.Outputs {
		outputDtos[i] = toResourceConditionDto(output)
	}

	var calculatedOutputDtos []CalculatedOutputDto
	if len(effect.CalculatedOutputs) > 0 {
		calculatedOutputDtos = make([]CalculatedOutputDto, len(effect.CalculatedOutputs))
		for i, co := range effect.CalculatedOutputs {
			calculatedOutputDtos[i] = CalculatedOutputDto{
				ResourceType: co.ResourceType,
				Amount:       co.Amount,
				IsScaled:     co.IsScaled,
			}
		}
	}

	var behaviorDtos []CardBehaviorDto
	if len(effect.Behaviors) > 0 {
		behaviorDtos = make([]CardBehaviorDto, len(effect.Behaviors))
		for i, b := range effect.Behaviors {
			behaviorDtos[i] = toCardBehaviorDto(b)
		}
	}

	var vpConditionDtos []VPConditionDto
	if len(effect.VPConditions) > 0 {
		vpConditionDtos = make([]VPConditionDto, len(effect.VPConditions))
		for i, vp := range effect.VPConditions {
			vpConditionDtos[i] = VPConditionDto{
				Amount:     vp.Amount,
				Condition:  VPConditionType(vp.Condition),
				MaxTrigger: vp.MaxTrigger,
				Per:        ptrCast(vp.Per, toPerConditionDto),
			}
		}
	}

	return TriggeredEffectDto{
		CardName:          effect.CardName,
		PlayerID:          effect.PlayerID,
		SourceType:        string(effect.SourceType),
		Outputs:           outputDtos,
		CalculatedOutputs: calculatedOutputDtos,
		Behaviors:         behaviorDtos,
		VPConditions:      vpConditionDtos,
	}
}

func buildGlobalParameterBonuses(venusEnabled bool) []GlobalParameterBonusDto {
	bonuses := []GlobalParameterBonusDto{
		{Parameter: "temperature", Threshold: -24, RewardType: "heat-production", RewardAmount: 1},
		{Parameter: "temperature", Threshold: -20, RewardType: "heat-production", RewardAmount: 1},
		{Parameter: "temperature", Threshold: 0, RewardType: "ocean-placement", RewardAmount: 1},
		{Parameter: "oxygen", Threshold: 8, RewardType: "temperature", RewardAmount: 1},
	}
	if venusEnabled {
		bonuses = append(bonuses,
			GlobalParameterBonusDto{Parameter: "venus", Threshold: 8, RewardType: "card-draw", RewardAmount: 1},
			GlobalParameterBonusDto{Parameter: "venus", Threshold: 16, RewardType: "tr", RewardAmount: 1},
		)
	}
	return bonuses
}

func toColonyDtos(g *game.Game, colonyRegistry colony.ColonyRegistry, cardRegistry gamecards.CardRegistry, playerID string) []ColonyDto {
	return mapColonyStates(g, colonyRegistry, cardRegistry, playerID, g.Colonies().States())
}

func unusedColonyDtos(g *game.Game, colonyRegistry colony.ColonyRegistry, cardRegistry gamecards.CardRegistry, playerID string) []ColonyDto {
	var states []*colony.ColonyState
	for _, def := range g.Colonies().UnusedDefinitions() {
		states = append(states, gamecards.InitializeColonyTile(g, def, cardRegistry))
	}
	return mapColonyStates(g, colonyRegistry, cardRegistry, playerID, states)
}

func mapColonyStates(g *game.Game, colonyRegistry colony.ColonyRegistry, cardRegistry gamecards.CardRegistry, playerID string, tileStates []*colony.ColonyState) []ColonyDto {
	if len(tileStates) == 0 {
		return nil
	}

	playerObj, _ := g.GetPlayer(playerID)

	dtos := make([]ColonyDto, 0, len(tileStates))
	for _, state := range tileStates {
		def, err := colonyRegistry.GetByID(state.DefinitionID)
		if err != nil {
			continue
		}

		steps := make([]ColonyStepDto, len(def.Steps))
		for i, s := range def.Steps {
			outputs := make([]ColonyOutputDto, len(s.Outputs))
			for j, o := range s.Outputs {
				outputs[j] = ColonyOutputDto{Type: o.Type, Amount: o.Amount}
			}
			steps[i] = ColonyStepDto{Outputs: outputs}
		}

		colonyBonus := make([]ColonyOutputDto, len(def.ColonyBonus))
		for i, b := range def.ColonyBonus {
			colonyBonus[i] = ColonyOutputDto{Type: b.Type, Amount: b.Amount}
		}

		colonySlots := make([]ColonySlotDto, len(def.Colonies))
		for i, c := range def.Colonies {
			reward := make([]ColonyOutputDto, len(c.Reward))
			for j, r := range c.Reward {
				reward[j] = ColonyOutputDto{Type: r.Type, Amount: r.Amount}
			}
			colonySlots[i] = ColonySlotDto{Reward: reward}
		}

		playerColonies := state.PlayerColonies
		if playerColonies == nil {
			playerColonies = []string{}
		}

		// Calculate trade availability
		tradeAvailable := true
		var tradeErrors []StateErrorDto
		if state.TradedThisGen {
			tradeAvailable = false
			tradeErrors = append(tradeErrors, StateErrorDto{
				Code:    StateErrorCode("colony-already-traded"),
				Message: "This colony has already been traded this generation",
			})
		}
		if g.Colonies().TradeFleet(playerID).Available() == 0 {
			tradeAvailable = false
			tradeErrors = append(tradeErrors, StateErrorDto{
				Code:    StateErrorCode("fleet-unavailable"),
				Message: "Your trade fleet is not available",
			})
		}
		if playerObj != nil {
			costs, _ := action.CalculateEffectiveTradeCosts(playerObj, cardRegistry)
			canAffordAny := false
			for rt, cost := range costs {
				if gamecards.PaymentCapacity(playerObj, g, cardRegistry, shared.ActionColonyTrade, shared.ResourceType(rt)) >= cost {
					canAffordAny = true
				}
			}
			if !canAffordAny {
				tradeAvailable = false
				tradeErrors = append(tradeErrors, StateErrorDto{
					Code:    StateErrorCode("insufficient-resources"),
					Message: "Cannot afford any trade payment type",
				})
			}
		}

		// Calculate build availability
		buildAvailable := true
		var buildErrors []StateErrorDto
		maxColonies := len(def.Colonies)
		if len(state.PlayerColonies) >= maxColonies {
			buildAvailable = false
			buildErrors = append(buildErrors, StateErrorDto{
				Code:    StateErrorCode("colony-full"),
				Message: "This colony tile is full",
			})
		}
		if slices.Contains(state.PlayerColonies, playerID) {
			buildAvailable = false
			buildErrors = append(buildErrors, StateErrorDto{
				Code:    StateErrorCode("already-has-colony"),
				Message: "You already have a colony here",
			})
		}
		if playerObj != nil {
			capacity := gamecards.PaymentCapacity(playerObj, g, cardRegistry, "build-colony", shared.ResourceCredit)
			if capacity < 17 {
				buildAvailable = false
				buildErrors = append(buildErrors, StateErrorDto{
					Code:    StateErrorCode("insufficient-credits"),
					Message: fmt.Sprintf("Insufficient payment: need 17, have %d", capacity),
				})
			}
		}

		tradeOptions := make([]ColonyTradeOptionDto, 0)
		for _, option := range action.CalculateColonyTradeOptions(playerObj, state, def, cardRegistry) {
			outputs := make([]ColonyOutputDto, 0, len(option.Outputs))
			for _, output := range option.Outputs {
				outputs = append(outputs, ColonyOutputDto{Type: output.Type, Amount: output.Amount})
			}
			tradeOptions = append(tradeOptions, ColonyTradeOptionDto{TrackSteps: option.TrackSteps, MarkerPosition: option.MarkerPosition, Outputs: outputs})
		}
		if state.AwaitingResource != "" {
			tradeAvailable = false
			buildAvailable = false
			reason := StateErrorDto{Code: StateErrorCode("colony-inactive"), Message: "Requires a card that can hold " + state.AwaitingResource + " resources in play"}
			tradeErrors = append(tradeErrors, reason)
			buildErrors = append(buildErrors, reason)
		}
		dtos = append(dtos, ColonyDto{
			Active:         state.AwaitingResource == "",
			ID:             def.ID,
			Name:           def.Name,
			Location:       def.Location,
			Steps:          steps,
			ColonyBonus:    colonyBonus,
			Colonies:       colonySlots,
			MarkerPosition: state.MarkerPosition,
			PlayerColonies: playerColonies,
			TradedThisGen:  state.TradedThisGen,
			TraderID:       state.TraderID,
			Style: StyleDto{
				Color: def.Style.Color,
				Icon:  def.Style.Icon,
			},
			TradeOptions:   tradeOptions,
			TradeAvailable: tradeAvailable,
			BuildAvailable: buildAvailable,
			TradeErrors:    tradeErrors,
			BuildErrors:    buildErrors,
		})
	}

	return dtos
}

func toProjectFundingDtos(g *game.Game, registry pfDomain.ProjectFundingRegistry, playerID string) []ProjectFundingDto {
	states := g.ProjectFundingStates()
	if len(states) == 0 {
		return nil
	}

	playerObj, _ := g.GetPlayer(playerID)
	var playerCredits int
	if playerObj != nil {
		playerCredits = playerObj.Resources().Get().Credits
	}

	playerCount := len(g.GetAllPlayers())

	dtos := make([]ProjectFundingDto, 0, len(states))
	for _, state := range states {
		def, err := registry.GetByID(state.DefinitionID)
		if err != nil {
			continue
		}

		scaledSeats := pfDomain.ScaledSeats(def.Seats, playerCount)
		scaledTiers := pfDomain.ScaledRewardTiers(def.RewardTiers, playerCount)

		// Build seat DTOs
		seats := make([]ProjectSeatDto, len(scaledSeats))
		for i, seatDef := range scaledSeats {
			subs := make([]ProjectPaymentSubDto, len(seatDef.PaymentSubstitutes))
			for j, sub := range seatDef.PaymentSubstitutes {
				subs[j] = ProjectPaymentSubDto{
					ResourceType:   sub.ResourceType,
					ConversionRate: sub.ConversionRate,
				}
			}

			seat := ProjectSeatDto{
				Cost:               seatDef.Cost,
				PaymentSubstitutes: subs,
			}

			if i < len(state.SeatOwners) {
				ownerID := state.SeatOwners[i]
				seat.IsFilled = true
				seat.OwnerID = ownerID
				if p, err := g.GetPlayer(ownerID); err == nil {
					seat.OwnerName = p.Name()
					seat.OwnerColor = p.Color()
				}
			}

			seats[i] = seat
		}

		// Build seat owners list
		seatOwners := make([]ProjectSeatOwnerDto, len(state.SeatOwners))
		for i, ownerID := range state.SeatOwners {
			owner := ProjectSeatOwnerDto{PlayerID: ownerID}
			if p, err := g.GetPlayer(ownerID); err == nil {
				owner.Name = p.Name()
				owner.Color = p.Color()
			}
			seatOwners[i] = owner
		}

		// Next seat info
		nextSeatIndex := len(state.SeatOwners)
		nextSeatCost := 0
		var nextSeatSubs []ProjectPaymentSubDto
		if nextSeatIndex < len(scaledSeats) {
			nextSeatCost = scaledSeats[nextSeatIndex].Cost
			for _, sub := range scaledSeats[nextSeatIndex].PaymentSubstitutes {
				nextSeatSubs = append(nextSeatSubs, ProjectPaymentSubDto{
					ResourceType:   sub.ResourceType,
					ConversionRate: sub.ConversionRate,
				})
			}
		}
		if nextSeatSubs == nil {
			nextSeatSubs = []ProjectPaymentSubDto{}
		}

		// Buy availability
		canBuy := true
		var buyErrors []StateErrorDto
		if state.IsCompleted {
			canBuy = false
			buyErrors = append(buyErrors, StateErrorDto{
				Code:    StateErrorCode("project-completed"),
				Message: "This project is already completed",
			})
		} else if nextSeatIndex >= len(scaledSeats) {
			canBuy = false
			buyErrors = append(buyErrors, StateErrorDto{
				Code:    StateErrorCode("all-seats-filled"),
				Message: "All seats are filled",
			})
		} else if playerCredits < nextSeatCost {
			canBuy = false
			buyErrors = append(buyErrors, StateErrorDto{
				Code:    StateErrorCode("insufficient-credits"),
				Message: fmt.Sprintf("Insufficient credits: need %d, have %d", nextSeatCost, playerCredits),
			})
		}

		// Count current player seats and find tier
		currentPlayerSeats := 0
		for _, ownerID := range state.SeatOwners {
			if ownerID == playerID {
				currentPlayerSeats++
			}
		}

		var currentPlayerTier *ProjectRewardTierDto
		if currentPlayerSeats > 0 {
			tier := pfDomain.FindBestTier(scaledTiers, currentPlayerSeats)
			if tier != nil {
				rewards := make([]ColonyOutputDto, len(tier.Rewards))
				for j, r := range tier.Rewards {
					rewards[j] = ColonyOutputDto{Type: r.Type, Amount: r.Amount}
				}
				currentPlayerTier = &ProjectRewardTierDto{
					SeatsOwned: tier.SeatsOwned,
					Rewards:    rewards,
				}
			}
		}

		// Reward tiers
		rewardTiers := make([]ProjectRewardTierDto, len(scaledTiers))
		for i, tier := range scaledTiers {
			rewards := make([]ColonyOutputDto, len(tier.Rewards))
			for j, r := range tier.Rewards {
				rewards[j] = ColonyOutputDto{Type: r.Type, Amount: r.Amount}
			}
			rewardTiers[i] = ProjectRewardTierDto{
				SeatsOwned: tier.SeatsOwned,
				Rewards:    rewards,
			}
		}

		// Completion effect
		completionRewards := make([]ColonyOutputDto, len(def.CompletionEffect.Rewards))
		for j, r := range def.CompletionEffect.Rewards {
			completionRewards[j] = ColonyOutputDto{Type: r.Type, Amount: r.Amount}
		}
		var completionGlobalEffects []ProjectGlobalOutputDto
		for _, ge := range def.CompletionEffect.GlobalEffects {
			completionGlobalEffects = append(completionGlobalEffects, ProjectGlobalOutputDto{
				Type:   ge.Type,
				Amount: ge.Amount,
			})
		}

		dtos = append(dtos, ProjectFundingDto{
			ID:                 def.ID,
			Name:               def.Name,
			Description:        def.Description,
			Seats:              seats,
			SeatOwners:         seatOwners,
			IsCompleted:        state.IsCompleted,
			NextSeatIndex:      nextSeatIndex,
			NextSeatCost:       nextSeatCost,
			CanBuySeat:         canBuy,
			BuyErrors:          buyErrors,
			CurrentPlayerSeats: currentPlayerSeats,
			CurrentPlayerTier:  currentPlayerTier,
			PaymentSubstitutes: nextSeatSubs,
			RewardTiers:        rewardTiers,
			CompletionEffect: ProjectCompletionEffectDto{
				Description:   def.CompletionEffect.Description,
				Rewards:       completionRewards,
				GlobalEffects: completionGlobalEffects,
			},
			Style: StyleDto{
				Color: def.Style.Color,
				Icon:  def.Style.Icon,
			},
		})
	}

	return dtos
}

// ToResumeGameDto exposes only the information needed to gather players.
func ToResumeGameDto(g *game.Game, playerID string, cardRegistry gamecards.CardRegistry) GameDto {
	cardRegistry = gamecards.ForGame(cardRegistry, g)
	settings := g.Settings()
	lobby := g.ResumeLobby()
	seats := make([]ResumeSeatDto, 0, len(g.GetAllPlayers()))
	for _, id := range g.PlayerOrder() {
		p, err := g.GetPlayer(id)
		if err != nil {
			continue
		}
		corpName := ""
		if card, err := cardRegistry.GetByID(p.CorporationID()); err == nil {
			corpName = card.Name
		}
		botError := ""
		if p.BotError() != "" {
			botError = "The bot could not start. Check its credentials and try again."
		}
		seats = append(seats, ResumeSeatDto{CorporationName: corpName, ID: id, Name: p.Name(), SavedName: lobby.SavedNames[id], CorporationID: p.CorporationID(), Color: p.Color(), PlayerType: string(p.PlayerType()), Claimed: lobby.Claimed[id], Connected: p.IsConnected(), Exited: p.HasExited(), BotStatus: string(p.BotStatus()), BotError: botError})
	}
	return GameDto{ID: g.ID(), Status: GameStatus(g.Status()), CurrentPhase: GamePhase(g.CurrentPhase()), Generation: g.Generation(), HostPlayerID: g.HostPlayerID(), ViewingPlayerID: playerID, Settings: GameSettingsDto{MapID: settings.MapID, GameModeID: settings.GameModeID, GameModeParameters: settings.GameModeParameters, Features: settings.Features, Systems: settings.Systems, AvailableGameModes: MapGameModes(gamecards.ModeDefinitions(cardRegistry)), MaxPlayers: settings.MaxPlayers, HasClaudeOAuthToken: settings.ClaudeOAuthToken != ""}, ResumeLobby: &ResumeLobbyDto{Seats: seats}, OtherPlayers: []OtherPlayerDto{}, ChatMessages: toChatMessageDtos(g)}
}
