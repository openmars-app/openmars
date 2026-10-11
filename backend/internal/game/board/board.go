package board

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"openmars/internal/events"
	"openmars/internal/game/shared"
)

// Tile type string constants for placement operations
const (
	TileTypeCommercialDistrict = "commercial-district"
	TileTypeIndustrialCenter   = "industrial-center"
	TileTypeCity               = "city"
	TileTypeGreenery           = "greenery"
	TileTypeOcean              = "ocean"
	TileTypeNaturalPreserve    = "natural-preserve"
	TileTypeMining             = "mining"
	TileTypeNuclearZone        = "nuclear-zone"
	TileTypeEcologicalZone     = "ecological-zone"
	TileTypeMohole             = "mohole"
	TileTypeRestricted         = "restricted"
	TileTypeVolcano            = "volcano"
	TileTypeColony             = "colony"
	TileTypeLandClaim          = "land-claim"
	TileTypeClear              = "clear"
	TileTypeWorldTree          = "world-tree"
)

// PlaceableTileType describes a tile type available in the demo tile picker
type PlaceableTileType struct {
	Type  string
	Label string
	Group string
}

// PlaceableTileTypes is the single registry of all tile types available for placement.
// Adding a new entry here automatically updates backend validation and frontend UI.
var PlaceableTileTypes = []PlaceableTileType{
	{Type: TileTypeCommercialDistrict, Label: "Commercial District", Group: "Special"},
	{Type: TileTypeIndustrialCenter, Label: "Industrial Center", Group: "Industrial"},
	{Type: TileTypeCity, Label: "City", Group: "Base"},
	{Type: TileTypeGreenery, Label: "Greenery", Group: "Base"},
	{Type: TileTypeOcean, Label: "Ocean", Group: "Base"},
	{Type: TileTypeNaturalPreserve, Label: "Natural Preserve", Group: "Special"},
	{Type: TileTypeEcologicalZone, Label: "Ecological Zone", Group: "Special"},
	{Type: TileTypeMining, Label: "Mining", Group: "Special"},
	{Type: TileTypeColony, Label: "Colony", Group: "Special"},
	{Type: TileTypeVolcano, Label: "Volcano", Group: "Industrial"},
	{Type: TileTypeNuclearZone, Label: "Nuclear Zone", Group: "Industrial"},
	{Type: TileTypeMohole, Label: "Mohole", Group: "Industrial"},
	{Type: TileTypeRestricted, Label: "Restricted", Group: "Industrial"},
	{Type: TileTypeWorldTree, Label: "World Tree", Group: "Special"},
	{Type: TileTypeLandClaim, Label: "Land Claim", Group: "Tools"},
	{Type: TileTypeClear, Label: "Clear", Group: "Tools"},
}

// ValidPlaceableTileType returns true if the given tile type is in the PlaceableTileTypes registry
func ValidPlaceableTileType(tileType string) bool {
	for _, pt := range PlaceableTileTypes {
		if pt.Type == tileType {
			return true
		}
	}
	return false
}

// BoardTag represents a tag on a board tile for reserved areas
type BoardTag = string

const (
	BoardTagNoctisCity       BoardTag = "noctis-city"
	BoardTagGanymedeColony   BoardTag = "ganymede-colony"
	BoardTagVolcanic         BoardTag = "volcanic"
	BoardTagPhobosSpaceHaven BoardTag = "phobos-space-haven"
	BoardTagDawnCity         BoardTag = "dawn-city"
	BoardTagMaxwellBase      BoardTag = "maxwell-base"
	BoardTagStratopolis      BoardTag = "stratopolis"
	BoardTagLunaMetropolis   BoardTag = "luna-metropolis"
)

// TileLocation represents the celestial body where tiles are located
type TileLocation string

const (
	TileLocationMars     TileLocation = "mars"
	TileLocationVenus    TileLocation = "venus"
	TileLocationJupiter  TileLocation = "jupiter"
	TileLocationGanymede TileLocation = "ganymede"
	TileLocationEarth    TileLocation = "earth"
	TileLocationLuna     TileLocation = "luna"
	TileLocationMercury  TileLocation = "mercury"
	TileLocationPhobos   TileLocation = "phobos"
)

// TileBonus represents a resource bonus provided by a tile when occupied
type TileBonus struct {
	Type   shared.ResourceType `json:"type"`
	Amount int                 `json:"amount"`
}

