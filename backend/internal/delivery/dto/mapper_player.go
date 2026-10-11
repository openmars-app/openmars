package dto

import (
	"openmars/internal/action"
	"openmars/internal/game"
	"openmars/internal/game/award"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/milestone"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/game/standardproject"
)

// toResourcesDto converts shared.Resources to ResourcesDto.
func toResourcesDto(res shared.Resources) ResourcesDto {
	return ResourcesDto{
		Credits:  res.Credits,
		Steel:    res.Steel,
		Titanium: res.Titanium,
		Plants:   res.Plants,
		Energy:   res.Energy,
		Heat:     res.Heat,
	}
}

// toProductionDto converts shared.Production to ProductionDto.
func toProductionDto(prod shared.Production) ProductionDto {
	return ProductionDto{
		Credits:  prod.Credits,
		Steel:    prod.Steel,
		Titanium: prod.Titanium,
		Plants:   prod.Plants,
		Energy:   prod.Energy,
		Heat:     prod.Heat,
	}
}

// calculateResourceDelta calculates the difference between two Resources and returns a ResourcesDto.
func calculateResourceDelta(before, after shared.Resources) ResourcesDto {
	return ResourcesDto{
		Credits:  after.Credits - before.Credits,
		Steel:    after.Steel - before.Steel,
		Titanium: after.Titanium - before.Titanium,
		Plants:   after.Plants - before.Plants,
		Energy:   after.Energy - before.Energy,
		Heat:     after.Heat - before.Heat,
	}
}

// ToPlayerDto converts Player to PlayerDto
func ToPlayerDto(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry, stdProjRegistry standardproject.StandardProjectRegistry, awardRegistry award.AwardRegistry, milestoneRegistry milestone.MilestoneRegistry) PlayerDto {
	cardRegistry = gamecards.ForGame(cardRegistry, g)
	resourcesComponent := p.Resources()
	resources := resourcesComponent.Get()
	production := resourcesComponent.Production()

	corporation := getCorporationCard(p, cardRegistry)
	playedCardIDs := p.PlayedCards().Cards()
	playedCards := getPlayedCards(playedCardIDs, cardRegistry)
	for i := range playedCards {
		if c, err := cardRegistry.GetByID(playedCards[i].ID); err == nil {
			AnnotateCardDependencies(&playedCards[i], *c, p)
		}
	}
	handCards := mapPlayerCards(p, g, cardRegistry)
	standardProjects := mapPlayerStandardProjects(p, g, cardRegistry, stdProjRegistry)
	milestones := mapPlayerMilestones(p, g, cardRegistry, milestoneRegistry)
	awards := mapPlayerAwards(p, g, awardRegistry, cardRegistry)

	// Only the WebSocket broadcast path supplies every action registry. When any is
	// nil (the HTTP player_handler.go path), leave the pointer nil so the field is
	// omitted: an honest "unknown", never a fabricated false.
	var hasAvailableActions *bool
	if stdProjRegistry != nil && awardRegistry != nil && milestoneRegistry != nil {
		has := action.HasAvailableActions(g, p, cardRegistry, stdProjRegistry, milestoneRegistry, awardRegistry)
		hasAvailableActions = &has
	}

	var pendingTileSelection *PendingTileSelectionDto
	var forcedFirstAction *ForcedFirstActionDto
	currentTurn := g.CurrentTurn()
	if currentTurn != nil && currentTurn.PlayerID() == p.ID() {
		pendingTileSelection = convertPendingTileSelection(g.GetPendingTileSelection(p.ID()))
		forcedFirstAction = convertForcedFirstAction(g.GetForcedFirstAction(p.ID()))
	}
	if g.CurrentPhase() == shared.GamePhaseStartingSelection ||
		g.CurrentPhase() == shared.GamePhaseInitApplyCorp ||
		g.CurrentPhase() == shared.GamePhaseInitApplyPrelude {
		pendingTileSelection = convertPendingTileSelection(g.GetPendingTileSelection(p.ID()))
		forcedFirstAction = convertForcedFirstAction(g.GetForcedFirstAction(p.ID()))
	}

	removalTargets := make([]ResourceRemovalTargetDto, 0)
	for _, target := range gamecards.AvailableResourceRemovalTargets(g, p, cardRegistry) {
		removalTargets = append(removalTargets, ResourceRemovalTargetDto{PlayerID: target.PlayerID, CardID: target.CardID, ResourceType: ResourceType(target.ResourceType), Amount: target.Amount})
	}
	return PlayerDto{
		ModeProduction:         modeProductionPreview(p, g),
		PermanentProduction:    toProductionDto(p.Resources().PermanentProduction()),
		CardReceipts:           convertCardReceipts(p.Selection().CardReceipts(), cardRegistry),
		ResourceRemovalTargets: removalTargets,
		PendingCardReveal:      convertPendingCardReveal(p.Selection().GetPendingCardReveal(), cardRegistry),
		PendingEffectSelection: convertPendingEffectSelection(p.Selection().GetPendingEffectSelection()),
		ID:                     p.ID(),
		Name:                   p.Name(),
		PlayerType:             string(p.PlayerType()),
		BotStatus:              string(p.BotStatus()),
		BotPersona:             p.BotPersona(),
		BotError:               p.BotError(),
		Color:                  p.Color(),
		Resources:              toResourcesDto(resources),
		Production:             toProductionDto(production),
		TerraformRating:        resourcesComponent.TerraformRating(),
		Status:                 playerStatus(p, g),
		Corporation:            corporation,
		Cards:                  handCards, // PlayerCardDto[] with state
		PlayedCards:            playedCards,
		Passed:                 p.HasPassed(),
		AvailableActions:       getAvailableActionsForPlayer(g, p.ID()),
		TotalActions:           getTotalActionsForPlayer(g, p.ID()),
		IsConnected:            p.IsConnected(),
		IsExited:               p.HasExited(),
		Effects:                convertPlayerEffects(p.Effects().List(), p, g, cardRegistry),
		Actions:                convertPlayerActions(p.Actions().List(), p, g, cardRegistry),
		StandardProjects:       standardProjects, // PlayerStandardProjectDto[] with state
		ResourceConversions:    mapPlayerResourceConversions(p, g, cardRegistry),
		Milestones:             milestones, // PlayerMilestoneDto[] with eligibility
		Awards:                 awards,     // PlayerAwardDto[] with eligibility

		DemoReady:          p.HasPendingDemoChoices(),
		PendingDemoChoices: convertPendingDemoChoices(p.PendingDemoChoices()),

		SelectCorporationPhase:          convertSelectCorporationPhase(g.GetSelectCorporationPhase(p.ID()), cardRegistry),
		SelectStartingCardsPhase:        convertSelectStartingCardsPhase(g.GetSelectStartingCardsPhase(p.ID()), cardRegistry),
		SelectPreludeCardsPhase:         convertSelectPreludeCardsPhase(g.GetSelectPreludeCardsPhase(p.ID()), cardRegistry),
		ProductionPhase:                 convertProductionPhase(g.GetProductionPhase(p.ID()), cardRegistry),
		StartingCards:                   []CardDto{},
		PendingTileSelection:            pendingTileSelection,
		PendingCardSelection:            convertPendingCardSelection(p.Selection().GetPendingCardSelection(), p, g, cardRegistry),
		PendingCardDrawSelection:        convertPendingCardDrawSelection(p.Selection().GetPendingCardDrawSelection(), p, g, cardRegistry),
		PendingBehaviorResolutions:      convertPendingBehaviorResolutions(p.Selection().GetPendingBehaviorResolutions(), p, g, cardRegistry),
		PendingResourceRemovalSelection: convertPendingResourceRemovalSelection(p.Selection().GetPendingResourceRemovalSelection()),
		PendingColonyResourceSelection:  convertPendingColonyResourceFromQueue(p.Selection().GetPendingColonyResourceQueue()),
		PendingAwardFundSelection:       convertPendingAwardFundSelection(p.Selection().GetPendingAwardFundSelection()),
		PendingColonySelection:          convertPendingColonySelection(p.Selection().GetPendingColonySelection()),
		PendingFreeTradeSelection:       convertPendingFreeTradeSelection(p.Selection().GetPendingFreeTradeSelection()),
		ForcedFirstAction:               forcedFirstAction,
		ResourceStorage:                 p.Resources().Storage(),
		PaymentSubstitutes:              convertPaymentSubstitutes(p.Resources().PaymentSubstitutes()),
		GenerationalEvents:              convertGenerationalEvents(p.GenerationalEvents().GetAll()),
		VPGranters:                      toVPGranterDtos(p.VPGranters().GetAll()),
		BonusTags:                       convertBonusTags(p.BonusTags()),
		ActionCosts:                     mapPlayerActionCosts(p, g, cardRegistry),
		HasAvailableActions:             hasAvailableActions,
	}
}

