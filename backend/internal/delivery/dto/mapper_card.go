package dto

import (
	"log/slog"
	gamecards "openmars/internal/game/cards"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
	"openmars/internal/logger"
)

// ToCardDto converts a Card to CardDto
func ToCardDto(card gamecards.Card) CardDto {
	out := CardDto{
		ID:          card.ID,
		Style:       toCardStyleDto(card.Style),
		Name:        card.Name,
		Type:        CardType(card.Type),
		Cost:        card.Cost,
		Description: toCardDescriptionDto(card.Description),

		GameModes: card.GameModes, RequiresFeatures: card.RequiresFeatures, RequiresSystems: card.RequiresSystems,
		Tags:               mapSlice(card.Tags, func(t shared.CardTag) CardTag { return CardTag(t) }),
		Requirements:       toCardRequirementsDto(card.Requirements),
		Behaviors:          mapSlice(card.Behaviors, toCardBehaviorDto),
		ResourceStorage:    ptrCast(card.ResourceStorage, toResourceStorageDto),
		VPConditions:       mapSlice(card.VPConditions, toVPConditionDto),
		StartingResources:  ptrCast(card.StartingResources, toResourceSetDto),
		StartingProduction: ptrCast(card.StartingProduction, toResourceSetDto),
	}
	AnnotateCardDependencies(&out, card, nil)
	return out
}

// toCardDescriptionDto converts a card description into its section DTOs
func toCardDescriptionDto(description gamecards.CardDescription) []CardDescriptionSectionDto {
	return mapSlice(description, func(s gamecards.DescriptionSection) CardDescriptionSectionDto {
		return CardDescriptionSectionDto{Type: string(s.Type), Text: s.Text}
	})
}

// CardDescriptionPlainText flattens description section DTOs into labelled lines
func CardDescriptionPlainText(sections []CardDescriptionSectionDto) string {
	description := make(gamecards.CardDescription, len(sections))
	for i, s := range sections {
		description[i] = gamecards.DescriptionSection{Type: gamecards.DescriptionSectionType(s.Type), Text: s.Text}
	}
	return description.PlainText()
}

// toResourceSetDto converts shared.ResourceSet to ResourceSet DTO.
func toResourceSetDto(rs shared.ResourceSet) ResourceSet {
	return ResourceSet{
		Credits:  rs.Credits,
		Steel:    rs.Steel,
		Titanium: rs.Titanium,
		Plants:   rs.Plants,
		Energy:   rs.Energy,
		Heat:     rs.Heat,
	}
}

// getCorporationCard fetches the corporation card for a player using the card registry
func getCorporationCard(p *player.Player, cardRegistry gamecards.CardRegistry) *CardDto {
	if p.CorporationID() == "" {
		return nil
	}

	card, err := cardRegistry.GetByID(p.CorporationID())
	if err != nil {
		log := logger.Get()
		log.Warn("Failed to fetch corporation card",
			slog.String("player_id", p.ID()),
			slog.String("corporation_id", p.CorporationID()),
			slog.Any("error", err))
		return nil
	}

	cardDto := ToCardDto(*card)
	AnnotateCardDependencies(&cardDto, *card, p)
	return &cardDto
}

// getPlayedCards converts a slice of card IDs to CardDto objects using the card registry
func getPlayedCards(cardIDs []string, cardRegistry gamecards.CardRegistry) []CardDto {
	cardDtos := make([]CardDto, 0, len(cardIDs))
	log := logger.Get()

	for _, cardID := range cardIDs {
		card, err := cardRegistry.GetByID(cardID)
		if err != nil {
			log.Warn("Failed to fetch played card",
				slog.String("card_id", cardID),
				slog.Any("error", err))
			continue // Skip cards that can't be found
		}
		cardDtos = append(cardDtos, ToCardDto(*card))
	}

	return cardDtos
}

// Card-related helper functions for nested DTO conversions

func toCardRequirementsDto(reqs *gamecards.CardRequirements) *CardRequirementsDto {
	if reqs == nil {
		return nil
	}
	return &CardRequirementsDto{
		Description: reqs.Description,
		Items:       mapSlice(reqs.Items, toRequirementDto),
	}
}

