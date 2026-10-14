import { webSocketService } from "./webSocketService.ts";
import { WebSocketConnection } from "../types/webSocketTypes.ts";
import type {
  ResumeGameRequest,
  MessageType,
  PaymentDto,
  ChatMessageDto,
  SelectDemoChoicesRequest,
  GameDto,
  LogUpdatePayload,
  UpdateGameSettingsRequest,
  EmotePayload,
  EmoteSendPayload,
  BotThoughtPayload,
  BotTraceDto,
  BotTraceEventDto,
} from "../types/generated/api-types.ts";

class GlobalWebSocketManager implements WebSocketConnection {
  private isInitialized = false;
  private initializationPromise: Promise<void> | null = null;
  private currentPlayerId: string | null = null;
  private eventCallbacks: { [event: string]: ((data: any) => void)[] } = {};
  private isIntentionalDisconnect = false;
  private handlersSetUp = false;

  async initialize() {
    if (this.isInitialized) {
      return;
    }

    if (this.initializationPromise) {
      return this.initializationPromise;
    }

    this.isIntentionalDisconnect = false;
    this.initializationPromise = this._doInitialize();

    try {
      await this.initializationPromise;
    } finally {
      this.initializationPromise = null;
    }
  }

  private async _doInitialize() {
    try {
      await webSocketService.connect();
      this.setupGlobalEventHandlers();
      this.isInitialized = true;
    } catch (error) {
      console.error("Failed to initialize global WebSocket connection:", error);
      throw error;
    }
  }

  async ensureConnected() {
    if (!this.isInitialized) {
      await this.initialize();
    }

    if (!webSocketService.connected) {
      return new Promise<void>((resolve, reject) => {
        const checkConnection = () => {
          if (webSocketService.connected) {
            resolve();
          } else {
            setTimeout(checkConnection, 100);
          }
        };

        checkConnection();

        setTimeout(() => {
          reject(new Error("WebSocket connection timeout"));
        }, 10000);
      });
    }
  }

  private setupGlobalEventHandlers() {
    if (this.handlersSetUp) {
      return;
    }
    this.handlersSetUp = true;

    webSocketService.on("game-updated", (updatedGame: GameDto) => {
      this.emit("game-updated", updatedGame);
    });

    webSocketService.on("player-kicked", (payload: any) => {
      this.emit("player-kicked", payload);
    });

    webSocketService.on("game-ended", (payload: any) => {
      this.emit("game-ended", payload);
    });

    webSocketService.on("log-update", (payload: LogUpdatePayload) => {
      this.emit("log-update", payload);
    });

    webSocketService.on("available-cards", (payload: any) => {
      this.emit("available-cards", payload);
    });

    webSocketService.on("spectator-connected", (payload: any) => {
      this.emit("spectator-connected", payload);
    });

    webSocketService.on("chat-update", (chatMessage: ChatMessageDto) => {
      this.emit("chat-update", chatMessage);
    });

    webSocketService.on("spectator-kicked", (payload: any) => {
      this.emit("spectator-kicked", payload);
    });

    webSocketService.on("emote", (payload: EmotePayload) => {
      this.emit("emote", payload);
    });

    webSocketService.on("bot-thought", (payload: BotThoughtPayload) => {
      this.emit("bot-thought", payload);
    });

    webSocketService.on("bot-trace-snapshot", (payload: BotTraceDto) => {
      this.emit("bot-trace-snapshot", payload);
    });

    webSocketService.on("bot-trace-event", (payload: BotTraceEventDto) => {
      this.emit("bot-trace-event", payload);
    });

    webSocketService.on("error", (error: any) => {
      console.error("WebSocket error:", error);
      this.emit("error", error);
    });

    webSocketService.on("disconnect", () => {
      this.emit("disconnect");
    });

    webSocketService.on("connect", () => {
      this.emit("connect");
    });

    webSocketService.on("max-reconnects-reached", () => {
      this.emit("max-reconnects-reached");
    });
  }

  setCurrentPlayerId(playerId: string) {
    this.currentPlayerId = playerId;
  }

