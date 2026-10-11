package shared

// StructureStyleRequest contains optional visual requests, independent of placement rules.
type StructureStyleRequest struct {
	Plan          string   `json:"plan,omitempty"`
	Density       string   `json:"density,omitempty"`
	Heights       string   `json:"heights,omitempty"`
	Ground        string   `json:"ground,omitempty"`
	Exposure      string   `json:"exposure,omitempty"`
	Perimeter     string   `json:"perimeter,omitempty"`
	Cover         string   `json:"cover,omitempty"`
	Landmark      string   `json:"landmark,omitempty"`
	Connections   string   `json:"connections,omitempty"`
	Landscaping   string   `json:"landscaping,omitempty"`
	Lighting      string   `json:"lighting,omitempty"`
	PerimeterRoad string   `json:"perimeterRoad,omitempty"`
	EntranceMin   int      `json:"entranceMin,omitempty"`
	EntranceMax   int      `json:"entranceMax,omitempty"`
	Details       []string `json:"details,omitempty"`
}

// CardStyle contains best-effort requests for the placed tile.
type CardStyle struct {
	Tile *StructureStyleRequest `json:"tile,omitempty"`
}

// TileVisual freezes placement-time visual inputs; meshes are generated on clients.
type TileVisual struct {
	Seed      uint32                 `json:"seed"`
	Structure *StructureStyleRequest `json:"structure,omitempty"`
}

// Clone copies a visual request, including mutable detail lists.
func (s *StructureStyleRequest) Clone() *StructureStyleRequest {
	if s == nil {
		return nil
	}
	copy := *s
	copy.Details = append([]string(nil), s.Details...)
	return &copy
}

// Clone copies a card style.
func (s *CardStyle) Clone() *CardStyle {
	if s == nil {
		return nil
	}
	return &CardStyle{Tile: s.Tile.Clone()}
}

// Clone copies the visual specification independently of the source card.
func (v *TileVisual) Clone() *TileVisual {
	if v == nil {
		return nil
	}
	return &TileVisual{Seed: v.Seed, Structure: v.Structure.Clone()}
}