// ToOtherPlayerDto converts Player to OtherPlayerDto
func ToOtherPlayerDto(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) OtherPlayerDto {
	cardRegistry = gamecards.ForGame(cardRegistry, g)
	resourcesComponent := p.Resources()
	resources := resourcesComponent.Get()
	production := resourcesComponent.Production()

	corporation := getCorporationCard(p, cardRegistry)
	playedCardIDs := p.PlayedCards().Cards()
	playedCards := getPlayedCards(playedCardIDs, cardRegistry)
	for i := range playedCards {
		if c, err := cardRegistry.GetByID(playedCards[i].ID); err == nil {
			AnnotateCardDependencies(&playedCards[i], *c, p)
		}
	}
	handCardCount := len(p.Hand().Cards())

	return OtherPlayerDto{
		PermanentProduction: toProductionDto(p.Resources().PermanentProduction()),
		ID:                  p.ID(),
		Name:                p.Name(),
		PlayerType:          string(p.PlayerType()),
		BotStatus:           string(p.BotStatus()),
		BotPersona:          p.BotPersona(),
		BotError:            p.BotError(),
		Color:               p.Color(),
		Resources:           toResourcesDto(resources),
		Production:          toProductionDto(production),
		TerraformRating:     resourcesComponent.TerraformRating(),
		Status:              playerStatus(p, g),
		Corporation:         corporation,
		HandCardCount:       handCardCount,
		PlayedCards:         playedCards,
		Passed:              p.HasPassed(),
		AvailableActions:    getAvailableActionsForPlayer(g, p.ID()),
		TotalActions:        getTotalActionsForPlayer(g, p.ID()),
		IsConnected:         p.IsConnected(),
		IsExited:            p.HasExited(),
		Effects:             convertPlayerEffects(p.Effects().List(), p, g, cardRegistry),
		Actions:             convertPlayerActions(p.Actions().List(), p, g, cardRegistry),

		DemoReady: p.HasPendingDemoChoices(),

		SelectCorporationPhase:   convertSelectCorporationPhaseForOtherPlayer(g.GetSelectCorporationPhase(p.ID())),
		SelectStartingCardsPhase: convertSelectStartingCardsPhaseForOtherPlayer(g.GetSelectStartingCardsPhase(p.ID())),
		SelectPreludeCardsPhase:  convertSelectPreludeCardsPhaseForOtherPlayer(g.GetSelectPreludeCardsPhase(p.ID())),
		ProductionPhase:          convertProductionPhaseForOtherPlayer(g.GetProductionPhase(p.ID())),
		ResourceStorage:          p.Resources().Storage(),
		PaymentSubstitutes:       convertPaymentSubstitutes(p.Resources().PaymentSubstitutes()),
		VPGranters:               toVPGranterDtos(p.VPGranters().GetAll()),
		BonusTags:                convertBonusTags(p.BonusTags()),
	}
}

func convertPendingDemoChoices(choices *shared.PendingDemoChoices) *PendingDemoChoicesDto {
	if choices == nil {
		return nil
	}
	return &PendingDemoChoicesDto{
		CorporationID:   choices.CorporationID,
		PreludeIDs:      choices.PreludeIDs,
		CardIDs:         choices.CardIDs,
		Resources:       toResourcesDto(choices.Resources),
		Production:      toProductionDto(choices.Production),
		TerraformRating: choices.TerraformRating,
	}
}

func convertBonusTags(tags map[shared.CardTag]int) map[string]int {
	if len(tags) == 0 {
		return map[string]int{}
	}
	result := make(map[string]int, len(tags))
	for k, v := range tags {
		result[string(k)] = v
	}
	return result
}