func toRequirementDto(req gamecards.Requirement) RequirementDto {
	return RequirementDto{Permanent: req.Permanent, Target: req.Target,
		State: req.State, Equals: req.Equals,
		Type:     RequirementType(req.Type),
		Min:      req.Min,
		Max:      req.Max,
		Location: ptrCast(req.Location, func(l gamecards.CardApplyLocation) CardApplyLocation { return CardApplyLocation(l) }),
		Tag:      ptrCast(req.Tag, func(t shared.CardTag) CardTag { return CardTag(t) }),
		Resource: ptrCast(req.Resource, func(r shared.ResourceType) ResourceType { return ResourceType(r) }),
	}
}

func toCardBehaviorDto(behavior shared.CardBehavior) CardBehaviorDto {
	return CardBehaviorDto{
		ID: behavior.ID, Dependency: behavior.Dependency, MaxUsesPerGame: behavior.MaxUsesPerGame, UsageGroup: behavior.UsageGroup, ResourceRequirements: mapSlice(behavior.ResourceRequirements, func(r shared.BehaviorResourceRequirement) BehaviorResourceRequirementDto {
			return BehaviorResourceRequirementDto{Location: r.Location, Type: ResourceType(r.Type), Min: r.Min, Max: r.Max, Permanent: r.Permanent, Target: r.Target}
		}), MaxUsesPerGeneration: behavior.MaxUsesPerGeneration, StateRequirements: mapSlice(behavior.StateRequirements, toStateRequirementDto),
		ProductionBox: behavior.ProductionBox,
		Optional:      behavior.Optional,
		Description:   behavior.Description,
		Triggers:      mapSlice(behavior.Triggers, toTriggerDto),
		Inputs:        mapSlice(behavior.Inputs, toResourceConditionDto),
		Outputs:       mapSlice(behavior.Outputs, toResourceConditionDto),

		Choices:                       toChoiceDtos(behavior.Choices),
		ChoicePolicy:                  toChoicePolicyDto(behavior.ChoicePolicy),
		GenerationalEventRequirements: mapSlice(behavior.GenerationalEventRequirements, toGenerationalEventRequirementDto),
		Group:                         behavior.Group,
	}
}

func toChoicePolicyDto(cp *shared.ChoicePolicy) *ChoicePolicyDto {
	if cp == nil {
		return nil
	}
	dto := &ChoicePolicyDto{
		Type:    string(cp.Type),
		Default: cp.Default,
	}
	if cp.Select != nil {
		sel := &ChoicePolicySelectDto{
			Option:       cp.Select.Option,
			MinMax:       MinMaxValueDto{Min: cp.Select.MinMax.Min, Max: cp.Select.MinMax.Max},
			ResourceType: cp.Select.ResourceType,
		}
		if cp.Select.Tag != nil {
			s := string(*cp.Select.Tag)
			sel.Tag = &s
		}
		dto.Select = sel
	}
	return dto
}

func toGenerationalEventRequirementDto(req shared.GenerationalEventRequirement) GenerationalEventRequirementDto {
	var countDto *MinMaxValueDto
	if req.Count != nil {
		countDto = &MinMaxValueDto{
			Min: req.Count.Min,
			Max: req.Count.Max,
		}
	}

	return GenerationalEventRequirementDto{
		Event:  GenerationalEvent(req.Event),
		Count:  countDto,
		Target: ptrCast(req.Target, func(t string) TargetType { return TargetType(t) }),
	}
}

func toTriggerDto(trigger shared.Trigger) TriggerDto {
	return TriggerDto{
		Type:      ResourceTriggerType(trigger.Type),
		Condition: ptrCast(trigger.Condition, toResourceTriggerConditionDto),
	}
}

