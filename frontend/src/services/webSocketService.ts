import type { ResumeGameRequest } from "../types/generated/api-types.ts";
import { useCardPlayFlowStore } from "@/stores/cardPlayFlowStore";
import { selectPayment, usePaymentStore } from "@/stores/paymentStore";
import type { PaymentIntentDto, PaymentQuoteDto } from "@/types/generated/api-types";
import { v4 as uuidv4 } from "uuid";
import { getWebSocketUrl } from "../config";
import {
  PaymentDto,
  SelectDemoChoicesRequest,
  ErrorPayload,
  GameUpdatedPayload,
  LogUpdatePayload,
  MessageType,
  MessageTypeError,
  MessageTypeGameUpdated,
  MessageTypeLogUpdate,
  MessageTypePlayerConnect,
  MessageTypePlayerConnected,
  MessageTypePlayerKicked,
  MessageTypeActionStandardProject,
  MessageTypeActionStartGame,
  MessageTypeActionSkipAction,
  MessageTypeActionPlayCard,
  MessageTypeActionCardAction,
  MessageTypeActionSelectStartingChoices,
  MessageTypeActionConfirmSellPatents,
  MessageTypeActionConfirmProductionCards,
  MessageTypeActionCardDrawConfirmed,
  MessageTypeActionTileSelected,
  MessageTypeActionConvertPlantsToGreenery,
  MessageTypeActionConvertHeatToTemperature,
  MessageTypeActionSelectDemoChoices,
  MessageTypeActionClaimMilestone,
  MessageTypeActionFundAward,
  MessageTypeAddBot,
  MessageTypeBotInspect,
  MessageTypeBotRetry,
  MessageTypeBotThought,
  MessageTypeBotTraceEvent,
  MessageTypeBotTraceSnapshot,
  MessageTypeEmote,
  MessageTypeEmoteSend,
  MessageTypeKickPlayer,
  MessageTypeEndGame,
  MessageTypeGameEnded,
  MessageTypeConvertToBot,
  MessageTypeActionBehaviorChoiceConfirmed,
  MessageTypeActionConfirmOptionalBehavior,
  MessageTypeActionConfirmResourceRemoval,
  MessageTypeActionConfirmColonyResource,
  MessageTypeActionConfirmColonyPlacement,
  MessageTypeActionConfirmFreeTrade,
  MessageTypeActionConfirmEffectSelection,
  MessageTypeActionConfirmCardReveal,
  MessageTypeActionConfirmAwardFund,
  MessageTypeActionCardDiscardConfirmed,
  MessageTypeActionConfirmInitAdvance,
  MessageTypeRequestLogs,
  MessageTypeSetPlayerColor,
  MessageTypeSpectatorConnect,
  MessageTypeSpectatorConnected,
  MessageTypeChatMessage,
  MessageTypeChatUpdate,
  MessageTypeKickSpectator,
  MessageTypeSpectatorKicked,
  MessageTypeUpdateGameSettings,
  UpdateGameSettingsRequest,
  MessageTypeActionColonyTrade,
  MessageTypeActionColonyBuild,
  MessageTypeActionProjectFundingSeat,
  // Payload types
  BotThoughtPayload,
  BotTraceDto,
  BotTraceEventDto,
  ChatUpdatePayload,
  EmotePayload,
  EmoteSendPayload,
  PlayerConnectedPayload,
  WebSocketMessage,
} from "../types/generated/api-types.ts";

type EventCallback = (data: any) => void;

export class WebSocketService {
  private paymentQuotes = new Map<
    string,
    { resolve: (q: PaymentQuoteDto) => void; reject: (e: Error) => void }
  >();
  private ws: WebSocket | null = null;
  private readonly url: string;
  private listeners: { [event: string]: EventCallback[] } = {};
  private isConnected = false;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private currentGameId: string | null = null;
  private pendingConnection: Promise<void> | null = null;
  private shouldReconnect = true;

  constructor(url?: string) {
    this.url = url || getWebSocketUrl();
  }

