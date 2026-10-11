package game

import (
	"openmars/internal/game/board"
	"openmars/internal/game/shared"
	"slices"
)

type VPCardInfo struct {
	CardID       string
	CardName     string
	CardType     string
	Description  string
	VPConditions []shared.VPCondition
	Tags         []shared.CardTag
}

type VPCardLookup interface {
	LookupVPCard(cardID string) (*VPCardInfo, error)
}

type gameVPRecalculationContext struct {
	game *Game
}

func (ctx *gameVPRecalculationContext) GetCardStorage(playerID string, cardID string) int {
	p, err := ctx.game.GetPlayer(playerID)
	if err != nil {
		return 0
	}
	return p.Resources().GetCardStorage(cardID)
}

func (ctx *gameVPRecalculationContext) CountPlayerTagsByType(playerID string, tagType shared.CardTag) int {
	p, err := ctx.game.GetPlayer(playerID)
	if err != nil {
		return 0
	}
	count := 0
	if ctx.game.vpCardLookup == nil {
		return 0
	}
	ids := append([]string(nil), p.PlayedCards().Cards()...)
	if id := p.CorporationID(); id != "" && !slices.Contains(ids, id) {
		ids = append(ids, id)
	}
	for _, cardID := range ids {
		cardInfo, err := ctx.game.vpCardLookup.LookupVPCard(cardID)
		if err != nil {
			continue
		}
		if cardInfo.CardType == "event" && tagType != shared.TagEvent {
			continue
		}
		for _, tag := range cardInfo.Tags {
			if tag == tagType {
				count++
			}
		}
	}
	return count + p.BonusTagCount(tagType)
}

func (ctx *gameVPRecalculationContext) CountAllTilesOfType(tileType shared.ResourceType) int {
	tiles := ctx.game.board.Tiles()
	count := 0
	for _, tile := range tiles {
		if tile.OccupiedBy != nil && tile.OccupiedBy.Type == tileType {
			count++
		}
	}
	return count
}

func (ctx *gameVPRecalculationContext) CountPlayerTilesOfType(playerID string, tileType shared.ResourceType) int {
	tiles := ctx.game.board.Tiles()
	count := 0
	for _, tile := range tiles {
		if tile.OccupiedBy != nil && tile.OccupiedBy.Type == tileType &&
			tile.OwnerID != nil && *tile.OwnerID == playerID {
			count++
		}
	}
	return count
}

func (ctx *gameVPRecalculationContext) CountAdjacentTilesForCard(cardID string, tileType shared.ResourceType) int {
	tiles := ctx.game.board.Tiles()
	sourceTag := "source:" + cardID

	var sourceTile *board.Tile
	for i := range tiles {
		if tiles[i].OccupiedBy == nil {
			continue
		}
		if slices.Contains(tiles[i].OccupiedBy.Tags, sourceTag) {
			sourceTile = &tiles[i]
			break
		}
	}

	if sourceTile == nil {
		return 0
	}

	neighbors := sourceTile.Coordinates.GetNeighbors()
	count := 0
	for _, tile := range tiles {
		if tile.OccupiedBy == nil || tile.OccupiedBy.Type != tileType {
			continue
		}
		if slices.Contains(neighbors, tile.Coordinates) {
			count++
		}
	}
	return count
}

func (ctx *gameVPRecalculationContext) CountAdjacentTilesToTileType(playerID string, countType, adjacentToType shared.ResourceType) int {
	tiles := ctx.game.board.Tiles()

	tilesByCoord := make(map[shared.HexPosition]*board.Tile, len(tiles))
	for i := range tiles {
		tilesByCoord[tiles[i].Coordinates] = &tiles[i]
	}

	counted := make(map[shared.HexPosition]bool)

	for _, tile := range tiles {
		if tile.OwnerID == nil || *tile.OwnerID != playerID {
			continue
		}
		if tile.OccupiedBy == nil || tile.OccupiedBy.Type != adjacentToType {
			continue
		}

		for _, neighborCoord := range tile.Coordinates.GetNeighbors() {
			neighborTile, exists := tilesByCoord[neighborCoord]
			if !exists || neighborTile.OccupiedBy == nil || counted[neighborCoord] {
				continue
			}

			var matches bool
			if countType == shared.ResourceGreeneryTile {
				matches = neighborTile.OccupiedBy.Type == shared.ResourceGreeneryTile || neighborTile.OccupiedBy.Type == shared.ResourceWorldTreeTile
			} else {
				matches = neighborTile.OccupiedBy.Type == countType
			}

			if matches {
				counted[neighborCoord] = true
			}
		}
	}

	return len(counted)
}

func (ctx *gameVPRecalculationContext) CountAllColonies() int {
	return ctx.game.Colonies().CountAllColonies()
}

func (ctx *gameVPRecalculationContext) ModeRequirementsMet(requirements []shared.StateRequirement) bool {
	return ctx.game.ModeRequirementsMet(requirements)
}
func (ctx *gameVPRecalculationContext) CountPermanentProductionTypes(playerID string) int {
	p, err := ctx.game.GetPlayer(playerID)
	if err != nil {
		return 0
	}
	return p.Resources().PermanentProduction().PositiveTypes()
}