func toResourceTriggerConditionDto(cond shared.ResourceTriggerCondition) ResourceTriggerConditionDto {
	return ResourceTriggerConditionDto{
		State: cond.State, To: cond.To, Direction: cond.Direction,
		Type:                 TriggerType(cond.Type),
		ResourceTypes:        mapSlice(cond.ResourceTypes, func(rt shared.ResourceType) ResourceType { return ResourceType(rt) }),
		Location:             ptrCast(cond.Location, func(l string) CardApplyLocation { return CardApplyLocation(l) }),
		Selectors:            mapSlice(cond.Selectors, toSelectorDto),
		Target:               ptrCast(cond.Target, func(t string) TargetType { return TargetType(t) }),
		RequiredOriginalCost: ptrCast(cond.RequiredOriginalCost, toMinMaxValueDto),
		OnBonusType:          cond.OnBonusType,
		Unique:               cond.Unique,
	}
}

func toMinMaxValueDto(v shared.MinMaxValue) MinMaxValueDto {
	return MinMaxValueDto{
		Min: v.Min,
		Max: v.Max,
	}
}

func toTileRestrictionsDto(tr shared.TileRestrictions) TileRestrictionsDto {
	dto := TileRestrictionsDto{
		BoardTags:         tr.BoardTags,
		Adjacency:         tr.Adjacency,
		Area:              tr.Area,
		AdjacentToType:    tr.AdjacentToType,
		MinAdjacentOfType: tr.MinAdjacentOfType,
	}
	if tr.AdjacentToOwned {
		dto.AdjacentToOwned = &tr.AdjacentToOwned
	}
	dto.OnBonusType = tr.OnBonusType
	return dto
}

func toTargetRestrictionDto(tr shared.TargetRestriction) TargetRestrictionDto {
	return TargetRestrictionDto{
		Adjacent:  tr.Adjacent,
		Selectors: mapSlice(tr.Selectors, toSelectorDto),
	}
}

func toSelectorDto(sel shared.Selector) SelectorDto {
	return SelectorDto{Dependency: sel.Dependency,
		TagCount:             ptrCast(sel.TagCount, toMinMaxValueDto),
		Tags:                 mapSlice(sel.Tags, func(t shared.CardTag) CardTag { return CardTag(t) }),
		CardTypes:            mapSlice(sel.CardTypes, func(ct string) CardType { return CardType(ct) }),
		Resources:            sel.Resources,
		StandardProjects:     mapSlice(sel.StandardProjects, func(sp shared.StandardProject) StandardProject { return StandardProject(sp) }),
		RequiredOriginalCost: ptrCast(sel.RequiredOriginalCost, toMinMaxValueDto),
		VP:                   ptrCast(sel.VP, toMinMaxValueDto),
		GlobalParameters:     sel.GlobalParameters,
		Actions:              sel.Actions,
	}
}

