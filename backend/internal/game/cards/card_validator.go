package cards

import (
	"errors"
	"fmt"
	"openmars/internal/game/board"
	"openmars/internal/game/colonies"

	"openmars/internal/game/global_parameters"
	"openmars/internal/game/player"
	"openmars/internal/game/shared"
)

// ValidateCardCanBePlayed checks if a card can be played in the current game context
// Returns nil if valid, or an error describing why the card cannot be played
func ValidateCardCanBePlayed(
	card *Card,
	pl *player.Player,
	globalParams *global_parameters.GlobalParameters,
	playedCards []*Card, // All cards played by the player (for tag counting)
) error {
	if card.Requirements == nil {
		return nil
	}
	counts := tagsFromCards(playedCards)
	for tag, n := range pl.BonusTags() {
		counts[tag] += n
	}
	if err := ValidateTagRequirements(card.Requirements.Items, counts); err != nil {
		return err
	}

	for _, req := range card.Requirements.Items {
		if err := validateRequirementMet(req, pl, globalParams); err != nil {
			return fmt.Errorf("requirement not met: %w", err)
		}
	}

	return nil
}

// CanAffordCard checks if a player can afford to pay for a card
func CanAffordCard(card *Card, pl *player.Player, discounts map[shared.CardTag]int) bool {
	totalCost := card.Cost

	// Apply tag-based discounts
	for _, tag := range card.Tags {
		if discount, ok := discounts[tag]; ok {
			totalCost -= discount
		}
	}

	// Cost cannot go below 0
	if totalCost < 0 {
		totalCost = 0
	}

	resources := pl.Resources().Get()
	return resources.Credits >= totalCost
}

// validateRequirementMet checks if a single requirement is met
func validateRequirementMet(
	req Requirement,
	pl *player.Player,
	globalParams *global_parameters.GlobalParameters,
) error {
	switch req.Type {
	case RequirementTemperature:
		return validateTemperatureRequirement(req, globalParams)
	case RequirementOxygen:
		return validateOxygenRequirement(req, globalParams)
	case RequirementOceans:
		return validateOceansRequirement(req, globalParams)
	case RequirementTags:
		return nil
	case RequirementProduction:
		return validateProductionRequirement(req, pl)
	case RequirementTR:
		return validateTRRequirement(req, pl)
	case RequirementResource:
		return validateResourceRequirement(req, pl)
	case RequirementCities:
		return validateCitiesRequirement(req)
	case RequirementGreeneries:
		return validateGreeeneriesRequirement(req)
	default:
		return fmt.Errorf("unknown requirement type: %s", req.Type)
	}
}

// validateTemperatureRequirement checks if temperature requirement is met
func validateTemperatureRequirement(req Requirement, globalParams *global_parameters.GlobalParameters) error {
	temp := globalParams.Temperature()

	if req.Min != nil && temp < *req.Min {
		return fmt.Errorf("temperature %d°C is below required minimum %d°C", temp, *req.Min)
	}

	if req.Max != nil && temp > *req.Max {
		return fmt.Errorf("temperature %d°C is above required maximum %d°C", temp, *req.Max)
	}

	return nil
}

// validateOxygenRequirement checks if oxygen requirement is met
func validateOxygenRequirement(req Requirement, globalParams *global_parameters.GlobalParameters) error {
	oxygen := globalParams.Oxygen()

	if req.Min != nil && oxygen < *req.Min {
		return fmt.Errorf("oxygen %d%% is below required minimum %d%%", oxygen, *req.Min)
	}

	if req.Max != nil && oxygen > *req.Max {
		return fmt.Errorf("oxygen %d%% is above required maximum %d%%", oxygen, *req.Max)
	}

	return nil
}

// validateOceansRequirement checks if oceans requirement is met
func validateOceansRequirement(req Requirement, globalParams *global_parameters.GlobalParameters) error {
	oceans := globalParams.Oceans()

	if req.Min != nil && oceans < *req.Min {
		return fmt.Errorf("ocean count %d is below required minimum %d", oceans, *req.Min)
	}

	if req.Max != nil && oceans > *req.Max {
		return fmt.Errorf("ocean count %d is above required maximum %d", oceans, *req.Max)
	}

	return nil
}

