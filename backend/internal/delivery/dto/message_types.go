package dto

// MessageType represents different types of WebSocket messages
type MessageType string

const (
	MessageTypeWatchResumeGame   MessageType = "watch-resume-game"
	MessageTypeClaimResumeSeat   MessageType = "claim-resume-seat"
	MessageTypeReleaseResumeSeat MessageType = "release-resume-seat"
	MessageTypeResumeGame        MessageType = "resume-game"
	MessageTypeResumeBotToken    MessageType = "resume-bot-token"
	MessageTypeQuotePayment      MessageType = "quote-payment"
	MessageTypePaymentQuote      MessageType = "payment-quote"
)

const (
	MessageTypePlayerConnect MessageType = "player-connect"

	MessageTypeGameUpdated     MessageType = "game-updated"
	MessageTypePlayerConnected MessageType = "player-connected"
	MessageTypeError           MessageType = "error"
	MessageTypeLogUpdate       MessageType = "log-update"

	MessageTypeActionStandardProject    MessageType = "action.standard-project"
	MessageTypeActionConfirmSellPatents MessageType = "action.standard-project.confirm-sell-patents"

	MessageTypeActionConvertPlantsToGreenery  MessageType = "action.resource-conversion.convert-plants-to-greenery"
	MessageTypeActionConvertHeatToTemperature MessageType = "action.resource-conversion.convert-heat-to-temperature"

	MessageTypeAddBot                   MessageType = "add-bot"
	MessageTypeActionStartGame          MessageType = "action.game-management.start-game"
	MessageTypeActionSkipAction         MessageType = "action.game-management.skip-action"
	MessageTypeActionSelectDemoChoices  MessageType = "action.game-management.select-demo-choices"
	MessageTypeActionConfirmInitAdvance MessageType = "action.game-management.confirm-init-advance"

	MessageTypeActionClaimMilestone MessageType = "action.milestone.claim-milestone"
	MessageTypeActionFundAward      MessageType = "action.award.fund-award"

	MessageTypeActionTileSelected MessageType = "action.tile-selection.tile-selected"

	MessageTypeActionPlayCard                MessageType = "action.card.play-card"
	MessageTypeActionCardAction              MessageType = "action.card.card-action"
	MessageTypeActionSelectStartingChoices   MessageType = "action.card.select-starting-choices"
	MessageTypeActionConfirmProductionCards  MessageType = "action.card.confirm-production-cards"
	MessageTypeActionCardDrawConfirmed       MessageType = "action.card.card-draw-confirmed"
	MessageTypeActionCardDiscardConfirmed    MessageType = "action.card.card-discard-confirmed"
	MessageTypeActionConfirmOptionalBehavior MessageType = "action.card.confirm-optional-behavior"
	MessageTypeActionBehaviorChoiceConfirmed MessageType = "action.card.behavior-choice-confirmed"
	MessageTypeActionConfirmResourceRemoval  MessageType = "action.card.confirm-resource-removal"

	MessageTypeActionColonyTrade            MessageType = "action.colony.trade"
	MessageTypeActionColonyBuild            MessageType = "action.colony.build"
	MessageTypeActionProjectFundingSeat     MessageType = "action.project-funding.buy-seat"
	MessageTypeActionConfirmColonyResource  MessageType = "action.confirm-colony-resource"
	MessageTypeActionConfirmAwardFund       MessageType = "action.confirm-award-fund"
	MessageTypeActionAcknowledgeCardReceipt MessageType = "action.acknowledge-card-receipt"
	MessageTypeActionConfirmColonyPlacement MessageType = "action.confirm-colony-placement"
	MessageTypeActionConfirmCardReveal      MessageType = "action.confirm-card-reveal"
	MessageTypeActionConfirmEffectSelection MessageType = "action.confirm-effect-selection"
	MessageTypeActionConfirmFreeTrade       MessageType = "action.confirm-free-trade"

	MessageTypeAdminCommand MessageType = "admin-command"

	MessageTypeRequestLogs MessageType = "request-logs"

	MessageTypePlayerTakeover MessageType = "player-takeover"
	MessageTypeKickPlayer     MessageType = "kick-player"
	MessageTypePlayerKicked   MessageType = "player-kicked"
	MessageTypeConvertToBot   MessageType = "convert-to-bot"
	MessageTypeEndGame        MessageType = "end-game"
	MessageTypeGameEnded      MessageType = "game-ended"

	MessageTypeUpdateGameSettings MessageType = "update-game-settings"
	MessageTypeSetPlayerColor     MessageType = "set-player-color"
	MessageTypeSpectatorConnect   MessageType = "spectator-connect"
	MessageTypeSpectatorConnected MessageType = "spectator-connected"
	MessageTypeChatMessage        MessageType = "chat-message"
	MessageTypeChatUpdate         MessageType = "chat-update"
	MessageTypeEmoteSend          MessageType = "emote-send"
	MessageTypeEmote              MessageType = "emote"
	MessageTypeBotThought         MessageType = "bot-thought"
	MessageTypeBotRetry           MessageType = "bot-retry"
	MessageTypeBotInspect         MessageType = "bot-inspect"
	MessageTypeBotTraceSnapshot   MessageType = "bot-trace-snapshot"
	MessageTypeBotTraceEvent      MessageType = "bot-trace-event"
	MessageTypeKickSpectator      MessageType = "kick-spectator"
	MessageTypeSpectatorKicked    MessageType = "spectator-kicked"
)
