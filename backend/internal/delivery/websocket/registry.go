package websocket

import (
	adminAction "openmars/internal/action/admin"
	awardAction "openmars/internal/action/award"
	cardAction "openmars/internal/action/card"
	colonyAction "openmars/internal/action/colony"
	confirmAction "openmars/internal/action/confirmation"
	connAction "openmars/internal/action/connection"
	gameAction "openmars/internal/action/game"
	milestoneAction "openmars/internal/action/milestone"
	pfAction "openmars/internal/action/projectfunding"
	resconvAction "openmars/internal/action/resource_conversion"
	stdprojAction "openmars/internal/action/standard_project"
	tileAction "openmars/internal/action/tile"
	turnAction "openmars/internal/action/turn_management"
	"openmars/internal/delivery/dto"
	"openmars/internal/delivery/websocket/core"
	"openmars/internal/delivery/websocket/handler/admin"
	"openmars/internal/delivery/websocket/handler/award"
	"openmars/internal/delivery/websocket/handler/card"
	colonyHandler "openmars/internal/delivery/websocket/handler/colony"
	"openmars/internal/delivery/websocket/handler/confirmation"
	"openmars/internal/delivery/websocket/handler/connection"
	"openmars/internal/delivery/websocket/handler/game"
	"openmars/internal/delivery/websocket/handler/milestone"
	pfHandler "openmars/internal/delivery/websocket/handler/projectfunding"
	"openmars/internal/delivery/websocket/handler/resource_conversion"
	"openmars/internal/delivery/websocket/handler/standard_project"
	"openmars/internal/delivery/websocket/handler/tile"
	"openmars/internal/delivery/websocket/handler/turn_management"
	gameModel "openmars/internal/game"
	"openmars/internal/logger"
)