func playerStatus(p *player.Player, g *game.Game) PlayerStatus {
	if p.HasExited() {
		return PlayerStatusExited
	}
	if g.GetPendingTileSelection(p.ID()) != nil {
		return PlayerStatusTile
	}
	if p.Selection().HasPendingSelection() {
		return PlayerStatusSelection
	}
	if g.GetSelectCorporationPhase(p.ID()) != nil ||
		g.GetSelectStartingCardsPhase(p.ID()) != nil ||
		g.GetSelectPreludeCardsPhase(p.ID()) != nil {
		return PlayerStatusSelectingStartingCards
	}
	pp := g.GetProductionPhase(p.ID())
	if pp != nil && !pp.SelectionComplete {
		return PlayerStatusSelectingProductionCards
	}
	turn := g.CurrentTurn()
	if turn != nil && turn.PlayerID() == p.ID() &&
		g.CurrentPhase() == shared.GamePhaseAction {
		return PlayerStatusActive
	}
	return PlayerStatusWaiting
}

func convertSelectCorporationPhase(phase *shared.SelectCorporationPhase, cardRegistry gamecards.CardRegistry) *SelectCorporationPhaseDto {
	if phase == nil {
		return nil
	}

	return &SelectCorporationPhaseDto{
		AvailableCorporations: getPlayedCards(phase.AvailableCorporations, cardRegistry),
	}
}

func convertSelectCorporationPhaseForOtherPlayer(phase *shared.SelectCorporationPhase) *SelectCorporationOtherPlayerDto {
	if phase == nil {
		return nil
	}

	return &SelectCorporationOtherPlayerDto{}
}

func convertSelectStartingCardsPhase(phase *shared.SelectStartingCardsPhase, cardRegistry gamecards.CardRegistry) *SelectStartingCardsPhaseDto {
	if phase == nil {
		return nil
	}

	return &SelectStartingCardsPhaseDto{
		AvailableCards: getPlayedCards(phase.AvailableCards, cardRegistry),
	}
}

func convertSelectStartingCardsPhaseForOtherPlayer(phase *shared.SelectStartingCardsPhase) *SelectStartingCardsOtherPlayerDto {
	if phase == nil {
		return nil
	}

	return &SelectStartingCardsOtherPlayerDto{}
}

// convertSelectPreludeCardsPhase converts SelectPreludeCardsPhase to DTO
func convertSelectPreludeCardsPhase(phase *shared.SelectPreludeCardsPhase, cardRegistry gamecards.CardRegistry) *SelectPreludeCardsPhaseDto {
	if phase == nil {
		return nil
	}

	return &SelectPreludeCardsPhaseDto{
		AvailablePreludes: getPlayedCards(phase.AvailablePreludes, cardRegistry),
		MaxSelectable:     phase.MaxSelectable,
	}
}

// convertSelectPreludeCardsPhaseForOtherPlayer converts SelectPreludeCardsPhase to DTO for other players
func convertSelectPreludeCardsPhaseForOtherPlayer(phase *shared.SelectPreludeCardsPhase) *SelectPreludeCardsOtherPlayerDto {
	if phase == nil {
		return nil
	}

	return &SelectPreludeCardsOtherPlayerDto{}
}

// convertProductionPhase converts production phase data to DTO for current player
func convertProductionPhase(phase *shared.ProductionPhase, cardRegistry gamecards.CardRegistry) *ProductionPhaseDto {
	if phase == nil {
		return nil
	}

	return &ProductionPhaseDto{
		ProductionUsed:    toProductionDto(phase.ProductionUsed),
		AvailableCards:    getPlayedCards(phase.AvailableCards, cardRegistry),
		SelectionComplete: phase.SelectionComplete,
		BeforeResources:   toResourcesDto(phase.BeforeResources),
		AfterResources:    toResourcesDto(phase.AfterResources),
		ResourceDelta:     calculateResourceDelta(phase.BeforeResources, phase.AfterResources),
		EnergyConverted:   phase.EnergyConverted,
		CreditsIncome:     phase.CreditsIncome,
	}
}

// convertProductionPhaseForOtherPlayer converts production phase data to DTO for other players
func convertProductionPhaseForOtherPlayer(phase *shared.ProductionPhase) *ProductionPhaseOtherPlayerDto {
	if phase == nil {
		return nil
	}

	return &ProductionPhaseOtherPlayerDto{
		ProductionUsed:    toProductionDto(phase.ProductionUsed),
		SelectionComplete: phase.SelectionComplete,
		BeforeResources:   toResourcesDto(phase.BeforeResources),
		AfterResources:    toResourcesDto(phase.AfterResources),
		ResourceDelta:     calculateResourceDelta(phase.BeforeResources, phase.AfterResources),
		EnergyConverted:   phase.EnergyConverted,
		CreditsIncome:     phase.CreditsIncome,
	}
}

// convertPlayerEffects converts CardEffect slice to PlayerEffectDto slice
func convertPlayerEffects(effects []shared.CardEffect, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) []PlayerEffectDto {
	if len(effects) == 0 {
		return []PlayerEffectDto{}
	}

	board := g.Board()
	allPlayers := g.GetAllPlayers()

	dtos := make([]PlayerEffectDto, len(effects))
	for i, effect := range effects {
		var computedValues []ComputedBehaviorValueDto
		var outputs []CalculatedOutputDto
		for _, outputBC := range effect.Behavior.Outputs {
			per := shared.GetPerCondition(outputBC)
			if per == nil {
				continue
			}
			count := gamecards.CountPerCondition(per, effect.CardID, p, board, cardRegistry, allPlayers, g.Colonies(), gamecards.TagCountContext{ActorID: p.ID()})
			if per.Amount > 0 {
				multiplier := count / per.Amount
				actualAmount := outputBC.GetAmount() * multiplier
				outputs = append(outputs, CalculatedOutputDto{
					ResourceType: string(outputBC.GetResourceType()),
					Amount:       actualAmount,
					IsScaled:     true,
				})
			}
		}
		if len(outputs) > 0 {
			computedValues = []ComputedBehaviorValueDto{{
				Target:  "behaviors::0",
				Outputs: outputs,
			}}
		}
		dtos[i] = PlayerEffectDto{
			CardID:         effect.CardID,
			CardName:       effect.CardName,
			BehaviorIndex:  effect.BehaviorIndex,
			Behavior:       toCardBehaviorDto(effect.Behavior),
			ComputedValues: computedValues,
		}
	}
	return dtos
}