func toResourceConditionDto(bc shared.BehaviorCondition) any {
	rt := string(bc.GetResourceType())
	target := TargetType(bc.GetTarget())
	amount := bc.GetAmount()

	switch c := bc.(type) {
	case *shared.StateCondition:
		return StateConditionDto{Description: stateOperationDescription(c), MinimumProduction: ptrCast(c.MinimumProduction, func(t shared.ProductionThreshold) ProductionThresholdDto {
			return ProductionThresholdDto{Type: ResourceType(t.Type), Amount: t.Amount}
		}), Type: rt, Amount: amount, Target: target, State: c.State, Value: c.Value, Dependency: c.Dependency, Start: ptrCast(c.Start, func(b shared.WindowBoundary) WindowBoundaryDto {
			return WindowBoundaryDto{GenerationOffset: b.GenerationOffset, Boundary: b.Boundary}
		}), End: ptrCast(c.End, func(b shared.WindowBoundary) WindowBoundaryDto {
			return WindowBoundaryDto{GenerationOffset: b.GenerationOffset, Boundary: b.Boundary}
		})}
	case *shared.CardRevealCondition:
		return CardRevealConditionDto{Type: rt, Amount: amount, Target: target, Destination: c.Destination, OnMatch: ptrCast(c.OnMatch, func(m shared.RevealMatch) RevealMatchDto {
			return RevealMatchDto{Selectors: mapSlice(m.Selectors, toSelectorDto), Outputs: mapSlice(m.Outputs, toResourceConditionDto)}
		})}
	case *shared.CopyCondition:
		return CopyConditionDto{Type: rt, Amount: amount, Target: target, Scope: c.Scope, Zone: c.Zone, Selectors: mapSlice(c.Selectors, toSelectorDto)}
	case *shared.BasicResourceCondition:
		dto := BasicResourceConditionDto{
			Type: rt, Amount: amount, Target: target,
			Per:               ptrCast(c.Per, toPerConditionDto),
			TargetRestriction: ptrCast(c.TargetRestriction, toTargetRestrictionDto),
			MaxTrigger:        c.MaxTrigger,
		}
		if c.VariableAmount {
			dto.VariableAmount = &c.VariableAmount
		}
		return dto
	case *shared.ProductionCondition:
		dto := ProductionConditionDto{
			Type: rt, Amount: amount, Target: target,
			Per: ptrCast(c.Per, toPerConditionDto),
		}
		if c.VariableAmount {
			dto.VariableAmount = &c.VariableAmount
		}
		return dto
	case *shared.TilePlacementCondition:
		return TilePlacementConditionDto{
			Type: rt, Amount: amount, Target: target,
			TileRestrictions: ptrCast(c.TileRestrictions, toTileRestrictionsDto),
			TileType:         c.TileType,
		}
	case *shared.GlobalParameterCondition:
		return GlobalParameterConditionDto{
			Type: rt, Amount: amount, Target: target,
			Per: ptrCast(c.Per, toPerConditionDto),
		}
	case *shared.CardOperationCondition:
		dto := CardOperationConditionDto{
			Optional: c.Optional,
			Per:      ptrCast(c.Per, toPerConditionDto),
			Type:     rt, Amount: amount, Target: target,
			Selectors: mapSlice(c.Selectors, toSelectorDto),
		}
		if c.VariableAmount {
			dto.VariableAmount = &c.VariableAmount
		}
		return dto
	case *shared.CardStorageCondition:
		dto := CardStorageConditionDto{
			Type: rt, Amount: amount, Target: target,
			Selectors: mapSlice(c.Selectors, toSelectorDto),
			Per:       ptrCast(c.Per, toPerConditionDto),
		}
		if c.VariableAmount {
			dto.VariableAmount = &c.VariableAmount
		}
		return dto
	case *shared.PaymentSubstituteCondition:
		return PaymentSubstituteConditionDto{Type: rt, Amount: amount, Target: target, Source: toPaymentSourceDto(c.Source), TargetResource: ResourceType(c.TargetResource), Selectors: mapSlice(c.Selectors, toSelectorDto)}
	case *shared.EffectCondition:
		return EffectConditionDto{
			Temporary: c.Temporary,
			Against:   c.Against,
			Type:      rt, Amount: amount, Target: target,
			Selectors: mapSlice(c.Selectors, toSelectorDto),
		}
	case *shared.ColonyCondition:
		return ColonyConditionDto{
			SelectionGroup: c.SelectionGroup,
			Optional:       c.Optional,
			Type:           rt, Amount: amount, Target: target,
		}
	case *shared.TileModificationCondition:
		return TileModificationConditionDto{
			Type: rt, Amount: amount, Target: target,
			TileType: c.TileType,
		}
	case *shared.MiscCondition:
		return MiscConditionDto{
			Type: rt, Amount: amount, Target: target,
			Per:       ptrCast(c.Per, toPerConditionDto),
			Selectors: mapSlice(c.Selectors, toSelectorDto),
		}
	default:
		return BasicResourceConditionDto{
			Type: rt, Amount: amount, Target: target,
		}
	}
}

func toChoiceDtos(choices []shared.Choice) []ChoiceDto {
	if len(choices) == 0 {
		return nil
	}
	dtos := make([]ChoiceDto, len(choices))
	for i, choice := range choices {
		dtos[i] = toChoiceDto(i, choice)
	}
	return dtos
}

func toChoiceDto(index int, choice shared.Choice) ChoiceDto {
	return ChoiceDto{StateRequirements: mapSlice(choice.StateRequirements, toStateRequirementDto),
		OriginalIndex: index,
		Inputs:        mapSlice(choice.Inputs, toResourceConditionDto),
		Outputs:       mapSlice(choice.Outputs, toResourceConditionDto),
		Requirements:  toChoiceRequirementsDto(choice.Requirements),
		Available:     true,
		Errors:        []StateErrorDto{},
	}
}

