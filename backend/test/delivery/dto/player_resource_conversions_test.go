package dto_test

import (
	"testing"

	"openmars/internal/delivery/dto"
	"openmars/internal/game/shared"
)

func findResourceConversion(t *testing.T, conversions []dto.PlayerResourceConversionDto, projectType dto.StandardProject) dto.PlayerResourceConversionDto {
	t.Helper()
	for _, conversion := range conversions {
		if conversion.ProjectType == projectType {
			return conversion
		}
	}
	t.Fatalf("expected resource conversion %q in player DTO", projectType)
	return dto.PlayerResourceConversionDto{}
}

func TestToPlayerDto_ResourceConversions_AvailableWhenAffordable(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayerGame(t)
	p.Resources().Set(shared.Resources{Plants: 8, Heat: 8})

	playerDto := dto.ToPlayerDto(p, g, cardRegistry, nil, nil, nil, nil)

	plants := findResourceConversion(t, playerDto.ResourceConversions, dto.StandardProjectConvertPlantsToGreenery)
	if !plants.Available {
		t.Fatalf("expected plant conversion to be available with 8 plants, errors: %+v", plants.Errors)
	}
	if plants.EffectiveCost[string(shared.ResourcePlant)] != 8 {
		t.Fatalf("expected plant conversion to cost 8 plants, got %+v", plants.EffectiveCost)
	}

	heat := findResourceConversion(t, playerDto.ResourceConversions, dto.StandardProjectConvertHeatToTemperature)
	if !heat.Available {
		t.Fatalf("expected heat conversion to be available with 8 heat, errors: %+v", heat.Errors)
	}
}

func TestToPlayerDto_ResourceConversions_UnavailableWhenShort(t *testing.T) {
	g, p, cardRegistry := setupStuckPlayerGame(t)
	p.Resources().Set(shared.Resources{Plants: 7, Heat: 7})

	playerDto := dto.ToPlayerDto(p, g, cardRegistry, nil, nil, nil, nil)

	for _, projectType := range []dto.StandardProject{dto.StandardProjectConvertPlantsToGreenery, dto.StandardProjectConvertHeatToTemperature} {
		conversion := findResourceConversion(t, playerDto.ResourceConversions, projectType)
		if conversion.Available {
			t.Fatalf("expected %q to be unavailable with 7 resources", projectType)
		}
		if len(conversion.Errors) == 0 {
			t.Fatalf("expected %q to report why it is unavailable", projectType)
		}
	}
}