// convertPlayerActions converts CardAction slice to PlayerActionDto slice
// Calculates state for each action to determine availability and errors.
func convertPlayerActions(actions []shared.CardAction, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) []PlayerActionDto {
	if len(actions) == 0 {
		return []PlayerActionDto{}
	}

	dtos := make([]PlayerActionDto, len(actions))
	for i, act := range actions {
		// Calculate state for this action
		state := action.CalculatePlayerCardActionState(
			act.CardID,
			act.Behavior,
			act.TimesUsedThisGeneration,
			p,
			g,
			cardRegistry,
		)

		behaviorDto := toCardBehaviorDto(act.Behavior)
		if card, err := cardRegistry.GetByID(act.CardID); err == nil {
			view := ToCardDto(*card)
			AnnotateCardDependencies(&view, *card, p)
			if act.BehaviorIndex >= 0 && act.BehaviorIndex < len(view.Behaviors) {
				behaviorDto.DependencyStatus = view.Behaviors[act.BehaviorIndex].DependencyStatus
			}
		}
		populateInputOptions(&behaviorDto, act.Behavior, act.CardID, p, g, cardRegistry)

		if act.Behavior.ChoicePolicy != nil && len(behaviorDto.Choices) > 0 {
			production := p.Resources().Production()
			validIndices := shared.FilterChoiceIndicesByPolicy(act.Behavior.Choices, act.Behavior.ChoicePolicy, production)
			filtered := make([]ChoiceDto, 0, len(validIndices))
			for _, idx := range validIndices {
				choice := behaviorDto.Choices[idx]
				choice.OriginalIndex = idx
				filtered = append(filtered, choice)
			}
			behaviorDto.Choices = filtered
		}

		var reuseOptions []ActionReuseOptionDto
		if action.IsActionReuse(act.Behavior) {
			reuseOptions = []ActionReuseOptionDto{}
			for _, option := range action.CalculateActionReuseOptions(act.CardID, p, g, cardRegistry) {
				reuseOptions = append(reuseOptions, ActionReuseOptionDto{CardID: option.Action.CardID, BehaviorIndex: option.Action.BehaviorIndex, Available: len(option.Errors) == 0, Errors: convertStateErrors(option.Errors)})
			}
		}

		dtos[i] = PlayerActionDto{
			DependencyTargets:       dependencyTargets(state),
			ReuseOptions:            reuseOptions,
			CardID:                  act.CardID,
			CardName:                act.CardName,
			BehaviorIndex:           act.BehaviorIndex,
			Behavior:                behaviorDto,
			TimesUsedThisTurn:       act.TimesUsedThisTurn,
			TimesUsedThisGeneration: act.TimesUsedThisGeneration,
			Available:               state.Available(),
			Errors:                  convertStateErrors(state.Errors),
			Warnings:                convertStateWarnings(state.Warnings),
			ComputedValues:          convertComputedValues(state.ComputedValues),
		}
	}
	return dtos
}

// convertComputedValues converts ComputedBehaviorValue slice to DTO slice
func convertComputedValues(values []player.ComputedBehaviorValue) []ComputedBehaviorValueDto {
	if len(values) == 0 {
		return nil
	}

	dtos := make([]ComputedBehaviorValueDto, len(values))
	for i, v := range values {
		outputs := make([]CalculatedOutputDto, len(v.Outputs))
		for j, o := range v.Outputs {
			outputs[j] = CalculatedOutputDto{
				ResourceType: o.ResourceType,
				Amount:       o.Amount,
				IsScaled:     o.IsScaled,
			}
		}
		dtos[i] = ComputedBehaviorValueDto{
			Target:  v.Target,
			Outputs: outputs,
		}
	}
	return dtos
}

func toPaymentSourceDto(s shared.PaymentSource) PaymentSourceDto {
	return PaymentSourceDto{Target: s.Target, Resource: ResourceType(s.Resource), CardID: s.CardID}
}

func ToPaymentQuoteDto(q shared.PaymentQuote) PaymentQuoteDto {
	result := PaymentQuoteDto{Costs: map[ResourceType]int{}, Options: []PaymentOptionDto{}}
	for rt, n := range q.Costs {
		result.Costs[ResourceType(rt)] = n
	}
	for _, o := range q.Options {
		result.Options = append(result.Options, PaymentOptionDto{Source: toPaymentSourceDto(o.Source), TargetResource: ResourceType(o.TargetResource), ConversionRate: o.ConversionRate, Available: o.Available})
	}
	return result
}

func convertPaymentSubstitutes(substitutes []shared.PaymentSubstitute) []PaymentSubstituteDto {
	result := make([]PaymentSubstituteDto, 0, len(substitutes))
	for _, s := range substitutes {
		result = append(result, PaymentSubstituteDto{Source: toPaymentSourceDto(s.Source), TargetResource: ResourceType(s.TargetResource), ConversionRate: s.ConversionRate, GrantedByCardID: s.GrantedByCardID, Selectors: mapSlice(s.Selectors, toSelectorDto)})
	}
	return result
}

// convertPendingCardSelection converts PendingCardSelection to DTO with playability state
func convertPendingCardSelection(selection *shared.PendingCardSelection, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) *PendingCardSelectionDto {
	if selection == nil {
		return nil
	}

	availableCards := make([]PlayerCardDto, 0, len(selection.AvailableCards))
	for _, cardID := range selection.AvailableCards {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		state := action.CalculatePendingCardPlayability(card, p, g, cardRegistry)
		availableCards = append(availableCards, ToPlayerCardDto(card, state))
	}

	return &PendingCardSelectionDto{
		AvailableCards: availableCards,
		CardCosts:      selection.CardCosts,
		CardRewards:    selection.CardRewards,
		Source:         selection.Source,
		MinCards:       selection.MinCards,
		MaxCards:       selection.MaxCards,
	}
}

// convertPendingCardDrawSelection converts PendingCardDrawSelection to DTO with playability state
func convertPendingCardDrawSelection(selection *shared.PendingCardDrawSelection, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) *PendingCardDrawSelectionDto {
	if selection == nil {
		return nil
	}

	availableCards := make([]PlayerCardDto, 0, len(selection.AvailableCards))
	for _, cardID := range selection.AvailableCards {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}
		state := action.CalculatePendingCardPlayability(card, p, g, cardRegistry)
		availableCards = append(availableCards, ToPlayerCardDto(card, state))
	}

	return &PendingCardDrawSelectionDto{
		AvailableCards:   availableCards,
		FreeTakeCount:    selection.FreeTakeCount,
		MinFreeTakeCount: selection.MinFreeTakeCount,
		MaxBuyCount:      selection.MaxBuyCount,
		CardBuyCost:      selection.CardBuyCost,
		Source:           selection.Source,
		PlayAsPrelude:    selection.PlayAsPrelude,
	}
}