  connect(): Promise<void> {
    // If already connected, resolve immediately
    if (this.isConnected && this.ws && this.ws.readyState === WebSocket.OPEN) {
      return Promise.resolve();
    }

    // If already connecting, return the existing pending promise
    if (this.pendingConnection) {
      return this.pendingConnection;
    }

    // Create new connection promise
    this.pendingConnection = new Promise((resolve, reject) => {
      try {
        // Close existing connection if it exists
        if (this.ws) {
          this.ws.close();
        }

        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
          this.isConnected = true;
          this.pendingConnection = null;
          this.reconnectAttempts = 0;
          this.emit("connect");
          resolve();
        };

        this.ws.onmessage = (event) => {
          let message: any;
          try {
            message = JSON.parse(event.data);
          } catch (error) {
            console.error("Failed to parse WebSocket message:", error);
            return;
          }

          try {
            this.handleMessage(message);
          } catch (error) {
            console.error("Error handling WebSocket message:", error);
          }
        };

        this.ws.onclose = (event) => {
          for (const request of this.paymentQuotes.values())
            request.reject(new Error("Connection closed"));
          this.paymentQuotes.clear();
          usePaymentStore.getState().pending?.reject(new Error("Connection closed"));
          this.isConnected = false;
          this.emit("disconnect");

          if (this.shouldReconnect && event.code !== 1000) {
            this.attemptReconnect();
          }
        };

        this.ws.onerror = (error) => {
          console.error("WebSocket error:", error);
          this.pendingConnection = null;
          this.emit("error", error);
          if (!this.isConnected) {
            reject(error);
          }
        };
      } catch (error) {
        this.pendingConnection = null;
        reject(error);
      }
    });