  getCurrentPlayerId(): string | null {
    return this.currentPlayerId;
  }

  on(event: string, callback: (data: any) => void) {
    if (!this.eventCallbacks[event]) {
      this.eventCallbacks[event] = [];
    }
    this.eventCallbacks[event].push(callback);
  }

  off(event: string, callback: (data: any) => void) {
    if (this.eventCallbacks[event]) {
      this.eventCallbacks[event] = this.eventCallbacks[event].filter((cb) => cb !== callback);
    }
  }

  private emit(event: string, data?: any) {
    if (this.eventCallbacks[event]) {
      this.eventCallbacks[event].forEach((callback) => {
        try {
          callback(data);
        } catch (error) {
          console.error(`Error in WebSocket event callback for ${event}:`, error);
        }
      });
    }
  }

  async playerConnect(playerName: string, gameId: string, playerId?: string) {
    if (this.isInitialized && webSocketService.gameId && webSocketService.gameId !== gameId) {
      this.disconnect();
    }
    await this.ensureConnected();
    return webSocketService.playerConnect(playerName, gameId, playerId);
  }

  async standardProject(projectId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.standardProject(projectId);
  }

  async convertPlantsToGreenery(payment?: PaymentDto): Promise<string> {
    await this.ensureConnected();
    return webSocketService.convertPlantsToGreenery(payment);
  }

  async convertHeatToTemperature(payment?: PaymentDto): Promise<string> {
    await this.ensureConnected();
    return webSocketService.convertHeatToTemperature(payment);
  }

  async startGame(): Promise<string> {
    await this.ensureConnected();
    return webSocketService.startGame();
  }

  async skipAction(): Promise<string> {
    await this.ensureConnected();
    return webSocketService.skipAction();
  }

  async playCard(
    cardId: string,
    payment: PaymentDto | undefined,
    choiceIndex?: number,
    cardStorageTargets?: string[],
    targetPlayerId?: string,
    selectedAmount?: number,
    cardStorageSources?: string[],
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.playCard(
      cardId,
      payment,
      choiceIndex,
      cardStorageTargets,
      targetPlayerId,
      selectedAmount,
      cardStorageSources,
    );
  }

  async playCardAction(
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
    await this.ensureConnected();
    return webSocketService.playCardAction(
      cardId,
      behaviorIndex,
      choiceIndex,
      cardStorageTargets,
      targetPlayerId,
      sourceCardForInput,
      selectedAmount,
      payment,
      reuseSourceCardId,
      cardStorageSources,
    );
  }

  async selectStartingChoices(
    corporationId: string,
    preludeIds: string[],
    cardIds: string[],
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.selectStartingChoices(corporationId, preludeIds, cardIds);
  }

