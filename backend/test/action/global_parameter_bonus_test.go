package action_test

import (
	"context"
	"testing"

	"openmars/test/testutil"
)

func TestTemperatureBonus_HeatProductionAtMinus24(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialHeatProd := player.Resources().Production().Heat

	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -26), "set temperature")
	_, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase temperature")

	finalHeatProd := player.Resources().Production().Heat
	testutil.AssertEqual(t, initialHeatProd+1, finalHeatProd,
		"Player should gain +1 heat production when temperature crosses -24C")
}

func TestTemperatureBonus_HeatProductionAtMinus20(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialHeatProd := player.Resources().Production().Heat

	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -22), "set temperature")
	_, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase temperature")

	finalHeatProd := player.Resources().Production().Heat
	testutil.AssertEqual(t, initialHeatProd+1, finalHeatProd,
		"Player should gain +1 heat production when temperature crosses -20C")
}

func TestTemperatureBonus_CrossBothThresholds(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialHeatProd := player.Resources().Production().Heat

	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -26), "set temperature")
	_, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 3, playerID)
	testutil.AssertNoError(t, err, "increase temperature")

	finalHeatProd := player.Resources().Production().Heat
	testutil.AssertEqual(t, initialHeatProd+2, finalHeatProd,
		"Player should gain +2 heat production when crossing both -24C and -20C thresholds")
}

func TestTemperatureBonus_OceanQueuedAtZero(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -2), "set temperature")
	_, err := testGame.GlobalParameters().IncreaseTemperature(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase temperature")

	// The queue auto-processes the first tile into the active pending tile selection
	selection := testGame.GetPendingTileSelection(playerID)
	testutil.AssertTrue(t, selection != nil,
		"Ocean tile selection should be pending when temperature crosses 0C")
	testutil.AssertEqual(t, "ocean", selection.TileType,
		"Pending tile type should be ocean")
}

func TestOxygenBonus_TemperatureStepAt8(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialTR := player.Resources().TerraformRating()

	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 7), "set oxygen")
	initialTemp := testGame.GlobalParameters().Temperature()

	_, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase oxygen")

	finalTemp := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, initialTemp+2, finalTemp,
		"Temperature should increase by 2 degrees (1 step) when oxygen crosses 8%")

	finalTR := player.Resources().TerraformRating()
	testutil.AssertEqual(t, initialTR+1, finalTR,
		"Player should gain +1 TR from the bonus temperature increase when oxygen crosses 8%")
}

func TestOxygenBonus_ChainedWithTemperatureBonus(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialHeatProd := player.Resources().Production().Heat
	initialTR := player.Resources().TerraformRating()

	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -26), "set temperature")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetOxygen(ctx, 7), "set oxygen")

	_, err := testGame.GlobalParameters().IncreaseOxygen(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase oxygen")

	finalTemp := testGame.GlobalParameters().Temperature()
	testutil.AssertEqual(t, -24, finalTemp,
		"Temperature should increase from -26 to -24 via oxygen bonus")

	finalHeatProd := player.Resources().Production().Heat
	testutil.AssertEqual(t, initialHeatProd+1, finalHeatProd,
		"Player should gain +1 heat production from chained temperature bonus at -24C")

	finalTR := player.Resources().TerraformRating()
	testutil.AssertEqual(t, initialTR+1, finalTR,
		"Player should gain +1 TR from the bonus temperature increase via oxygen bonus chain")
}

func TestVenusBonus_CardDrawAt8(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialCardCount := player.Hand().CardCount()

	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 6), "set venus")
	_, err := testGame.GlobalParameters().IncreaseVenus(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase venus")

	finalCardCount := player.Hand().CardCount()
	testutil.AssertEqual(t, initialCardCount+1, finalCardCount,
		"Player should draw 1 card when venus crosses 8%")
	receipts := player.Selection().CardReceipts()
	testutil.AssertEqual(t, 1, len(receipts), "Venus draw creates one private receipt")
	testutil.AssertEqual(t, "Venus Bonus", receipts[0].Source, "Venus receipt source")
	testutil.AssertEqual(t, 1, len(receipts[0].Cards), "Venus receipt shows the drawn card")
}

func TestVenusBonus_TRAt16(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialTR := player.Resources().TerraformRating()

	testutil.AssertNoError(t, testGame.GlobalParameters().SetVenus(ctx, 14), "set venus")
	_, err := testGame.GlobalParameters().IncreaseVenus(ctx, 1, playerID)
	testutil.AssertNoError(t, err, "increase venus")

	finalTR := player.Resources().TerraformRating()
	testutil.AssertEqual(t, initialTR+1, finalTR,
		"TR should increase by 1 from the venus bonus at 16%")
}

func TestAdminSetDoesNotTriggerBonus(t *testing.T) {
	testGame, _, _, playerID := testutil.SetupSoloGame(t)
	ctx := context.Background()

	player, _ := testGame.GetPlayer(playerID)
	initialHeatProd := player.Resources().Production().Heat

	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -26), "set temperature to -26")
	testutil.AssertNoError(t, testGame.GlobalParameters().SetTemperature(ctx, -20), "set temperature to -20")

	finalHeatProd := player.Resources().Production().Heat
	testutil.AssertEqual(t, initialHeatProd, finalHeatProd,
		"Admin SetTemperature should not trigger heat production bonus")
}