func convertPendingBehaviorResolutions(resolutions []*shared.PendingBehaviorResolution, p *player.Player, g *game.Game, registry gamecards.CardRegistry) []PendingBehaviorResolutionDto {
	result := make([]PendingBehaviorResolutionDto, 0, len(resolutions))
	for _, resolution := range resolutions {
		item := PendingBehaviorResolutionDto{ID: resolution.ID, Kind: resolution.Kind, Source: resolution.Source, SourceCardID: resolution.SourceCardID, SourceBehaviorIndex: resolution.SourceBehaviorIndex, TriggeringCardID: resolution.TriggeringCardID, MinCards: resolution.MinCards, MaxCards: resolution.MaxCards, Outputs: mapSlice(resolution.PendingOutputs, toResourceConditionDto)}
		if registry != nil && resolution.TriggeringCardID != "" {
			if card, err := registry.GetByID(resolution.TriggeringCardID); err == nil {
				item.TriggeringCardName = card.Name
			}
		}
		if resolution.OptionalBehavior != nil {
			errors := action.CalculateOptionalEffectErrors(resolution, p, g, registry)
			item.OptionalEffect = &OptionalEffectDto{Behavior: toCardBehaviorDto(*resolution.OptionalBehavior), Available: len(errors) == 0, Errors: convertStateErrors(errors)}
		}
		for i, choice := range resolution.Choices {
			dto := toChoiceDtoWithState(i, choice, p, g, registry)
			errors := action.CalculateResolutionChoiceErrors(choice, resolution, p, g, registry)
			dto.Available = len(errors) == 0
			dto.Errors = convertStateErrors(errors)
			dto.StorageTargets = action.ResolutionStorageTargets(choice, p, registry)
			item.Choices = append(item.Choices, dto)
		}
		result = append(result, item)
	}
	return result
}

func convertPendingResourceRemovalSelection(selection *shared.PendingResourceRemovalSelection) *PendingResourceRemovalSelectionDto {
	if selection == nil {
		return nil
	}

	return &PendingResourceRemovalSelectionDto{
		ID:                selection.ID,
		MaxAmounts:        selection.MaxAmounts,
		EligiblePlayerIDs: selection.EligiblePlayerIDs,
		ResourceType:      string(selection.ResourceType),
		Amount:            selection.Amount,
		Source:            selection.Source,
		SourceCardID:      selection.SourceCardID,
	}
}

func convertPendingColonyResourceFromQueue(queue []shared.PendingColonyResourceSelection) *PendingColonyResourceSelectionDto {
	if len(queue) == 0 {
		return nil
	}
	selection := queue[0]
	return &PendingColonyResourceSelectionDto{
		ResourceType: selection.ResourceType,
		Amount:       selection.Amount,
		Source:       selection.Source,
		ColonyID:     selection.ColonyID,
		Reason:       ColonyResourceReason(selection.Reason),
	}
}

// toChoiceDtoWithState maps a choice to DTO with computed errors from the state calculator.
func toChoiceDtoWithState(index int, choice shared.Choice, p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) ChoiceDto {
	errors := action.CalculateChoiceErrors(choice, p, g, cardRegistry)
	return ChoiceDto{
		OriginalIndex:     index,
		StateRequirements: mapSlice(choice.StateRequirements, toStateRequirementDto),
		StorageTargets:    action.ResolutionStorageTargets(choice, p, cardRegistry),
		Inputs:            mapSlice(choice.Inputs, toResourceConditionDto),
		Outputs:           mapSlice(choice.Outputs, toResourceConditionDto),
		Requirements:      toChoiceRequirementsDto(choice.Requirements),
		Available:         len(errors) == 0,
		Errors:            convertStateErrors(errors),
	}
}

func convertPendingAwardFundSelection(selection *shared.PendingAwardFundSelection) *PendingAwardFundSelectionDto {
	if selection == nil {
		return nil
	}

	return &PendingAwardFundSelectionDto{
		AvailableAwards: selection.AvailableAwards,
		Source:          selection.Source,
	}
}

func convertPendingColonySelection(selection *shared.PendingColonySelection) *PendingColonySelectionDto {
	if selection == nil {
		return nil
	}

	return &PendingColonySelectionDto{
		AddTile:                    selection.AddTile,
		AvailableColonyIDs:         selection.AvailableColonyIDs,
		AllowDuplicatePlayerColony: selection.AllowDuplicatePlayerColony,
		Source:                     selection.Source,
		SourceCardID:               selection.SourceCardID,
	}
}

func convertPendingFreeTradeSelection(selection *shared.PendingFreeTradeSelection) *PendingFreeTradeSelectionDto {
	if selection == nil {
		return nil
	}

	return &PendingFreeTradeSelectionDto{
		AvailableColonyIDs: selection.AvailableColonyIDs,
		Source:             selection.Source,
		SourceCardID:       selection.SourceCardID,
	}
}

// convertForcedFirstAction converts ForcedFirstAction to DTO
func convertForcedFirstAction(action *shared.ForcedFirstAction) *ForcedFirstActionDto {
	if action == nil {
		return nil
	}

	return &ForcedFirstActionDto{
		State:         action.State,
		CorporationID: action.CorporationID,

		Description: action.Description,
	}
}

// convertPendingTileSelection converts PendingTileSelection to DTO
func convertPendingTileSelection(selection *shared.PendingTileSelection) *PendingTileSelectionDto {
	if selection == nil {
		return nil
	}

	return &PendingTileSelectionDto{
		TileType:       selection.TileType,
		AvailableHexes: selection.AvailableHexes,
		Source:         selection.Source,
	}
}

// getAvailableActionsForPlayer returns the available actions for a player
// Actions are now at game level, so only the current player has actions
func getAvailableActionsForPlayer(g *game.Game, playerID string) int {
	currentTurn := g.CurrentTurn()
	if currentTurn == nil {
		return 0
	}

	if currentTurn.PlayerID() == playerID {
		return currentTurn.ActionsRemaining()
	}

	return 0
}

// getTotalActionsForPlayer returns the total actions granted this turn
func getTotalActionsForPlayer(g *game.Game, playerID string) int {
	currentTurn := g.CurrentTurn()
	if currentTurn == nil {
		return 0
	}

	if currentTurn.PlayerID() == playerID {
		return currentTurn.TotalActions()
	}

	return 0
}

// convertStateErrors converts EntityState errors to DTOs.
// Since domain and DTO enums have identical string values, we cast between them.
func convertStateErrors(errors []player.StateError) []StateErrorDto {
	result := make([]StateErrorDto, len(errors))
	for i, err := range errors {
		result[i] = StateErrorDto{
			Code:     StateErrorCode(err.Code),
			Category: StateErrorCategory(err.Category),
			Message:  err.Message,
		}
	}
	return result
}