  async confirmInitAdvance(): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmInitAdvance();
  }

  async selectCards(cardIds: string[]): Promise<string> {
    await this.ensureConnected();
    return webSocketService.selectCards(cardIds);
  }

  async confirmProductionCards(
    cardIds: string[],
    options?: { randomBuy?: boolean },
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmProductionCards(cardIds, options);
  }

  async acknowledgeCardReceipt(receiptId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.acknowledgeCardReceipt(receiptId);
  }

  async confirmCardDraw(cardsToTake: string[], cardsToBuy: string[]): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmCardDraw(cardsToTake, cardsToBuy);
  }

  async selectTile(coordinate: { q: number; r: number; s: number }): Promise<string> {
    await this.ensureConnected();
    return webSocketService.selectTile(coordinate);
  }

  async selectDemoChoices(request: SelectDemoChoicesRequest): Promise<string> {
    await this.ensureConnected();
    return webSocketService.selectDemoChoices(request);
  }

  async sendAdminCommand(adminRequest: any): Promise<string> {
    await this.ensureConnected();
    const { MessageTypeAdminCommand } = await import("../types/generated/api-types.ts");
    return webSocketService.send(MessageTypeAdminCommand, adminRequest);
  }

  async resumeCommand(type: MessageType, request: ResumeGameRequest): Promise<void> {
    await this.ensureConnected();
    webSocketService.resumeCommand(type, request);
  }

  async playerTakeover(targetPlayerId: string, gameId: string): Promise<void> {
    await this.ensureConnected();
    return webSocketService.playerTakeover(targetPlayerId, gameId);
  }

  async confirmCardDiscard(resolutionId: string, cardsToDiscard: string[]): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmCardDiscard(resolutionId, cardsToDiscard);
  }

  async confirmOptionalBehavior(resolutionId: string, accept: boolean): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmOptionalBehavior(resolutionId, accept);
  }

  async confirmBehaviorChoice(
    resolutionId: string,
    choiceIndex: number,
    cardStorageTargets?: string[],
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmBehaviorChoice(resolutionId, choiceIndex, cardStorageTargets);
  }

  async confirmResourceRemoval(
    selectionId: string,
    targetPlayerId: string,
    amount: number,
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmResourceRemoval(selectionId, targetPlayerId, amount);
  }

  async confirmColonyResource(cardId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmColonyResource(cardId);
  }

  async continueSelection(
    requestType: Parameters<typeof webSocketService.continueSelection>[0],
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.continueSelection(requestType);
  }

  async confirmColonyPlacement(colonyId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmColonyPlacement(colonyId);
  }

  async confirmCardReveal(): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmCardReveal();
  }

  async confirmEffectSelection(optionIndex: number): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmEffectSelection(optionIndex);
  }

  async confirmFreeTrade(colonyId: string, trackSteps: number): Promise<string> {
    await this.ensureConnected();
    return webSocketService.confirmFreeTrade(colonyId, trackSteps);
  }

  async tradeWithColony(
    colonyId: string,
    paymentType: string,
    trackSteps: number,
  ): Promise<string> {
    await this.ensureConnected();
    return webSocketService.tradeWithColony(colonyId, paymentType, trackSteps);
  }

  async buildColony(colonyId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.buildColony(colonyId);
  }

  async addBot(): Promise<string> {
    await this.ensureConnected();
    return webSocketService.addBot();
  }

  async retryBot(playerId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.retryBot(playerId);
  }

  async inspectBot(playerId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.inspectBot(playerId);
  }

  async sendEmote(emote: EmoteSendPayload["emote"]): Promise<string> {
    await this.ensureConnected();
    return webSocketService.sendEmote(emote);
  }

  async kickPlayer(targetPlayerId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.kickPlayer(targetPlayerId);
  }

  async endGame(): Promise<string> {
    await this.ensureConnected();
    return webSocketService.endGame();
  }

  async convertToBot(targetPlayerId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.convertToBot(targetPlayerId);
  }

  async requestLogs(): Promise<void> {
    await this.ensureConnected();
    webSocketService.requestLogs();
  }

  async updateGameSettings(patch: UpdateGameSettingsRequest): Promise<void> {
    await this.ensureConnected();
    webSocketService.updateGameSettings(patch);
  }

  async setPlayerColor(color: string, targetPlayerId?: string): Promise<void> {
    await this.ensureConnected();
    webSocketService.setPlayerColor(color, targetPlayerId);
  }

  async spectatorConnect(spectatorName: string, gameId: string): Promise<void> {
    if (this.isInitialized) {
      this.disconnect();
    }
    await this.ensureConnected();
    webSocketService.spectatorConnect(spectatorName, gameId);
  }

  async sendChatMessage(message: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.sendChatMessage(message);
  }

  async kickSpectator(targetSpectatorId: string): Promise<string> {
    await this.ensureConnected();
    return webSocketService.kickSpectator(targetSpectatorId);
  }

  get connected() {
    return webSocketService.connected;
  }

  get gameId() {
    return webSocketService.gameId;
  }

  disconnect() {
    this.isIntentionalDisconnect = true;
    webSocketService.disconnect();
    this.isInitialized = false;
    this.currentPlayerId = null;
  }

  isGracefulDisconnect(): boolean {
    return this.isIntentionalDisconnect;
  }
}

// Singleton instance - initialized once globally
export const globalWebSocketManager = new GlobalWebSocketManager();