// TileOccupant represents what currently occupies a tile
type TileOccupant struct {
	DisplayName string              `json:"displayName,omitempty"`
	Visual      *shared.TileVisual  `json:"visual,omitempty"`
	Type        shared.ResourceType `json:"type"`
	Tags        []string            `json:"tags"`
}

// Tile represents a single hexagonal tile on the game board
type Tile struct {
	Coordinates shared.HexPosition  `json:"coordinates"`
	Tags        []string            `json:"tags"`
	Type        shared.ResourceType `json:"type"`
	Location    TileLocation        `json:"location"`
	DisplayName *string             `json:"displayName,omitempty"`
	Bonuses     []TileBonus         `json:"bonuses"`
	OccupiedBy  *TileOccupant       `json:"occupiedBy,omitempty"`
	OwnerID     *string             `json:"ownerId,omitempty"`
	ReservedBy  *string             `json:"reservedBy,omitempty" ts:"reservedBy"`
}

// TilesPtr is a pointer to a tile slice, used by Board as its backing store.
// This allows Board to be a view into GameState's Tiles field.
type TilesPtr = *[]Tile

// Board represents the complete game board state.
type Board struct {
	tiles    TilesPtr
	gameID   string
	eventBus *events.EventBusImpl
}

// NewBoard creates a new Board view backed by the given tiles pointer.
func NewBoard(tiles TilesPtr, gameID string, eventBus *events.EventBusImpl) *Board {
	return &Board{
		tiles:    tiles,
		gameID:   gameID,
		eventBus: eventBus,
	}
}

// NewBoardWithTiles creates a new Board view and initializes the tiles.
func NewBoardWithTiles(tiles TilesPtr, gameID string, initialTiles []Tile, eventBus *events.EventBusImpl) *Board {
	tilesCopy := make([]Tile, len(initialTiles))
	copy(tilesCopy, initialTiles)
	*tiles = tilesCopy
	return &Board{
		tiles:    tiles,
		gameID:   gameID,
		eventBus: eventBus,
	}
}

