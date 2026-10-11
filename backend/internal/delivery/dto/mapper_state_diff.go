package dto

import (
	"openmars/internal/game"
	"openmars/internal/game/shared"
)

// ToStateDiffDto converts a domain StateDiff to a DTO
func ToStateDiffDto(diff *game.StateDiff) StateDiffDto {
	var calculatedOutputs []CalculatedOutputDto
	if len(diff.CalculatedOutputs) > 0 {
		calculatedOutputs = make([]CalculatedOutputDto, len(diff.CalculatedOutputs))
		for i, output := range diff.CalculatedOutputs {
			calculatedOutputs[i] = CalculatedOutputDto{
				ResourceType: output.ResourceType,
				Amount:       output.Amount,
				IsScaled:     output.IsScaled,
				Operation:    calculatedOperationDto(output.Operation),
			}
		}
	}

	return StateDiffDto{
		SequenceNumber:    diff.SequenceNumber,
		Timestamp:         diff.Timestamp.Format("2006-01-02T15:04:05.000Z"),
		GameID:            diff.GameID,
		Changes:           toGameChangesDto(diff.Changes),
		Source:            diff.Source,
		SourceType:        string(diff.SourceType),
		PlayerID:          diff.PlayerID,
		Description:       diff.Description,
		ChoiceIndex:       diff.ChoiceIndex,
		CalculatedOutputs: calculatedOutputs,
		DisplayData:       toLogDisplayDataDto(diff.DisplayData),
	}
}

// ToStateDiffDtos converts a slice of domain StateDiffs to DTOs
func ToStateDiffDtos(diffs []game.StateDiff) []StateDiffDto {
	result := make([]StateDiffDto, len(diffs))
	for i, diff := range diffs {
		result[i] = ToStateDiffDto(&diff)
	}
	return result
}

// ToDiffLogDto converts a domain DiffLog to a DTO
func ToDiffLogDto(log *game.DiffLog) DiffLogDto {
	return DiffLogDto{
		GameID:          log.GameID,
		Diffs:           ToStateDiffDtos(log.Diffs),
		CurrentSequence: log.CurrentSequence,
	}
}

func toGameChangesDto(changes *game.GameChanges) *GameChangesDto {
	if changes == nil {
		return nil
	}

	dto := &GameChangesDto{}

	if changes.Status != nil {
		dto.Status = &DiffValueStringDto{Old: changes.Status.Old, New: changes.Status.New}
	}
	if changes.Phase != nil {
		dto.Phase = &DiffValueStringDto{Old: changes.Phase.Old, New: changes.Phase.New}
	}
	if changes.Generation != nil {
		dto.Generation = &DiffValueIntDto{Old: changes.Generation.Old, New: changes.Generation.New}
	}
	if changes.CurrentTurnPlayerID != nil {
		dto.CurrentTurnPlayerID = &DiffValueStringDto{Old: changes.CurrentTurnPlayerID.Old, New: changes.CurrentTurnPlayerID.New}
	}
	if changes.Temperature != nil {
		dto.Temperature = &DiffValueIntDto{Old: changes.Temperature.Old, New: changes.Temperature.New}
	}
	if changes.Oxygen != nil {
		dto.Oxygen = &DiffValueIntDto{Old: changes.Oxygen.Old, New: changes.Oxygen.New}
	}
	if changes.Oceans != nil {
		dto.Oceans = &DiffValueIntDto{Old: changes.Oceans.Old, New: changes.Oceans.New}
	}

	if len(changes.PlayerChanges) > 0 {
		dto.PlayerChanges = make(map[string]*PlayerChangesDto)
		for playerID, pc := range changes.PlayerChanges {
			dto.PlayerChanges[playerID] = toPlayerChangesDto(pc)
		}
	}

	if changes.BoardChanges != nil {
		dto.BoardChanges = toBoardChangesDto(changes.BoardChanges)
	}

	return dto
}