// convertStateWarnings converts EntityState warnings to DTOs.
// Since domain and DTO enums have identical string values, we cast between them.
func convertStateWarnings(warnings []player.StateWarning) []StateWarningDto {
	if len(warnings) == 0 {
		return nil
	}
	result := make([]StateWarningDto, len(warnings))
	for i, warn := range warnings {
		result[i] = StateWarningDto{
			Code:    StateWarningCode(warn.Code),
			Message: warn.Message,
		}
	}
	return result
}

// ToPlayerCardDto converts a card and its computed state to a PlayerCardDto.
func ToPlayerCardDto(card *gamecards.Card, state player.EntityState) PlayerCardDto {
	discounts := make(map[string]int)
	if discountData, ok := state.Metadata["discounts"].(map[string]int); ok {
		discounts = discountData
	}

	tags := make([]CardTag, len(card.Tags))
	for i, tag := range card.Tags {
		tags[i] = CardTag(tag)
	}

	requirements := toCardRequirementsDto(card.Requirements)

	var behaviors []CardBehaviorDto
	if len(card.Behaviors) > 0 {
		behaviors = make([]CardBehaviorDto, len(card.Behaviors))
		for i, behavior := range card.Behaviors {
			behaviors[i] = toCardBehaviorDto(behavior)
		}
	}

	var resourceStorage *ResourceStorageDto
	if card.ResourceStorage != nil {
		storage := toResourceStorageDto(*card.ResourceStorage)
		resourceStorage = &storage
	}

	var vpConditions []VPConditionDto
	if len(card.VPConditions) > 0 {
		vpConditions = make([]VPConditionDto, len(card.VPConditions))
		for i, vp := range card.VPConditions {
			vpConditions[i] = toVPConditionDto(vp)
		}
	}

	effectiveCost := 0
	if state.Cost != nil {
		if credits, ok := state.Cost[string(shared.ResourceCredit)]; ok {
			effectiveCost = credits
		}
	}

	return PlayerCardDto{
		DependencyTargets: dependencyTargets(state),
		ID:                card.ID,
		Name:              card.Name,
		Type:              CardType(card.Type),
		Cost:              card.Cost,
		Description:       toCardDescriptionDto(card.Description),

		GameModes: card.GameModes, RequiresFeatures: card.RequiresFeatures, RequiresSystems: card.RequiresSystems,
		Tags:            tags,
		Requirements:    requirements,
		Behaviors:       behaviors,
		ResourceStorage: resourceStorage,
		VPConditions:    vpConditions,
		Available:       state.Available(),
		Errors:          convertStateErrors(state.Errors),
		Warnings:        convertStateWarnings(state.Warnings),
		EffectiveCost:   effectiveCost,
		Discounts:       discounts,
		ComputedValues:  convertComputedValues(state.ComputedValues),
	}
}

// mapPlayerCards converts hand cards to DTOs using CardStateStore for cached state.
// Enriches behavior choices with computed errors from the state calculator.
func mapPlayerCards(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) []PlayerCardDto {
	handCardIDs := p.Hand().Cards()
	result := make([]PlayerCardDto, 0, len(handCardIDs))

	for _, cardID := range handCardIDs {
		state, exists := p.CardStateStore().GetState(cardID)
		if !exists {
			continue
		}
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			continue
		}

		dto := ToPlayerCardDto(card, state)
		dependencyView := CardDto{Behaviors: dto.Behaviors}
		AnnotateCardDependencies(&dependencyView, *card, p)
		dto.Behaviors = dependencyView.Behaviors

		// Enrich choices with computed errors and apply choice policy filtering
		for bi, behavior := range card.Behaviors {
			if bi < len(dto.Behaviors) {
				populateInputOptions(&dto.Behaviors[bi], behavior, card.ID, p, g, cardRegistry)
				if behavior.ChoicePolicy != nil && len(dto.Behaviors[bi].Choices) > 0 {
					production := p.Resources().Production()
					validIndices := shared.FilterChoiceIndicesByPolicy(behavior.Choices, behavior.ChoicePolicy, production)
					filtered := make([]ChoiceDto, 0, len(validIndices))
					for _, idx := range validIndices {
						choiceDto := dto.Behaviors[bi].Choices[idx]
						choiceDto.OriginalIndex = idx
						choiceErrors := action.CalculateChoiceErrors(behavior.Choices[idx], p, g, cardRegistry)
						choiceDto.Available = len(choiceErrors) == 0
						choiceDto.Errors = convertStateErrors(choiceErrors)
						filtered = append(filtered, choiceDto)
					}
					dto.Behaviors[bi].Choices = filtered
				} else {
					for ci, choice := range behavior.Choices {
						if ci < len(dto.Behaviors[bi].Choices) {
							choiceErrors := action.CalculateChoiceErrors(choice, p, g, cardRegistry)
							dto.Behaviors[bi].Choices[ci].Available = len(choiceErrors) == 0
							dto.Behaviors[bi].Choices[ci].Errors = convertStateErrors(choiceErrors)
						}
					}
				}
			}
		}

		result = append(result, dto)
	}

	return result
}

// mapPlayerStandardProjects calculates state for all standard projects and converts to DTOs.
// Uses the state calculator to compute availability and effective costs for each project.
// Conversion projects (plants→greenery, heat→temperature) are mapped by mapPlayerResourceConversions.
func mapPlayerStandardProjects(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry, stdProjRegistry standardproject.StandardProjectRegistry) []PlayerStandardProjectDto {
	if stdProjRegistry == nil {
		return nil
	}
	allDefinitions := stdProjRegistry.GetAll()
	settings := g.Settings()

	result := make([]PlayerStandardProjectDto, 0, len(allDefinitions))

	for _, def := range allDefinitions {
		if !settings.AllowsContent(def.GameModes, def.RequiresSystems, def.RequiresFeatures) {
			continue
		}
		projectType := shared.StandardProject(def.ID)
		state := action.CalculatePlayerStandardProjectState(projectType, p, g, cardRegistry)

		baseCost := map[string]int{}
		if creditCost := def.CreditCost(); creditCost > 0 {
			baseCost[string(shared.ResourceCredit)] = creditCost
		}

		discounts := make(map[string]int)
		if discountData, ok := state.Metadata["discounts"].(map[string]int); ok {
			discounts = discountData
		}

		behaviorDtos := make([]CardBehaviorDto, 0, len(def.Behaviors))
		for _, b := range def.Behaviors {
			behaviorDtos = append(behaviorDtos, toCardBehaviorDto(b))
		}

		dto := PlayerStandardProjectDto{
			ProjectType:   def.ID,
			Name:          def.Name,
			Description:   def.Description,
			Behaviors:     behaviorDtos,
			Style:         &StyleDto{Color: def.Style.Color, Icon: def.Style.Icon},
			BaseCost:      baseCost,
			Available:     state.Available(),
			Errors:        convertStateErrors(state.Errors),
			Warnings:      convertStateWarnings(state.Warnings),
			EffectiveCost: state.Cost,
			Discounts:     discounts,
			Metadata:      state.Metadata,
		}

		result = append(result, dto)
	}

	return result
}