// GenerateMarsBoard creates the standard Mars board layout
// Returns a hexagonal grid with ocean spaces, bonus tiles, and land tiles.
// When includeVenus is true, Venus tiles are also included.
func GenerateMarsBoard(includeVenus bool) []Tile {
	tiles := []Tile{}

	// Official Tharsis map ocean-reserved spaces (12 total, 9 ocean tiles placed during game)
	oceanSpaces := map[shared.HexPosition]bool{
		// Row 0 (top)
		{Q: 1, R: -4, S: 3}: true,
		{Q: 3, R: -4, S: 1}: true,
		{Q: 4, R: -4, S: 0}: true,
		// Row 1
		{Q: 4, R: -3, S: -1}: true,
		// Row 3
		{Q: 4, R: -1, S: -3}: true,
		// Row 4 (middle)
		{Q: -1, R: 0, S: 1}: true,
		{Q: 0, R: 0, S: 0}:  true,
		{Q: 1, R: 0, S: -1}: true,
		// Row 5
		{Q: 1, R: 1, S: -2}: true,
		{Q: 2, R: 1, S: -3}: true,
		{Q: 3, R: 1, S: -4}: true,
		// Row 8 (bottom)
		{Q: 0, R: 4, S: -4}: true,
	}

	// Official Tharsis map placement bonuses
	bonusTiles := map[shared.HexPosition][]TileBonus{
		// Row 0: steel in upper-left
		{Q: 0, R: -4, S: 4}: {{Type: shared.ResourceSteel, Amount: 2}},
		{Q: 1, R: -4, S: 3}: {{Type: shared.ResourceSteel, Amount: 2}},
		{Q: 3, R: -4, S: 1}: {{Type: shared.ResourceCardDraw, Amount: 1}},
		// Row 1: Tharsis Tholus steel, rightmost ocean has 2 card draws
		{Q: 0, R: -3, S: 3}:  {{Type: shared.ResourceSteel, Amount: 1}},
		{Q: 4, R: -3, S: -1}: {{Type: shared.ResourceCardDraw, Amount: 2}},
		// Row 2: Ascraeus Mons card draw, rightmost steel
		{Q: -2, R: -2, S: 4}: {{Type: shared.ResourceCardDraw, Amount: 1}},
		{Q: 4, R: -2, S: -2}: {{Type: shared.ResourceSteel, Amount: 1}},
		// Row 3: Pavonis Mons plant+titanium, plant bonuses across equator
		{Q: -3, R: -1, S: 4}: {{Type: shared.ResourcePlant, Amount: 1}, {Type: shared.ResourceTitanium, Amount: 1}},
		{Q: -2, R: -1, S: 3}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: -1, R: -1, S: 2}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 0, R: -1, S: 1}:  {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 1, R: -1, S: 0}:  {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: 2, R: -1, S: -1}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 3, R: -1, S: -2}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 4, R: -1, S: -3}: {{Type: shared.ResourcePlant, Amount: 2}},
		// Row 4: all tiles have 2 plants (equatorial belt)
		{Q: -4, R: 0, S: 4}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: -3, R: 0, S: 3}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: -2, R: 0, S: 2}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: -1, R: 0, S: 1}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: 0, R: 0, S: 0}:  {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: 1, R: 0, S: -1}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: 2, R: 0, S: -2}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: 3, R: 0, S: -3}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: 4, R: 0, S: -4}: {{Type: shared.ResourcePlant, Amount: 2}},
		// Row 5: plant bonuses
		{Q: -4, R: 1, S: 3}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: -3, R: 1, S: 2}: {{Type: shared.ResourcePlant, Amount: 2}},
		{Q: -2, R: 1, S: 1}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: -1, R: 1, S: 0}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 0, R: 1, S: -1}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 1, R: 1, S: -2}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 2, R: 1, S: -3}: {{Type: shared.ResourcePlant, Amount: 1}},
		{Q: 3, R: 1, S: -4}: {{Type: shared.ResourcePlant, Amount: 1}},
		// Row 6: single plant on one tile
		{Q: 1, R: 2, S: -3}: {{Type: shared.ResourcePlant, Amount: 1}},
		// Row 7: steel, card draws, titanium
		{Q: -4, R: 3, S: 1}:  {{Type: shared.ResourceSteel, Amount: 2}},
		{Q: -2, R: 3, S: -1}: {{Type: shared.ResourceCardDraw, Amount: 1}},
		{Q: -1, R: 3, S: -2}: {{Type: shared.ResourceCardDraw, Amount: 1}},
		{Q: 1, R: 3, S: -4}:  {{Type: shared.ResourceTitanium, Amount: 1}},
		// Row 8: steel and titanium
		{Q: -4, R: 4, S: 0}:  {{Type: shared.ResourceSteel, Amount: 1}},
		{Q: -3, R: 4, S: -1}: {{Type: shared.ResourceSteel, Amount: 2}},
		{Q: 0, R: 4, S: -4}:  {{Type: shared.ResourceTitanium, Amount: 2}},
	}

	type taggedTileInfo struct {
		Tags        []string
		DisplayName string
	}
	taggedTiles := map[shared.HexPosition]taggedTileInfo{
		{Q: 0, R: -3, S: 3}:  {Tags: []string{BoardTagVolcanic}, DisplayName: "Tharsis Tholus"},
		{Q: -2, R: -2, S: 4}: {Tags: []string{BoardTagVolcanic}, DisplayName: "Ascraeus Mons"},
		{Q: -3, R: -1, S: 4}: {Tags: []string{BoardTagVolcanic}, DisplayName: "Pavonis Mons"},
		{Q: -4, R: 0, S: 4}:  {Tags: []string{BoardTagVolcanic}, DisplayName: "Arsia Mons"},
		{Q: -2, R: 0, S: 2}:  {Tags: []string{BoardTagNoctisCity}, DisplayName: "Noctis City"},
	}

	radius := 4
	for q := -radius; q <= radius; q++ {
		r1 := max(-radius, -q-radius)
		r2 := min(radius, -q+radius)

		for r := r1; r <= r2; r++ {
			s := -q - r
			pos := shared.HexPosition{Q: q, R: r, S: s}

			var tileType shared.ResourceType
			var bonuses []TileBonus

			if oceanSpaces[pos] {
				tileType = shared.ResourceOceanSpace
			} else {
				tileType = shared.ResourceLandTile
			}

			if tileBonuses, hasBonus := bonusTiles[pos]; hasBonus {
				bonuses = append(bonuses, tileBonuses...)
			}

			var tags []string
			var displayName *string
			if tagInfo, hasTag := taggedTiles[pos]; hasTag {
				tags = tagInfo.Tags
				displayName = &tagInfo.DisplayName
			} else {
				tags = []string{}
			}

			tile := Tile{
				Coordinates: pos,
				Type:        tileType,
				Location:    TileLocationMars,
				Tags:        tags,
				DisplayName: displayName,
				Bonuses:     bonuses,
				OccupiedBy:  nil,
				OwnerID:     nil,
			}

			tiles = append(tiles, tile)
		}
	}

	// Add celestial body tiles (planets and moons outside Mars)
	celestialTiles := []struct {
		Pos         shared.HexPosition
		Tags        []string
		DisplayName string
		Location    TileLocation
	}{
		{shared.HexPosition{Q: 500, R: 0, S: -500}, []string{BoardTagPhobosSpaceHaven}, "Phobos Space Haven", TileLocationPhobos},
		{shared.HexPosition{Q: 400, R: 0, S: -400}, []string{BoardTagDawnCity}, "Dawn City", TileLocationMercury},
		{shared.HexPosition{Q: 200, R: 0, S: -200}, []string{BoardTagGanymedeColony}, "Ganymede Colony", TileLocationGanymede},
		{shared.HexPosition{Q: 300, R: 0, S: -300}, []string{BoardTagLunaMetropolis}, "Luna Metropolis", TileLocationLuna},
	}
	for _, ct := range celestialTiles {
		displayName := ct.DisplayName
		tiles = append(tiles, Tile{
			Coordinates: ct.Pos,
			Type:        shared.ResourceLandTile,
			Location:    ct.Location,
			Tags:        ct.Tags,
			DisplayName: &displayName,
			Bonuses:     nil,
			OccupiedBy:  nil,
			OwnerID:     nil,
		})
	}

	if !includeVenus {
		return tiles
	}

	// Venus tiles (non-adjacent coordinates so cities can't neighbor each other)
	venusTiles := []struct {
		Pos         shared.HexPosition
		Tags        []string
		DisplayName string
	}{
		{shared.HexPosition{Q: 100, R: 0, S: -100}, []string{BoardTagMaxwellBase}, "Maxwell Base"},
		{shared.HexPosition{Q: 102, R: 0, S: -102}, []string{BoardTagStratopolis}, "Stratopolis"},
	}
	for _, vt := range venusTiles {
		displayName := vt.DisplayName
		tiles = append(tiles, Tile{
			Coordinates: vt.Pos,
			Type:        shared.ResourceLandTile,
			Location:    TileLocationVenus,
			Tags:        vt.Tags,
			DisplayName: &displayName,
			Bonuses:     nil,
			OccupiedBy:  nil,
			OwnerID:     nil,
		})
	}

	return tiles
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FreeOceanSpaces returns the count of unoccupied ocean-space tiles on the board
func (b *Board) FreeOceanSpaces() int {
	count := 0
	for _, tile := range *b.tiles {
		if tile.Type == shared.ResourceOceanSpace && tile.OccupiedBy == nil {
			count++
		}
	}
	return count
}