func toChoiceRequirementsDto(reqs *shared.ChoiceRequirements) *CardRequirementsDto {
	if reqs == nil {
		return nil
	}
	return &CardRequirementsDto{
		Items: mapSlice(reqs.Items, toChoiceRequirementDto),
	}
}

func toChoiceRequirementDto(req shared.ChoiceRequirement) RequirementDto {
	return RequirementDto{
		Type:     RequirementType(req.Type),
		Min:      req.Min,
		Max:      req.Max,
		Location: ptrCast(req.Location, func(l string) CardApplyLocation { return CardApplyLocation(l) }),
		Tag:      ptrCast(req.Tag, func(t shared.CardTag) CardTag { return CardTag(t) }),
		Resource: ptrCast(req.Resource, func(r shared.ResourceType) ResourceType { return ResourceType(r) }),
	}
}

func toPerConditionDto(pc shared.PerCondition) PerConditionDto {
	return PerConditionDto{
		Zone: pc.Zone, Selectors: mapSlice(pc.Selectors, toSelectorDto), IncludeSource: pc.IncludeSource,
		Type:               ResourceType(pc.ResourceType),
		Amount:             pc.Amount,
		Location:           ptrCast(pc.Location, func(l string) CardApplyLocation { return CardApplyLocation(l) }),
		Target:             ptrCast(pc.Target, func(t string) TargetType { return TargetType(t) }),
		Tag:                ptrCast(pc.Tag, func(t shared.CardTag) CardTag { return CardTag(t) }),
		Tags:               mapSlice(pc.Tags, func(t shared.CardTag) CardTag { return CardTag(t) }),
		AdjacentToSelfTile: pc.AdjacentToSelfTile,
	}
}

func toResourceStorageDto(storage gamecards.ResourceStorage) ResourceStorageDto {
	return ResourceStorageDto{
		Type:        ResourceType(storage.Type),
		Icon:        storage.Icon,
		Label:       storage.Label,
		Capacity:    storage.Capacity,
		Starting:    storage.Starting,
		Description: storage.Description,
	}
}

func toVPConditionDto(vp gamecards.VictoryPointCondition) VPConditionDto {
	return VPConditionDto{
		StateRequirements: mapSlice(vp.StateRequirements, toStateRequirementDto),
		Amount:            vp.Amount,
		Condition:         VPConditionType(vp.Condition),
		MaxTrigger:        vp.MaxTrigger,
		Per:               ptrCast(vp.Per, toVPPerConditionDto),
		Description:       vp.Description,
	}
}

func toVPPerConditionDto(pc shared.PerCondition) PerConditionDto {
	return toPerConditionDto(pc)
}

func toCardStyleDto(style *shared.CardStyle) *CardStyleDto {
	if style == nil {
		return nil
	}
	return &CardStyleDto{Tile: toStructureStyleDto(style.Tile)}
}
func toStructureStyleDto(style *shared.StructureStyleRequest) *StructureStyleRequestDto {
	if style == nil {
		return nil
	}
	return &StructureStyleRequestDto{
		Plan:          style.Plan,
		Density:       style.Density,
		Heights:       style.Heights,
		Ground:        style.Ground,
		Exposure:      style.Exposure,
		Perimeter:     style.Perimeter,
		Cover:         style.Cover,
		Landmark:      style.Landmark,
		Connections:   style.Connections,
		Landscaping:   style.Landscaping,
		Lighting:      style.Lighting,
		PerimeterRoad: style.PerimeterRoad,
		EntranceMin:   style.EntranceMin,
		EntranceMax:   style.EntranceMax,
		Details:       style.Details,
	}
}
func toTileVisualDto(visual *shared.TileVisual) *TileVisualDto {
	if visual == nil {
		return nil
	}
	return &TileVisualDto{Seed: visual.Seed, Structure: toStructureStyleDto(visual.Structure)}
}

func calculatedOperationDto(operation *shared.StateCondition) *StateConditionDto {
	if operation == nil {
		return nil
	}
	result := toResourceConditionDto(operation).(StateConditionDto)
	return &result
}
