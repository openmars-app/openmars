package board

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"openmars/internal/game/shared"
)

// MapDefinition represents a map loaded from JSON
type MapDefinition struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Rows        [][]TileDefJSON `json:"rows"`
}

// TileDefJSON represents a single tile in the JSON map definition
type TileDefJSON struct {
	Type        string          `json:"type"`
	Bonuses     []TileBonusJSON `json:"bonuses,omitempty"`
	Volcanic    bool            `json:"volcanic,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	DisplayName string          `json:"displayName,omitempty"`
}

// TileBonusJSON represents a tile bonus in JSON
type TileBonusJSON struct {
	Type   string `json:"type"`
	Amount int    `json:"amount"`
}

// MapRegistry holds all loaded map definitions
type MapRegistry struct {
	maps    map[string]*MapDefinition
	mapList []*MapDefinition
}

// NewMapRegistry creates a new empty map registry
func NewMapRegistry() *MapRegistry {
	return &MapRegistry{
		maps:    make(map[string]*MapDefinition),
		mapList: make([]*MapDefinition, 0),
	}
}

// LoadMapsFromJSON loads map definitions from a JSON file
func LoadMapsFromJSON(filepath string) (*MapRegistry, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read maps file: %w", err)
	}

	var defs []MapDefinition
	if err := json.Unmarshal(data, &defs); err != nil {
		return nil, fmt.Errorf("failed to parse maps JSON: %w", err)
	}

	if len(defs) == 0 {
		return nil, fmt.Errorf("no maps found in file: %s", filepath)
	}

	registry := NewMapRegistry()
	for i := range defs {
		registry.maps[defs[i].ID] = &defs[i]
		registry.mapList = append(registry.mapList, &defs[i])
	}

	return registry, nil
}

// RegisterMap adds a map definition to the registry
func (r *MapRegistry) RegisterMap(def *MapDefinition) {
	r.maps[def.ID] = def
	r.mapList = append(r.mapList, def)
}

// GetMap returns a map definition by ID
func (r *MapRegistry) GetMap(id string) (*MapDefinition, bool) {
	m, ok := r.maps[id]
	return m, ok
}

// ListMaps returns all available map IDs and names
func (r *MapRegistry) ListMaps() []MapInfo {
	infos := make([]MapInfo, len(r.mapList))
	for i, m := range r.mapList {
		infos[i] = MapInfo{ID: m.ID, Name: m.Name, Description: m.Description}
	}
	return infos
}

// MapInfo contains basic map identification
type MapInfo struct {
	ID          string
	Name        string
	Description string
}

// DefaultMapID returns the default map ID
func DefaultMapID() string {
	return "tharsis"
}

// GenerateBoardFromMap converts a map definition to board tiles with cube coordinates.
// The row pattern is [5,6,7,8,9,8,7,6,5] matching the standard Mars hex grid.
func GenerateBoardFromMap(mapDef *MapDefinition, includeVenus bool) []Tile {
	tiles := []Tile{}
	radius := 4

	for rowIdx, row := range mapDef.Rows {
		r := rowIdx - radius
		qMin := max(-radius, -r-radius)

		for colIdx, tileDef := range row {
			q := qMin + colIdx
			s := -q - r
			pos := shared.HexPosition{Q: q, R: r, S: s}

			tileType := mapTileType(tileDef.Type)
			bonuses := mapBonuses(tileDef.Bonuses)

			tags := tileDef.Tags
			if tags == nil {
				tags = []string{}
			}
			if tileDef.Volcanic {
				tags = appendIfMissing(tags, BoardTagVolcanic)
			}

			var displayName *string
			if tileDef.DisplayName != "" {
				name := tileDef.DisplayName
				displayName = &name
			}

			tiles = append(tiles, Tile{
				Coordinates: pos,
				Type:        tileType,
				Location:    TileLocationMars,
				Tags:        tags,
				DisplayName: displayName,
				Bonuses:     bonuses,
			})
		}
	}

	return appendReservedCelestialTiles(tiles, includeVenus)
}

func mapTileType(t string) shared.ResourceType {
	switch t {
	case "ocean":
		return shared.ResourceOceanSpace
	case "cove":
		return shared.ResourceOceanSpace
	case "empty":
		return shared.ResourceType("empty")
	case "deflection-zone":
		return shared.ResourceLandTile
	default:
		return shared.ResourceLandTile
	}
}

func mapBonuses(bonuses []TileBonusJSON) []TileBonus {
	if len(bonuses) == 0 {
		return nil
	}
	result := make([]TileBonus, len(bonuses))
	for i, b := range bonuses {
		result[i] = TileBonus{
			Type:   shared.ResourceType(b.Type),
			Amount: b.Amount,
		}
	}
	return result
}

func appendIfMissing(slice []string, val string) []string {
	if slices.Contains(slice, val) {
		return slice
	}
	return append(slice, val)
}