// Tiles returns a deep copy of all tiles to prevent external mutation
func (b *Board) Tiles() []Tile {
	return b.deepCopyTiles()
}

// GetTile returns a copy of a specific tile by coordinates
func (b *Board) GetTile(coords shared.HexPosition) (*Tile, error) {
	for i := range *b.tiles {
		if (*b.tiles)[i].Coordinates == coords {
			tileCopy := b.deepCopyTile(&(*b.tiles)[i])
			return tileCopy, nil
		}
	}
	return nil, fmt.Errorf("tile not found at coordinates %v", coords)
}

// SetTiles replaces all tiles (used for board generation)
func (b *Board) SetTiles(ctx context.Context, tiles []Tile) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	newTiles := make([]Tile, len(tiles))
	copy(newTiles, tiles)
	*b.tiles = newTiles
	return nil
}

// UpdateTileOccupancy updates a tile's occupancy state and publishes TilePlacedEvent
func (b *Board) UpdateTileOccupancy(ctx context.Context, coords shared.HexPosition, occupant TileOccupant, ownerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var found bool
	for i := range *b.tiles {
		if (*b.tiles)[i].Coordinates == coords {
			if occupant.Visual != nil || occupant.Type == shared.ResourceCityTile || occupant.Type == shared.ResourceGreeneryTile {
				occupant.Visual = occupant.Visual.Clone()
				if occupant.Visual == nil {
					occupant.Visual = &shared.TileVisual{}
				}
				source := "standard"
				for _, tag := range occupant.Tags {
					if strings.HasPrefix(tag, "source:") {
						source = tag
						break
					}
				}
				hash := fnv.New32a()
				_, _ = fmt.Fprintf(hash, "landscape-v1|%s|%s|%s|%s|%s", b.gameID, (*b.tiles)[i].Location, coords.String(), occupant.Type, source)
				occupant.Visual.Seed = hash.Sum32()
			}
			(*b.tiles)[i].OccupiedBy = &occupant
			(*b.tiles)[i].OwnerID = &ownerID
			(*b.tiles)[i].ReservedBy = nil
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("tile not found at coordinates %v", coords)
	}

	if b.eventBus != nil {
		events.Publish(b.eventBus, events.TilePlacedEvent{
			GameID:   b.gameID,
			PlayerID: ownerID,
			TileType: string(occupant.Type),
			Q:        coords.Q,
			R:        coords.R,
			S:        coords.S,
		})
	}

	return nil
}

// ClearTileOccupant removes the occupant and owner from a tile (admin debug tool)
func (b *Board) ClearTileOccupant(ctx context.Context, coords shared.HexPosition) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var found bool
	for i := range *b.tiles {
		if (*b.tiles)[i].Coordinates == coords {
			(*b.tiles)[i].OccupiedBy = nil
			(*b.tiles)[i].OwnerID = nil
			(*b.tiles)[i].ReservedBy = nil
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("tile not found at coordinates %v", coords)
	}

	if b.eventBus != nil {
		events.Publish(b.eventBus, events.GameStateChangedEvent{
			GameID:    b.gameID,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// ClearTileBonuses removes all bonuses from a tile after they have been claimed
func (b *Board) ClearTileBonuses(ctx context.Context, coords shared.HexPosition) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var found bool
	for i := range *b.tiles {
		if (*b.tiles)[i].Coordinates == coords {
			(*b.tiles)[i].Bonuses = nil
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("tile not found at coordinates %v", coords)
	}

	return nil
}

// ReserveTile reserves a tile for exclusive future placement by a player
func (b *Board) ReserveTile(ctx context.Context, coords shared.HexPosition, playerID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	for i := range *b.tiles {
		if (*b.tiles)[i].Coordinates == coords {
			if (*b.tiles)[i].OccupiedBy != nil {
				return fmt.Errorf("cannot reserve tile at %v: already occupied", coords)
			}
			if (*b.tiles)[i].ReservedBy != nil {
				return fmt.Errorf("cannot reserve tile at %v: already reserved by another player", coords)
			}
			(*b.tiles)[i].ReservedBy = &playerID

			if b.eventBus != nil {
				events.Publish(b.eventBus, events.GameStateChangedEvent{
					GameID:    b.gameID,
					Timestamp: time.Now(),
				})
			}
			return nil
		}
	}

	return fmt.Errorf("tile not found at coordinates %v", coords)
}

// deepCopyTiles creates a deep copy of all tiles
func (b *Board) deepCopyTiles() []Tile {
	tiles := make([]Tile, len(*b.tiles))
	for i := range *b.tiles {
		tiles[i] = *b.deepCopyTile(&(*b.tiles)[i])
	}
	return tiles
}

// deepCopyTile creates a deep copy of a single tile
func (b *Board) deepCopyTile(tile *Tile) *Tile {
	tileCopy := *tile

	tileCopy.Tags = make([]string, len(tile.Tags))
	copy(tileCopy.Tags, tile.Tags)

	tileCopy.Bonuses = make([]TileBonus, len(tile.Bonuses))
	copy(tileCopy.Bonuses, tile.Bonuses)

	if tile.DisplayName != nil {
		displayNameCopy := *tile.DisplayName
		tileCopy.DisplayName = &displayNameCopy
	}

	if tile.OccupiedBy != nil {
		occupantCopy := *tile.OccupiedBy
		occupantCopy.Visual = tile.OccupiedBy.Visual.Clone()
		occupantCopy.Tags = make([]string, len(tile.OccupiedBy.Tags))
		copy(occupantCopy.Tags, tile.OccupiedBy.Tags)
		tileCopy.OccupiedBy = &occupantCopy
	}

	if tile.OwnerID != nil {
		ownerIDCopy := *tile.OwnerID
		tileCopy.OwnerID = &ownerIDCopy
	}

	if tile.ReservedBy != nil {
		reservedByCopy := *tile.ReservedBy
		tileCopy.ReservedBy = &reservedByCopy
	}

	return &tileCopy
}