// RegisterHandlers registers all action handlers with the hub.
func RegisterHandlers(
	hub *core.Hub,
	broadcaster *Broadcaster,
	gameRepo gameModel.GameRepository,
	joinGameAction *gameAction.JoinGameAction,
	addBotAction *gameAction.AddBotAction,
	selectDemoChoicesAction *gameAction.SelectDemoChoicesAction,
	updateGameSettingsAction *gameAction.UpdateGameSettingsAction,
	playCardAction *cardAction.PlayCardAction,
	useCardActionAction *cardAction.UseCardActionAction,
	executeStandardProjectAction *stdprojAction.ExecuteStandardProjectAction,
	convertHeatAction *resconvAction.ConvertHeatToTemperatureAction,
	convertPlantsAction *resconvAction.ConvertPlantsToGreeneryAction,
	selectTileAction *tileAction.SelectTileAction,
	startGameAction *turnAction.StartGameAction,
	skipActionAction *turnAction.SkipActionAction,
	selectStartingChoicesAction *turnAction.SelectStartingChoicesAction,
	confirmInitAdvanceAction *turnAction.ConfirmInitAdvanceAction,
	confirmSellPatentsAction *confirmAction.ConfirmSellPatentsAction,
	confirmProductionCardsAction *confirmAction.ConfirmProductionCardsAction,
	confirmCardDrawAction *confirmAction.ConfirmCardDrawAction,
	confirmCardDiscardAction *confirmAction.ConfirmCardDiscardAction,
	confirmBehaviorChoiceAction *confirmAction.ConfirmBehaviorChoiceAction,
	confirmResourceRemovalAction *confirmAction.ConfirmResourceRemovalAction,
	confirmColonyResourceAction *confirmAction.ConfirmColonyResourceAction,
	confirmAwardFundAction *confirmAction.ConfirmAwardFundAction,
	confirmColonyPlacementAction *confirmAction.ConfirmColonyPlacementAction,
	confirmFreeTradeAction *confirmAction.ConfirmFreeTradeAction,
	confirmEffectSelectionAction *confirmAction.ConfirmEffectSelectionAction,
	confirmCardRevealAction *confirmAction.ConfirmCardRevealAction,
	playerDisconnectedAction *connAction.PlayerDisconnectedAction,
	playerTakeoverAction *connAction.PlayerTakeoverAction,
	kickPlayerAction *connAction.KickPlayerAction,
	endGameAction *connAction.EndGameAction,
	setPlayerColorAction *connAction.SetPlayerColorAction,
	spectateGameAction *connAction.SpectateGameAction,
	spectatorDisconnectedAction *connAction.SpectatorDisconnectedAction,
	kickSpectatorAction *connAction.KickSpectatorAction,
	sendChatMessageAction *connAction.SendChatMessageAction,
	convertToBotAction *gameAction.ConvertToBotAction,
	bots interface {
		game.BotRetrier
		game.BotInspector
	},
	claimMilestoneAction *milestoneAction.ClaimMilestoneAction,
	fundAwardAction *awardAction.FundAwardAction,
	colonyTradeAction *colonyAction.TradeAction,
	colonyBuildAction *colonyAction.BuildColonyAction,
	fundSeatAction *pfAction.FundSeatAction,
	adminSetPhaseAction *adminAction.SetPhaseAction,
	adminSetCurrentTurnAction *adminAction.SetCurrentTurnAction,
	adminSetResourcesAction *adminAction.SetResourcesAction,
	adminSetProductionAction *adminAction.SetProductionAction,
	adminSetGlobalParametersAction *adminAction.SetGlobalParametersAction,
	adminGiveCardAction *adminAction.GiveCardAction,
	adminSetCorporationAction *adminAction.SetCorporationAction,
	adminStartTileSelectionAction *adminAction.StartTileSelectionAction,
	adminSetTRAction *adminAction.SetTRAction,
	adminRestartGameAction *adminAction.RestartGameAction,
	adminSetActionsRemainingAction *adminAction.SetActionsRemainingAction,
) {
	hub.RegisterHandler(dto.MessageTypeQuotePayment, &paymentQuoteHandler{broadcaster: broadcaster})
	log := logger.Get()
	log.Debug("Registering WebSocket handlers")

	joinGameHandler := game.NewJoinGameHandler(joinGameAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypePlayerConnect, joinGameHandler)

	addBotHandler := game.NewAddBotHandler(addBotAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeAddBot, addBotHandler)

	selectDemoChoicesHandler := game.NewSelectDemoChoicesHandler(selectDemoChoicesAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionSelectDemoChoices, selectDemoChoicesHandler)

	updateGameSettingsHandler := game.NewUpdateGameSettingsHandler(updateGameSettingsAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeUpdateGameSettings, updateGameSettingsHandler)

	playCardHandler := card.NewPlayCardHandler(playCardAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionPlayCard, playCardHandler)

	useCardActionHandler := card.NewUseCardActionHandler(useCardActionAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionCardAction, useCardActionHandler)

	stdProjHandler := standard_project.NewExecuteHandler(executeStandardProjectAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionStandardProject, stdProjHandler)

	convertHeatHandler := resource_conversion.NewConvertHeatHandler(convertHeatAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConvertHeatToTemperature, convertHeatHandler)

	convertPlantsHandler := resource_conversion.NewConvertPlantsHandler(convertPlantsAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConvertPlantsToGreenery, convertPlantsHandler)

	selectTileHandler := tile.NewSelectTileHandler(selectTileAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionTileSelected, selectTileHandler)

	startGameHandler := turn_management.NewStartGameHandler(startGameAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionStartGame, startGameHandler)

	skipActionHandler := turn_management.NewSkipActionHandler(skipActionAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionSkipAction, skipActionHandler)

	selectStartingChoicesHandler := turn_management.NewSelectStartingChoicesHandler(selectStartingChoicesAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionSelectStartingChoices, selectStartingChoicesHandler)

	confirmInitAdvanceHandler := turn_management.NewConfirmInitAdvanceHandler(confirmInitAdvanceAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmInitAdvance, confirmInitAdvanceHandler)

	confirmSellPatentsHandler := confirmation.NewConfirmSellPatentsHandler(confirmSellPatentsAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmSellPatents, confirmSellPatentsHandler)

	confirmProductionCardsHandler := confirmation.NewConfirmProductionCardsHandler(confirmProductionCardsAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmProductionCards, confirmProductionCardsHandler)

	confirmCardDrawHandler := confirmation.NewConfirmCardDrawHandler(confirmCardDrawAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionCardDrawConfirmed, confirmCardDrawHandler)
	hub.RegisterHandler(dto.MessageTypeActionAcknowledgeCardReceipt, confirmCardDrawHandler)

	confirmCardDiscardHandler := confirmation.NewConfirmCardDiscardHandler(confirmCardDiscardAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionCardDiscardConfirmed, confirmCardDiscardHandler)

	confirmBehaviorChoiceHandler := confirmation.NewConfirmBehaviorChoiceHandler(confirmBehaviorChoiceAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionBehaviorChoiceConfirmed, confirmBehaviorChoiceHandler)
	hub.RegisterHandler(dto.MessageTypeActionConfirmOptionalBehavior, confirmBehaviorChoiceHandler)

	confirmResourceRemovalHandler := confirmation.NewConfirmResourceRemovalHandler(confirmResourceRemovalAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmResourceRemoval, confirmResourceRemovalHandler)

	confirmColonyResourceHandler := confirmation.NewConfirmColonyResourceHandler(confirmColonyResourceAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmColonyResource, confirmColonyResourceHandler)

	confirmAwardFundHandler := confirmation.NewConfirmAwardFundHandler(confirmAwardFundAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmAwardFund, confirmAwardFundHandler)

	confirmColonyPlacementHandler := confirmation.NewConfirmColonyPlacementHandler(confirmColonyPlacementAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmColonyPlacement, confirmColonyPlacementHandler)

	hub.RegisterHandler(dto.MessageTypeActionConfirmCardReveal, confirmation.NewConfirmCardRevealHandler(confirmCardRevealAction, broadcaster))
	hub.RegisterHandler(dto.MessageTypeActionConfirmEffectSelection, confirmation.NewConfirmEffectSelectionHandler(confirmEffectSelectionAction, broadcaster))

	confirmFreeTradeHandler := confirmation.NewConfirmFreeTradeHandler(confirmFreeTradeAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionConfirmFreeTrade, confirmFreeTradeHandler)

	requestLogsHandler := connection.NewRequestLogsHandler(broadcaster)
	hub.RegisterHandler(dto.MessageTypeRequestLogs, requestLogsHandler)

	playerTakeoverHandler := connection.NewPlayerTakeoverHandler(playerTakeoverAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypePlayerTakeover, playerTakeoverHandler)

	kickPlayerHandler := connection.NewKickPlayerHandler(kickPlayerAction, broadcaster, hub)
	hub.RegisterHandler(dto.MessageTypeKickPlayer, kickPlayerHandler)

	endGameHandler := connection.NewEndGameHandler(endGameAction, hub)
	hub.RegisterHandler(dto.MessageTypeEndGame, endGameHandler)

	setPlayerColorHandler := connection.NewSetPlayerColorHandler(setPlayerColorAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeSetPlayerColor, setPlayerColorHandler)

	spectateGameHandler := connection.NewSpectateGameHandler(spectateGameAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeSpectatorConnect, spectateGameHandler)

	kickSpectatorHandler := connection.NewKickSpectatorHandler(kickSpectatorAction, broadcaster, hub)
	hub.RegisterHandler(dto.MessageTypeKickSpectator, kickSpectatorHandler)

	chatMessageHandler := connection.NewChatMessageHandler(sendChatMessageAction, broadcaster, gameRepo)
	hub.RegisterHandler(dto.MessageTypeChatMessage, chatMessageHandler)

	convertToBotHandler := game.NewConvertToBotHandler(convertToBotAction, broadcaster, hub)
	hub.RegisterHandler(dto.MessageTypeConvertToBot, convertToBotHandler)

	hub.RegisterHandler(dto.MessageTypeBotRetry, game.NewBotRetryHandler(bots))
	hub.RegisterHandler(dto.MessageTypeBotInspect, game.NewBotInspectHandler(bots))
	hub.RegisterHandler(dto.MessageTypeEmoteSend, connection.NewEmoteSendHandler(broadcaster))

	claimMilestoneHandler := milestone.NewClaimMilestoneHandler(claimMilestoneAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionClaimMilestone, claimMilestoneHandler)

	fundAwardHandler := award.NewFundAwardHandler(fundAwardAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionFundAward, fundAwardHandler)

	colonyTradeHandler := colonyHandler.NewTradeHandler(colonyTradeAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionColonyTrade, colonyTradeHandler)

	colonyBuildHandler := colonyHandler.NewBuildColonyHandler(colonyBuildAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionColonyBuild, colonyBuildHandler)

	fundSeatHandler := pfHandler.NewFundSeatHandler(fundSeatAction, broadcaster)
	hub.RegisterHandler(dto.MessageTypeActionProjectFundingSeat, fundSeatHandler)

	adminCommandHandler := admin.NewAdminCommandHandler(
		adminSetPhaseAction,
		adminSetCurrentTurnAction,
		adminSetResourcesAction,
		adminSetProductionAction,
		adminSetGlobalParametersAction,
		adminGiveCardAction,
		adminSetCorporationAction,
		adminStartTileSelectionAction,
		adminSetTRAction,
		adminRestartGameAction,
		adminSetActionsRemainingAction,
		gameRepo,
		broadcaster,
	)
	hub.RegisterHandler(dto.MessageTypeAdminCommand, adminCommandHandler)

	hub.OnLeave(connection.LeaveHandlers(playerDisconnectedAction, spectatorDisconnectedAction, broadcaster))

	log.Debug("WebSocket handlers registered")
}