// validateProductionRequirement checks if production requirement is met
func validateProductionRequirement(req Requirement, pl *player.Player) error {
	if req.Resource == nil {
		return fmt.Errorf("production requirement missing resource type specification")
	}

	production := pl.Resources().Production()
	var productionAmount int

	switch *req.Resource {
	case shared.ResourceCredit:
		productionAmount = production.Credits
	case shared.ResourceSteel:
		productionAmount = production.Steel
	case shared.ResourceTitanium:
		productionAmount = production.Titanium
	case shared.ResourcePlant:
		productionAmount = production.Plants
	case shared.ResourceEnergy:
		productionAmount = production.Energy
	case shared.ResourceHeat:
		productionAmount = production.Heat
	default:
		return fmt.Errorf("invalid resource type for production requirement: %s", *req.Resource)
	}

	if req.Min != nil && productionAmount < *req.Min {
		return fmt.Errorf("%s production %d is below required minimum %d", *req.Resource, productionAmount, *req.Min)
	}

	if req.Max != nil && productionAmount > *req.Max {
		return fmt.Errorf("%s production %d is above required maximum %d", *req.Resource, productionAmount, *req.Max)
	}

	return nil
}

// validateTRRequirement checks if terraform rating requirement is met
func validateTRRequirement(req Requirement, pl *player.Player) error {
	tr := pl.Resources().TerraformRating()

	if req.Min != nil && tr < *req.Min {
		return fmt.Errorf("terraform rating %d is below required minimum %d", tr, *req.Min)
	}

	if req.Max != nil && tr > *req.Max {
		return fmt.Errorf("terraform rating %d is above required maximum %d", tr, *req.Max)
	}

	return nil
}

// validateResourceRequirement checks if resource amount requirement is met
func validateResourceRequirement(req Requirement, pl *player.Player) error {
	if req.Resource == nil {
		return fmt.Errorf("resource requirement missing resource type specification")
	}

	resources := pl.Resources().Get()
	var resourceAmount int

	switch *req.Resource {
	case shared.ResourceCredit:
		resourceAmount = resources.Credits
	case shared.ResourceSteel:
		resourceAmount = resources.Steel
	case shared.ResourceTitanium:
		resourceAmount = resources.Titanium
	case shared.ResourcePlant:
		resourceAmount = resources.Plants
	case shared.ResourceEnergy:
		resourceAmount = resources.Energy
	case shared.ResourceHeat:
		resourceAmount = resources.Heat
	default:
		storage := pl.Resources().Storage()
		if amount, ok := storage[string(*req.Resource)]; ok {
			resourceAmount = amount
		} else {
			resourceAmount = 0
		}
	}

	if req.Min != nil && resourceAmount < *req.Min {
		return fmt.Errorf("%s amount %d is below required minimum %d", *req.Resource, resourceAmount, *req.Min)
	}

	if req.Max != nil && resourceAmount > *req.Max {
		return fmt.Errorf("%s amount %d is above required maximum %d", *req.Resource, resourceAmount, *req.Max)
	}

	return nil
}

// validateCitiesRequirement checks if cities requirement is met
// TODO: Implement when city tracking is available
func validateCitiesRequirement(req Requirement) error {
	// Placeholder - requires board state to count cities
	return nil
}

// validateGreeeneriesRequirement checks if greeneries requirement is met
// TODO: Implement when greenery tracking is available
func validateGreeeneriesRequirement(req Requirement) error {
	// Placeholder - requires board state to count greeneries
	return nil
}

// ValidateTileRequirement checks city and greenery requirements.
func ValidateTileRequirement(req Requirement, p *player.Player, b *board.Board) error {
	count := 0
	if req.Type == RequirementCities {
		var location *string
		if req.Location != nil {
			value := string(*req.Location)
			location = &value
		}
		var owner *string
		if req.Target == "self-player" {
			id := p.ID()
			owner = &id
		}
		count = CountTilesOfTypeByLocation(b, shared.ResourceCityTile, location, owner)
	} else {
		resource := shared.ResourceGreeneryTile
		count = CountPlayerTiles(p.ID(), b, &resource)
	}
	if req.Min != nil && count < *req.Min {
		return fmt.Errorf("need at least %d %s tiles, have %d", *req.Min, req.Type, count)
	}
	if req.Max != nil && count > *req.Max {
		return fmt.Errorf("need at most %d %s tiles, have %d", *req.Max, req.Type, count)
	}
	return nil
}

var ErrColoniesTooFew = errors.New("not enough colonies")
var ErrColoniesTooMany = errors.New("too many colonies")

// ValidateColonyRequirement uses the authored ownership scope, retaining self as the default.
func ValidateColonyRequirement(req Requirement, playerID string, colonies *colonies.Colonies) error {
	count := colonies.CountPlayerColonies(playerID)
	if req.Target == "any-player" {
		count = colonies.CountAllColonies()
	}
	if req.Min != nil && count < *req.Min {
		return fmt.Errorf("%w: need %d, have %d", ErrColoniesTooFew, *req.Min, count)
	}
	if req.Max != nil && count > *req.Max {
		return fmt.Errorf("%w: maximum %d, have %d", ErrColoniesTooMany, *req.Max, count)
	}
	return nil
}