func mapPlayerResourceConversions(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) []PlayerResourceConversionDto {
	result := make([]PlayerResourceConversionDto, 0, len(action.ConversionStandardProjects))
	for _, projectType := range action.ConversionStandardProjects {
		state := action.CalculatePlayerStandardProjectState(projectType, p, g, cardRegistry)
		result = append(result, PlayerResourceConversionDto{
			ProjectType:   StandardProject(projectType),
			Available:     state.Available(),
			Errors:        convertStateErrors(state.Errors),
			Warnings:      convertStateWarnings(state.Warnings),
			EffectiveCost: state.Cost,
		})
	}
	return result
}

// mapPlayerMilestones calculates state for all milestones and converts to DTOs.
// Uses the state calculator to compute availability on-the-fly (same pattern as standard projects).
func mapPlayerMilestones(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry, milestoneRegistry milestone.MilestoneRegistry) []PlayerMilestoneDto {
	if milestoneRegistry == nil {
		return nil
	}
	filteredDefs := action.FilterMilestones(milestoneRegistry.GetAll(), g.SelectedMilestones(), g.Settings())

	result := make([]PlayerMilestoneDto, 0, len(filteredDefs))
	gameMilestones := g.Milestones()

	for _, def := range filteredDefs {
		milestoneType := shared.MilestoneType(def.ID)
		state := action.CalculateMilestoneState(milestoneType, p, g, cardRegistry, milestoneRegistry)

		isClaimed := gameMilestones.IsClaimed(milestoneType)
		var claimedBy *string
		for _, claimed := range gameMilestones.ClaimedMilestones() {
			if claimed.Type == milestoneType {
				claimedBy = &claimed.PlayerID
				break
			}
		}

		progress := 0
		if prog, ok := state.Metadata["progress"].(int); ok {
			progress = prog
		}

		rewardDtos := buildMilestoneRewardDtos(def.Reward)

		var styleDtoPtr *StyleDto
		if def.Style.Color != "" || def.Style.Icon != "" {
			styleDtoPtr = &StyleDto{Color: def.Style.Color, Icon: def.Style.Icon}
		}

		dto := PlayerMilestoneDto{
			Type:        def.ID,
			Name:        def.Name,
			Description: def.Description,
			ClaimCost:   def.ClaimCost,
			IsClaimed:   isClaimed,
			ClaimedBy:   claimedBy,
			Available:   state.Available(),
			Progress:    progress,
			Required:    def.GetRequired(),
			Errors:      convertStateErrors(state.Errors),
			Reward:      rewardDtos,
			Style:       styleDtoPtr,
		}
		result = append(result, dto)
	}

	return result
}

// convertGenerationalEvents converts PlayerGenerationalEventEntry slice to DTO slice
func convertGenerationalEvents(entries []shared.PlayerGenerationalEventEntry) []PlayerGenerationalEventEntryDto {
	if len(entries) == 0 {
		return []PlayerGenerationalEventEntryDto{}
	}

	dtos := make([]PlayerGenerationalEventEntryDto, len(entries))
	for i, entry := range entries {
		dtos[i] = PlayerGenerationalEventEntryDto{
			Event: GenerationalEvent(entry.Event),
			Count: entry.Count,
		}
	}
	return dtos
}

// mapPlayerAwards calculates state for all awards and converts to DTOs.
// Uses the state calculator to compute availability on-the-fly (same pattern as standard projects).
func mapPlayerAwards(p *player.Player, g *game.Game, awardRegistry award.AwardRegistry, cardRegistry gamecards.CardRegistry) []PlayerAwardDto {
	if awardRegistry == nil {
		return nil
	}
	filteredDefs := action.FilterAwards(awardRegistry.GetAll(), g.SelectedAwards(), g.Settings())

	result := make([]PlayerAwardDto, 0, len(filteredDefs))
	gameAwards := g.Awards()
	fundedCount := gameAwards.FundedCount()

	for _, def := range filteredDefs {
		awardType := shared.AwardType(def.ID)
		state := action.CalculateAwardState(awardType, p, g, awardRegistry, cardRegistry)

		isFunded := gameAwards.IsFunded(awardType)
		var fundedBy *string
		for _, funded := range gameAwards.FundedAwards() {
			if funded.Type == awardType {
				fundedBy = &funded.FundedByPlayer
				break
			}
		}

		var styleDtoPtr *StyleDto
		if def.Style.Color != "" || def.Style.Icon != "" {
			styleDtoPtr = &StyleDto{Color: def.Style.Color, Icon: def.Style.Icon}
		}

		dto := PlayerAwardDto{
			Type:        def.ID,
			Name:        def.Name,
			Description: def.Description,
			FundingCost: def.GetCostForFundedCount(fundedCount),
			IsFunded:    isFunded,
			FundedBy:    fundedBy,
			Available:   state.Available(),
			Errors:      convertStateErrors(state.Errors),
			Style:       styleDtoPtr,
		}
		result = append(result, dto)
	}

	return result
}