func toPlayerChangesDto(changes *game.PlayerChanges) *PlayerChangesDto {
	if changes == nil {
		return nil
	}

	dto := &PlayerChangesDto{}

	if changes.Credits != nil {
		dto.Credits = &DiffValueIntDto{Old: changes.Credits.Old, New: changes.Credits.New}
	}
	if changes.Steel != nil {
		dto.Steel = &DiffValueIntDto{Old: changes.Steel.Old, New: changes.Steel.New}
	}
	if changes.Titanium != nil {
		dto.Titanium = &DiffValueIntDto{Old: changes.Titanium.Old, New: changes.Titanium.New}
	}
	if changes.Plants != nil {
		dto.Plants = &DiffValueIntDto{Old: changes.Plants.Old, New: changes.Plants.New}
	}
	if changes.Energy != nil {
		dto.Energy = &DiffValueIntDto{Old: changes.Energy.Old, New: changes.Energy.New}
	}
	if changes.Heat != nil {
		dto.Heat = &DiffValueIntDto{Old: changes.Heat.Old, New: changes.Heat.New}
	}
	if changes.TerraformRating != nil {
		dto.TerraformRating = &DiffValueIntDto{Old: changes.TerraformRating.Old, New: changes.TerraformRating.New}
	}
	if changes.CreditsProduction != nil {
		dto.CreditsProduction = &DiffValueIntDto{Old: changes.CreditsProduction.Old, New: changes.CreditsProduction.New}
	}
	if changes.SteelProduction != nil {
		dto.SteelProduction = &DiffValueIntDto{Old: changes.SteelProduction.Old, New: changes.SteelProduction.New}
	}
	if changes.TitaniumProduction != nil {
		dto.TitaniumProduction = &DiffValueIntDto{Old: changes.TitaniumProduction.Old, New: changes.TitaniumProduction.New}
	}
	if changes.PlantsProduction != nil {
		dto.PlantsProduction = &DiffValueIntDto{Old: changes.PlantsProduction.Old, New: changes.PlantsProduction.New}
	}
	if changes.EnergyProduction != nil {
		dto.EnergyProduction = &DiffValueIntDto{Old: changes.EnergyProduction.Old, New: changes.EnergyProduction.New}
	}
	if changes.HeatProduction != nil {
		dto.HeatProduction = &DiffValueIntDto{Old: changes.HeatProduction.Old, New: changes.HeatProduction.New}
	}

	if len(changes.CardsAdded) > 0 {
		dto.CardsAdded = changes.CardsAdded
	}
	if len(changes.CardsRemoved) > 0 {
		dto.CardsRemoved = changes.CardsRemoved
	}
	if len(changes.CardsPlayed) > 0 {
		dto.CardsPlayed = changes.CardsPlayed
	}

	if changes.Corporation != nil {
		dto.Corporation = &DiffValueStringDto{Old: changes.Corporation.Old, New: changes.Corporation.New}
	}
	if changes.Passed != nil {
		dto.Passed = &DiffValueBoolDto{Old: changes.Passed.Old, New: changes.Passed.New}
	}

	return dto
}

func toBoardChangesDto(changes *game.BoardChanges) *BoardChangesDto {
	if changes == nil || len(changes.TilesPlaced) == 0 {
		return nil
	}

	placements := make([]TilePlacementDto, len(changes.TilesPlaced))
	for i, tp := range changes.TilesPlaced {
		placements[i] = TilePlacementDto{
			HexID:    tp.HexID,
			TileType: tp.TileType,
			OwnerID:  tp.OwnerID,
		}
	}

	return &BoardChangesDto{TilesPlaced: placements}
}

func toLogDisplayDataDto(data *game.LogDisplayData) *LogDisplayDataDto {
	if data == nil {
		return nil
	}

	return &LogDisplayDataDto{
		RevealedCards: mapSlice(data.RevealedCards, toRevealedCardDto),
		Behaviors:     mapSlice(data.Behaviors, toCardBehaviorDto),
		Tags:          mapSlice(data.Tags, func(t shared.CardTag) CardTag { return CardTag(t) }),
		VPConditions:  mapSlice(data.VPConditions, toVPConditionForLogDto),
	}
}

func toVPConditionForLogDto(vp shared.VPConditionForLog) VPConditionDto {
	return VPConditionDto{
		Amount:     vp.Amount,
		Condition:  VPConditionType(vp.Condition),
		MaxTrigger: vp.MaxTrigger,
		Per:        ptrCast(vp.Per, toPerConditionDto),
	}
}
