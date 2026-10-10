package game

import (
	"openmars/internal/game/global_parameters"
)

// Default values for game settings
const (
	DefaultMaxPlayers  = 5
	DefaultTemperature = global_parameters.MinTemperature // -30°C
	DefaultOxygen      = global_parameters.MinOxygen      // 0%
	DefaultOceans      = global_parameters.MinOceans      // 0
	DefaultVenus       = global_parameters.MinVenus       // 0%

	// MaxPlayerNameLength is the maximum number of runes allowed in a player name.
	MaxPlayerNameLength = 45
)
