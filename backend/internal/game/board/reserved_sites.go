package board

import "openmars/internal/game/shared"

var reservedCelestialSites = []struct {
	Pos         shared.HexPosition
	Tags        []string
	DisplayName string
	Location    TileLocation
}{
	{shared.HexPosition{Q: 500, R: 0, S: -500}, []string{BoardTagPhobosSpaceHaven}, "Phobos Space Haven", TileLocationPhobos},
	{shared.HexPosition{Q: 400, R: 0, S: -400}, []string{BoardTagDawnCity}, "Dawn City", TileLocationMercury},
	{shared.HexPosition{Q: 200, R: 0, S: -200}, []string{BoardTagGanymedeColony}, "Ganymede Colony", TileLocationGanymede},
	{shared.HexPosition{Q: 300, R: 0, S: -300}, []string{BoardTagLunaMetropolis}, "Luna Metropolis", TileLocationLuna},
	{shared.HexPosition{Q: 100, R: 0, S: -100}, []string{BoardTagMaxwellBase}, "Maxwell Base", TileLocationVenus},
	{shared.HexPosition{Q: 102, R: 0, S: -102}, []string{BoardTagStratopolis}, "Stratopolis", TileLocationVenus},
}

func appendReservedCelestialTiles(tiles []Tile, includeVenus bool) []Tile {
	for _, site := range reservedCelestialSites {
		if site.Location == TileLocationVenus && !includeVenus {
			continue
		}
		name := site.DisplayName
		tiles = append(tiles, Tile{Coordinates: site.Pos, Type: shared.ResourceLandTile, Location: site.Location, Tags: site.Tags, DisplayName: &name})
	}
	return tiles
}

// ReservedPlacementLocation describes reserved sites using the same registry as the board.
func ReservedPlacementLocation(tags []string) string {
	for _, tag := range tags {
		if tag == BoardTagNoctisCity || tag == BoardTagVolcanic {
			return string(TileLocationMars)
		}
		for _, site := range reservedCelestialSites {
			if site.Tags[0] == tag {
				return string(site.Location)
			}
		}
	}
	return ""
}