func mapPlayerActionCosts(p *player.Player, g *game.Game, cardRegistry gamecards.CardRegistry) []ActionCostDto {
	calc := gamecards.NewRequirementModifierCalculator(cardRegistry)

	cardBuyDiscounts := calc.CalculateActionDiscounts(p, shared.ActionCardBuying)
	creditDiscount := cardBuyDiscounts[shared.ResourceCredit]
	cardBuyBase := 3
	cardBuyEffective := cardBuyBase - creditDiscount
	if cardBuyEffective < 0 {
		cardBuyEffective = 0
	}

	result := []ActionCostDto{
		{
			ActionType: shared.ActionCardBuying,
			Costs: []ActionCostEntryDto{
				{
					Resource:        string(shared.ResourceCredit),
					BaseCost:        cardBuyBase,
					EffectiveCost:   cardBuyEffective,
					PaymentCapacity: gamecards.PaymentCapacity(p, g, cardRegistry, shared.ActionCardBuying, shared.ResourceCredit),
					Discount:        creditDiscount,
				},
			},
		},
	}

	if g.HasColonies() {
		effectiveTradeCosts, _ := action.CalculateEffectiveTradeCosts(p, cardRegistry)
		tradeBaseCosts := []struct {
			resource shared.ResourceType
			baseCost int
		}{
			{shared.ResourceCredit, action.TradeCreditsCost},
			{shared.ResourceEnergy, action.TradeEnergyCost},
			{shared.ResourceTitanium, action.TradeTitaniumCost},
		}
		tradeCosts := make([]ActionCostEntryDto, 0, len(tradeBaseCosts))
		for _, bc := range tradeBaseCosts {
			effective := effectiveTradeCosts[string(bc.resource)]
			tradeCosts = append(tradeCosts, ActionCostEntryDto{
				Resource:        string(bc.resource),
				BaseCost:        bc.baseCost,
				EffectiveCost:   effective,
				PaymentCapacity: gamecards.PaymentCapacity(p, g, cardRegistry, shared.ActionColonyTrade, bc.resource),
				Discount:        bc.baseCost - effective,
			})
		}
		result = append(result, ActionCostDto{
			ActionType: shared.ActionColonyTrade,
			Costs:      tradeCosts,
		})
	}

	return result
}

func convertPendingEffectSelection(selection *shared.PendingEffectSelection) *PendingEffectSelectionDto {
	if selection == nil {
		return nil
	}
	return &PendingEffectSelectionDto{Source: selection.Source, SourceCardID: selection.SourceCardID, Outputs: mapSlice(selection.Outputs, toResourceConditionDto), Options: mapSlice(selection.Options, func(o shared.EffectSelectionOption) EffectSelectionOptionDto {
		return EffectSelectionOptionDto{CardID: o.CardID, TargetPlayerID: o.TargetPlayerID, ColonyIDs: o.ColonyIDs, Outputs: mapSlice(o.Outputs, toResourceConditionDto)}
	})}
}

func toRevealedCardDto(card shared.RevealedCard) RevealedCardDto {
	return RevealedCardDto{CardID: card.CardID, Name: card.Name, Matched: card.Matched}
}
func convertPendingCardReveal(reveal *shared.PendingCardReveal, registry gamecards.CardRegistry) *PendingCardRevealDto {
	if reveal == nil {
		return nil
	}
	result := &PendingCardRevealDto{Source: reveal.Source, SourceCardID: reveal.SourceCardID, Cards: []CardDto{}, Results: mapSlice(reveal.Cards, toRevealedCardDto), Rewards: []CalculatedOutputDto{}}
	for _, entry := range reveal.Cards {
		if card, err := registry.GetByID(entry.CardID); err == nil {
			result.Cards = append(result.Cards, ToCardDto(*card))
		}
	}
	for _, reward := range reveal.Rewards {
		result.Rewards = append(result.Rewards, CalculatedOutputDto{ResourceType: reward.ResourceType, Amount: reward.Amount, IsScaled: reward.IsScaled})
	}
	return result
}

func populateInputOptions(dto *CardBehaviorDto, behavior shared.CardBehavior, cardID string, p *player.Player, g *game.Game, registry gamecards.CardRegistry) {
	applier := gamecards.NewBehaviorApplier(p, g, "", nil).WithSourceCardID(cardID).WithCardRegistry(registry)
	convert := func(inputs []shared.BehaviorCondition) *BehaviorInputOptionsDto {
		options := applier.InputOptions(inputs)
		if len(options.StorageSources) == 0 && options.VariableAmount == nil {
			return nil
		}
		result := &BehaviorInputOptionsDto{StorageSources: options.StorageSources}
		if v := options.VariableAmount; v != nil {
			result.VariableAmount = &VariableInputAmountDto{ResourceType: ResourceType(v.ResourceType), Min: v.Min, Max: v.Max}
		}
		return result
	}
	dto.InputOptions = convert(behavior.Inputs)
	for i := range behavior.Choices {
		inputs, _ := behavior.ExtractInputsOutputs(&i)
		dto.Choices[i].InputOptions = convert(inputs)
	}
}

// ToPayment maps a client selection without coercing resource quantities.
func ToPayment(p PaymentDto) shared.Payment {
	result := shared.Payment{Allocations: []shared.PaymentAllocation{}}
	for _, a := range p.Allocations {
		result.Allocations = append(result.Allocations, shared.PaymentAllocation{Source: shared.PaymentSource{Target: a.Source.Target, Resource: shared.ResourceType(a.Source.Resource), CardID: a.Source.CardID}, TargetResource: shared.ResourceType(a.TargetResource), Amount: a.Amount})
	}
	return result
}

func convertCardReceipts(receipts []shared.CardReceipt, registry gamecards.CardRegistry) []CardReceiptDto {
	result := make([]CardReceiptDto, 0, len(receipts))
	for _, receipt := range receipts {
		item := CardReceiptDto{ID: receipt.ID, Source: receipt.Source, SourceCardID: receipt.SourceCardID, Cards: []CardDto{}}
		for _, id := range receipt.Cards {
			if card, err := registry.GetByID(id); err == nil {
				item.Cards = append(item.Cards, ToCardDto(*card))
			}
		}
		result = append(result, item)
	}
	return result
}

func dependencyTargets(state player.EntityState) []string {
	targets, _ := state.Metadata["dependencyTargets"].([]string)
	return targets
}

func modeProductionPreview(p *player.Player, g *game.Game) *ModeProductionDto {
	runtime := g.ModeRuntime()
	if len(runtime.Definition.Deadlines) == 0 {
		return nil
	}
	deadline := runtime.Definition.Deadlines[0]
	runtime.Windows = nil
	runtime.Values[deadline.ActiveState] = deadline.ActiveValue
	connected := p.Resources().ProductionForMode(runtime)
	runtime.Values[deadline.ActiveState] = deadline.InactiveValue
	runtime.Values[deadline.OccurredState] = 1
	disconnected := p.Resources().ProductionForMode(runtime)
	return &ModeProductionDto{Connected: toProductionDto(connected), Disconnected: toProductionDto(disconnected)}
}
