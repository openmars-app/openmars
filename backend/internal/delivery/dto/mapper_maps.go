package dto

import (
	gameaction "openmars/internal/action/game"
	"openmars/internal/game/board"
	"openmars/internal/game/shared"
)

// MapPreviews projects the registered boards for setup and lobby selection.
func MapPreviews(mapRegistry *board.MapRegistry) []MapInfoDto {
	availableMaps := make([]MapInfoDto, 0)
	for _, m := range mapRegistry.ListMaps() {
		mapDef, _ := mapRegistry.GetMap(m.ID)
		availableMaps = append(availableMaps, mapPreview(mapDef))
	}
	return availableMaps
}

func mapPreview(mapDef *board.MapDefinition) MapInfoDto {
	tiles := board.GenerateBoardFromMap(mapDef, false)
	previewTiles := make([]MapPreviewTile, 0)
	for _, t := range tiles {
		if t.Location != "mars" || string(t.Type) == "empty" {
			continue
		}
		volcanic := false
		for _, tag := range t.Tags {
			if tag == "volcanic" {
				volcanic = true
				break
			}
		}
		previewTiles = append(previewTiles, MapPreviewTile{
			Q:        t.Coordinates.Q,
			R:        t.Coordinates.R,
			S:        t.Coordinates.S,
			Type:     string(t.Type),
			Volcanic: volcanic,
			Tags:     t.Tags,
		})
	}
	return MapInfoDto{ID: mapDef.ID, Name: mapDef.Name, Description: mapDef.Description, Tiles: previewTiles}
}

// MapGameOptions converts domain setup options to the frontend contract.
func MapGameOptions(options gameaction.GameOptions) GameOptionsDto {
	settings := options.Defaults
	previews := make([]MapInfoDto, 0, len(options.AvailableMaps))
	for _, definition := range options.AvailableMaps {
		previews = append(previews, mapPreview(definition))
	}
	return GameOptionsDto{Defaults: GameSetupDto{
		MaxPlayers: settings.MaxPlayers, MapID: settings.MapID, GameModeID: settings.GameModeID, GameModeParameters: settings.GameModeParameters, Features: settings.Features,
		DevelopmentMode: settings.DevelopmentMode,
		DemoGame:        settings.DemoGame, AllowRandomBuy: settings.AllowRandomBuy,
	}, AvailableMaps: previews, GameModes: MapGameModes(options.GameModes)}
}

// ToSettings maps an optional setup request into domain settings.
func (setup *GameSetupDto) ToSettings() *shared.GameSettings {
	if setup == nil {
		return nil
	}
	return &shared.GameSettings{
		MaxPlayers: setup.MaxPlayers, MapID: setup.MapID, GameModeID: setup.GameModeID, GameModeParameters: setup.GameModeParameters, Features: setup.Features,
		DevelopmentMode: setup.DevelopmentMode,
		DemoGame:        setup.DemoGame, AllowRandomBuy: setup.AllowRandomBuy,
	}
}