    return this.pendingConnection;
  }

  private handleMessage(message: WebSocketMessage) {
    switch (message.type) {
      case "payment-quote": {
        const payload = message.payload as {
          requestId: string;
          quote?: PaymentQuoteDto;
          error?: string;
        };
        const request = this.paymentQuotes.get(payload.requestId);
        this.paymentQuotes.delete(payload.requestId);
        if (payload.quote) request?.resolve(payload.quote);
        else request?.reject(new Error(payload.error ?? "Could not quote payment"));
        break;
      }
      case MessageTypeGameUpdated: {
        const gamePayload = message.payload as GameUpdatedPayload;
        // Handle both direct game data and nested structure
        const gameData = gamePayload.game || gamePayload;
        this.emit("game-updated", gameData);
        break;
      }
      case MessageTypePlayerConnected: {
        const connectedPayload = message.payload as PlayerConnectedPayload;
        // This is a confirmation that player joined successfully
        // The full game state will arrive via game-updated from broadcaster
        this.emit("player-connected", connectedPayload);
        break;
      }
      case MessageTypeError: {
        const errorPayload = message.payload as ErrorPayload;
        this.emit("error", errorPayload);
        break;
      }
      case MessageTypeLogUpdate: {
        const logPayload = message.payload as LogUpdatePayload;
        this.emit("log-update", logPayload);
        break;
      }
      case MessageTypePlayerKicked: {
        this.emit("player-kicked", message.payload);
        break;
      }
      case MessageTypeGameEnded: {
        this.emit("game-ended", message.payload);
        break;
      }
      case MessageTypeSpectatorConnected: {
        this.emit("spectator-connected", message.payload);
        break;
      }
      case MessageTypeChatUpdate: {
        const chatPayload = message.payload as ChatUpdatePayload;
        this.emit("chat-update", chatPayload.chatMessage);
        break;
      }
      case MessageTypeSpectatorKicked: {
        this.emit("spectator-kicked", message.payload);
        break;
      }
      case MessageTypeEmote: {
        this.emit("emote", message.payload as EmotePayload);
        break;
      }
      case MessageTypeBotThought: {
        this.emit("bot-thought", message.payload as BotThoughtPayload);
        break;
      }
      case MessageTypeBotTraceSnapshot: {
        this.emit("bot-trace-snapshot", message.payload as BotTraceDto);
        break;
      }
      case MessageTypeBotTraceEvent: {
        this.emit("bot-trace-event", message.payload as BotTraceEventDto);
        break;
      }
      default:
        console.warn("Unknown message type:", message.type);
    }
  }

  send(type: MessageType, payload: unknown, gameId?: string): string {
    const reqId = uuidv4();

    if (!this.isConnected || !this.ws) {
      throw new Error("WebSocket is not connected");
    }

    const message: WebSocketMessage = {
      type,
      payload,
      gameId: gameId || this.currentGameId || undefined,
    };

    this.ws.send(JSON.stringify(message));

    return reqId;
  }

  quotePayment(intent: PaymentIntentDto): Promise<PaymentQuoteDto> {
    const requestId = uuidv4();
    return new Promise((resolve, reject) => {
      this.paymentQuotes.set(requestId, { resolve, reject });
      try {
        this.send("quote-payment", { requestId, intent });
      } catch (error) {
        this.paymentQuotes.delete(requestId);
        reject(error);
      }
    });
  }

  private async sendWithPayment(
    type: MessageType,
    action: string,
    payload: Record<string, unknown>,
  ): Promise<string> {
    const session = action === "play-card" ? useCardPlayFlowStore.getState().playSession : null;
    const sessionId = session && session.card.id === payload.cardId ? session.id : null;
    try {
      const intent = { ...payload, action } as PaymentIntentDto;
      const quote = await this.quotePayment(intent);
      const activeSession = useCardPlayFlowStore.getState().playSession;
      if (
        sessionId &&
        (activeSession?.id !== sessionId ||
          (activeSession.phase !== "preparing" && activeSession.phase !== "choosing"))
      ) {
        throw new Error("Payment cancelled");
      }
      let payment = payload.payment as PaymentDto | undefined;
      if (!payment) {
        const hasSubstitutes = quote.options.some(
          (o) =>
            o.available > 0 &&
            (o.source.target !== "self-player" || o.source.resource !== o.targetResource),
        );
        if (hasSubstitutes) {
          payment = await selectPayment(intent, quote, () => this.quotePayment(intent));
        } else {
          payment = {
            allocations: quote.options
              .filter(
                (o) =>
                  o.source.target === "self-player" &&
                  o.source.resource === o.targetResource &&
                  (quote.costs[o.targetResource] ?? 0) > 0,
              )
              .map((o) => ({
                source: o.source,
                targetResource: o.targetResource,
                amount: quote.costs[o.targetResource],
              })),
          };
        }
      }
      if (sessionId && !useCardPlayFlowStore.getState().submitPlay(sessionId)) {
        throw new Error("Payment cancelled");
      }
      return this.send(type, { ...payload, payment });
    } catch (error) {
      if (sessionId) {
        useCardPlayFlowStore.getState().returnPlay(sessionId, error);
      }
      throw error;
    }
  }

  playerConnect(playerName: string, gameId: string, playerId?: string): void {
    const payload: any = { playerName, gameId };
    if (playerId) {
      payload.playerId = playerId;
    }

    this.send(MessageTypePlayerConnect, payload, gameId);
    this.currentGameId = gameId;
  }

  standardProject(projectId: string): Promise<string> {
    return this.sendWithPayment(MessageTypeActionStandardProject, "standard-project", {
      projectId,
    });
  }

  convertPlantsToGreenery(payment?: PaymentDto): Promise<string> {
    return this.sendWithPayment(MessageTypeActionConvertPlantsToGreenery, "convert-plants", {
      type: "convert-plants-to-greenery",
      payment,
    });
  }

  convertHeatToTemperature(payment?: PaymentDto): Promise<string> {
    return this.sendWithPayment(MessageTypeActionConvertHeatToTemperature, "convert-heat", {
      type: "convert-heat-to-temperature",
      payment,
    });
  }

  startGame(): string {
    return this.send(MessageTypeActionStartGame, {});
  }

  skipAction(): string {
    return this.send(MessageTypeActionSkipAction, {});
  }

  playCard(
    cardId: string,
    payment: PaymentDto | undefined,
    choiceIndex?: number,
    cardStorageTargets?: string[],
    targetPlayerId?: string,
    selectedAmount?: number,
    cardStorageSources?: string[],
  ): Promise<string> {
    return this.sendWithPayment(MessageTypeActionPlayCard, "play-card", {
      type: "play-card",
      cardId,
      payment,
      ...(choiceIndex !== undefined && { choiceIndex }),
      ...(cardStorageTargets !== undefined && { cardStorageTargets }),
      ...(cardStorageSources !== undefined && { cardStorageSources }),
      ...(targetPlayerId !== undefined && { targetPlayerId }),
      ...(selectedAmount !== undefined && { selectedAmount }),
    });
  }

  playCardAction(
    cardId: string,
    behaviorIndex: number,
    choiceIndex?: number,
    cardStorageTargets?: string[],
    targetPlayerId?: string,
    sourceCardForInput?: string,
    selectedAmount?: number,
    payment?: PaymentDto,
    reuseSourceCardId?: string,
    cardStorageSources?: string[],
  ): Promise<string> {
    return this.sendWithPayment(MessageTypeActionCardAction, "card-action", {
      type: "card-action",
      cardId,
      behaviorIndex,
      ...(choiceIndex !== undefined && { choiceIndex }),
      ...(cardStorageTargets !== undefined && { cardStorageTargets }),
      ...(cardStorageSources !== undefined && { cardStorageSources }),
      ...(targetPlayerId !== undefined && { targetPlayerId }),
      ...(sourceCardForInput !== undefined && { sourceCardForInput }),
      ...(selectedAmount !== undefined && { selectedAmount }),
      ...(payment !== undefined && { payment }),
      ...(reuseSourceCardId !== undefined && { reuseSourceCardId }),
    });
  }

  selectStartingChoices(
    corporationId: string,
    preludeIds: string[],
    cardIds: string[],
  ): Promise<string> {
    return this.sendWithPayment(MessageTypeActionSelectStartingChoices, "select-starting-choices", {
      corporationId,
      preludeIds,
      cardIds,
    });
  }

  confirmInitAdvance(): string {
    return this.send(MessageTypeActionConfirmInitAdvance, {});
  }

  selectCards(cardIds: string[]): string {
    return this.send(MessageTypeActionConfirmSellPatents, {
      selectedCardIds: cardIds,
    });
  }

  confirmProductionCards(cardIds: string[], options?: { randomBuy?: boolean }): Promise<string> {
    return this.sendWithPayment(
      MessageTypeActionConfirmProductionCards,
      "confirm-production-cards",
      {
        cardIds,
        randomBuy: options?.randomBuy ?? false,
      },
    );
  }

  acknowledgeCardReceipt(receiptId: string): string {
    return this.send("action.acknowledge-card-receipt", { receiptId });
  }

  confirmCardDraw(cardsToTake: string[], cardsToBuy: string[]): Promise<string> {
    return this.sendWithPayment(MessageTypeActionCardDrawConfirmed, "confirm-card-draw", {
      cardsToTake,
      cardsToBuy,
    });
  }

  selectTile(coordinate: { q: number; r: number; s: number }): string {
    const hex = `${coordinate.q},${coordinate.r},${coordinate.s}`;
    return this.send(MessageTypeActionTileSelected, { hex });
  }

  selectDemoChoices(request: SelectDemoChoicesRequest): string {
    return this.send(MessageTypeActionSelectDemoChoices, request);
  }

  claimMilestone(milestoneType: string): Promise<string> {
    return this.sendWithPayment(MessageTypeActionClaimMilestone, "claim-milestone", {
      milestoneType,
    });
  }

  fundAward(awardType: string): Promise<string> {
    return this.sendWithPayment(MessageTypeActionFundAward, "fund-award", { awardType });
  }

  tradeWithColony(colonyId: string, paymentType: string, trackSteps: number): Promise<string> {
    return this.sendWithPayment(MessageTypeActionColonyTrade, "colony-trade", {
      colonyId,
      paymentType,
      trackSteps,
    });
  }

  buildColony(colonyId: string): Promise<string> {
    return this.sendWithPayment(MessageTypeActionColonyBuild, "build-colony", { colonyId });
  }

  buyProjectSeat(projectId: string, credits: number, steel: number, titanium: number): string {
    return this.send(MessageTypeActionProjectFundingSeat, {
      projectId,
      credits,
      steel,
      titanium,
    });
  }

  resumeCommand(type: MessageType, request: ResumeGameRequest): void {
    this.currentGameId = request.gameId;
    this.send(type, request, request.gameId);
  }

  playerTakeover(targetPlayerId: string, gameId: string): void {
    this.send("player-takeover" as MessageType, { targetPlayerId, gameId }, gameId);
    this.currentGameId = gameId;
  }

  confirmCardDiscard(resolutionId: string, cardsToDiscard: string[]): string {
    return this.send(MessageTypeActionCardDiscardConfirmed, { resolutionId, cardsToDiscard });
  }

  confirmOptionalBehavior(resolutionId: string, accept: boolean): string {
    return this.send(MessageTypeActionConfirmOptionalBehavior, { resolutionId, accept });
  }

  confirmBehaviorChoice(
    resolutionId: string,
    choiceIndex: number,
    cardStorageTargets?: string[],
  ): string {
    return this.send(MessageTypeActionBehaviorChoiceConfirmed, {
      resolutionId,
      choiceIndex,
      ...(cardStorageTargets !== undefined && { cardStorageTargets }),
    });
  }

  confirmResourceRemoval(selectionId: string, targetPlayerId: string, amount: number): string {
    return this.send(MessageTypeActionConfirmResourceRemoval, {
      selectionId,
      targetPlayerId,
      amount,
    });
  }

  confirmColonyResource(cardId: string): string {
    return this.send(MessageTypeActionConfirmColonyResource, { cardId });
  }

  continueSelection(
    requestType:
      | "action.confirm-colony-placement"
      | "action.confirm-free-trade"
      | "action.confirm-effect-selection"
      | "action.confirm-award-fund",
  ): string {
    return this.send(requestType, { skip: true });
  }

  confirmColonyPlacement(colonyId: string): string {
    return this.send(MessageTypeActionConfirmColonyPlacement, { colonyId });
  }

  confirmCardReveal(): string {
    return this.send(MessageTypeActionConfirmCardReveal, {});
  }

  confirmEffectSelection(optionIndex: number): string {
    return this.send(MessageTypeActionConfirmEffectSelection, { optionIndex });
  }

  confirmFreeTrade(colonyId: string, trackSteps: number): string {
    return this.send(MessageTypeActionConfirmFreeTrade, { colonyId, trackSteps });
  }

  confirmAwardFund(awardType: string): string {
    return this.send(MessageTypeActionConfirmAwardFund, { awardType });
  }

  addBot(): string {
    return this.send(MessageTypeAddBot, {});
  }

  retryBot(playerId: string): string {
    return this.send(MessageTypeBotRetry, { playerId });
  }

  inspectBot(playerId: string): string {
    return this.send(MessageTypeBotInspect, { playerId });
  }

  sendEmote(emote: EmoteSendPayload["emote"]): string {
    return this.send(MessageTypeEmoteSend, { emote });
  }

  kickPlayer(targetPlayerId: string): string {
    return this.send(MessageTypeKickPlayer, { targetPlayerId });
  }

  endGame(): string {
    return this.send(MessageTypeEndGame, {});
  }

  convertToBot(targetPlayerId: string): string {
    return this.send(MessageTypeConvertToBot, { targetPlayerId });
  }

  requestLogs(): void {
    this.send(MessageTypeRequestLogs, {});
  }

  updateGameSettings(patch: UpdateGameSettingsRequest): void {
    this.send(MessageTypeUpdateGameSettings, patch);
  }

  setPlayerColor(color: string, targetPlayerId?: string): void {
    this.send(MessageTypeSetPlayerColor, { color, targetPlayerId });
  }

  spectatorConnect(spectatorName: string, gameId: string): void {
    this.send(MessageTypeSpectatorConnect, { spectatorName, gameId }, gameId);
    this.currentGameId = gameId;
  }

  sendChatMessage(message: string): string {
    return this.send(MessageTypeChatMessage, { message });
  }

  kickSpectator(targetSpectatorId: string): string {
    return this.send(MessageTypeKickSpectator, { targetSpectatorId });
  }

  on(event: string, callback: EventCallback) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(callback);
  }

  off(event: string, callback: EventCallback) {
    if (this.listeners[event]) {
      this.listeners[event] = this.listeners[event].filter((cb) => cb !== callback);
    }
  }

  private emit(event: string, data?: unknown) {
    if (this.listeners[event]) {
      this.listeners[event].forEach((callback) => {
        try {
          callback(data);
        } catch (error) {
          console.error(`Error in event listener for ${event}:`, error);
        }
      });
    }
  }

  private attemptReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;

      setTimeout(() => {
        this.connect().catch((error) => {
          console.error("Reconnection failed:", error);
        });
      }, this.reconnectDelay * this.reconnectAttempts);
    } else {
      console.error("Max reconnection attempts reached");
      this.emit("max-reconnects-reached");
    }
  }

  disconnect() {
    this.shouldReconnect = false;
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
    this.currentGameId = null;
  }

  get connected() {
    return this.isConnected;
  }

  get gameId() {
    return this.currentGameId;
  }
}

// Singleton instance for application-wide use
export const webSocketService = new WebSocketService();
