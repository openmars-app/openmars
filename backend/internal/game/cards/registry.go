package cards

import (
	"encoding/json"
	"fmt"
	"openmars/internal/game/mode"
	"sync"

	"openmars/internal/game"
	"openmars/internal/game/shared"
)

// CardRegistry provides lookup functionality for card data
type CardRegistry interface {
	// GetByID retrieves a card by its ID
	GetByID(cardID string) (*Card, error)

	// GetAll returns all cards in the registry
	GetAll() []Card
}

// InMemoryCardRegistry implements CardRegistry with an in-memory map
type InMemoryCardRegistry struct {
	cards     map[string]Card
	order     []string // preserves load (JSON) order so GetAll is deterministic
	modes     []mode.Definition
	cache     sync.Map
	effective bool
}

// NewInMemoryCardRegistry creates a new card registry from a slice of cards
func NewInMemoryCardRegistry(cardList []Card, modes ...mode.Definition) *InMemoryCardRegistry {
	cardMap := make(map[string]Card, len(cardList))
	order := make([]string, 0, len(cardList))
	for _, card := range cardList {
		if _, exists := cardMap[card.ID]; !exists {
			order = append(order, card.ID)
		}
		cardMap[card.ID] = card.DeepCopy()
	}

	return &InMemoryCardRegistry{
		cards: cardMap,
		order: order,
		modes: cloneJSON(modes),
	}
}

// GetByID retrieves a card by its ID, returning a copy to prevent mutation
func (r *InMemoryCardRegistry) GetByID(cardID string) (*Card, error) {
	card, exists := r.cards[cardID]
	if !exists {
		return nil, fmt.Errorf("card not found: %s", cardID)
	}

	// Return a deep copy to prevent external mutation
	cardCopy := card.DeepCopy()
	return &cardCopy, nil
}

// GetAll returns all cards in the registry, in their original load (JSON) order
// so that deck construction is deterministic for a given seed.
func (r *InMemoryCardRegistry) GetAll() []Card {
	cardList := make([]Card, 0, len(r.order))
	for _, id := range r.order {
		cardList = append(cardList, r.cards[id].DeepCopy())
	}
	return cardList
}

func (r *InMemoryCardRegistry) ModeDefinitions() []mode.Definition { return cloneJSON(r.modes) }
func (r *InMemoryCardRegistry) resolve(settings shared.GameSettings) (CardRegistry, error) {
	if r.effective {
		return r, nil
	}
	keyData, err := json.Marshal(struct {
		Mode              string
		Features, Systems []string
	}{settings.GameModeID, settings.Features, settings.Systems})
	if err != nil {
		return nil, err
	}
	key := string(keyData)
	if cached, ok := r.cache.Load(key); ok {
		return cached.(CardRegistry), nil
	}
	resolved, err := resolveUncached(r, settings)
	if err != nil {
		return nil, err
	}
	actual, _ := r.cache.LoadOrStore(key, resolved)
	return actual.(CardRegistry), nil
}

type VPCardLookupAdapter struct {
	registry CardRegistry
}

func NewVPCardLookupAdapter(registry CardRegistry) *VPCardLookupAdapter {
	return &VPCardLookupAdapter{registry: registry}
}

func (a *VPCardLookupAdapter) LookupVPCard(cardID string) (*game.VPCardInfo, error) {
	card, err := a.registry.GetByID(cardID)
	if err != nil {
		return nil, err
	}

	vpConditions := make([]shared.VPCondition, len(card.VPConditions))
	for i, vc := range card.VPConditions {
		vpConditions[i] = convertVPCondition(vc)
	}

	tags := make([]shared.CardTag, len(card.Tags))
	copy(tags, card.Tags)

	return &game.VPCardInfo{
		CardID:       card.ID,
		CardName:     card.Name,
		CardType:     string(card.Type),
		Description:  card.Description.PlainText(),
		VPConditions: vpConditions,
		Tags:         tags,
	}, nil
}

func convertVPCondition(vc VictoryPointCondition) shared.VPCondition {
	cond := shared.VPCondition{
		StateRequirements: append([]shared.StateRequirement(nil), vc.StateRequirements...),
		Amount:            vc.Amount,
		Condition:         string(vc.Condition),
		MaxTrigger:        vc.MaxTrigger,
		Per:               vc.Per,
	}
	return cond
}

// UniqueStructureCard resolves an unambiguous authored appearance for a special tile.
func UniqueStructureCard(registry CardRegistry, tileType string) *Card {
	var match *Card
	for _, card := range registry.GetAll() {
		if card.Style == nil || card.Style.Tile == nil {
			continue
		}
		places := func(outputs []shared.BehaviorCondition) bool {
			for _, output := range outputs {
				switch condition := output.(type) {
				case *shared.TilePlacementCondition:
					if condition.TileType == tileType {
						return true
					}
				case *shared.TileModificationCondition:
					if condition.TileType == tileType {
						return true
					}
				}
			}
			return false
		}
		found := false
		for _, behavior := range card.Behaviors {
			found = found || places(behavior.Outputs)
			for _, choice := range behavior.Choices {
				found = found || places(choice.Outputs)
			}
		}
		if found {
			if match != nil {
				return nil
			}
			copy := card.DeepCopy()
			match = &copy
		}
	}
	return match
}
