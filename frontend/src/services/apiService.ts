import {
  GameSaveSummaryDto,
  ImportGameSaveResponse,
  GameOptionsDto,
  GameModeDto,
  GameSetupDto,
  FeedbackDto,
  FeedbackResponse,
  CreateGameResponse,
  GameDto,
  GetGameResponse,
  GetGameHistoryResponse,
  GameHistoryEntryDto,
  ListGamesResponse,
  ListCardsResponse,
  ListMilestonesAwardsResponse,
  MetaResponse,
  ChangelogResponse,
} from "../types/generated/api-types.ts";
import { config } from "../config";

const SAVE_MESSAGES: Record<string, string> = {
  invalid_json: "Invalid JSON",
  invalid_save: "Invalid file",
  save_too_large: "File too large (max 128 MiB)",
  incompatible_save: "Incompatible game version",
  forbidden: "Host only",
  game_not_found: "Game not found",
  seat_unavailable: "Seat unavailable",
  seat_taken: "Seat already taken",
  not_ready: "Players or bots not ready",
  invalid_name: "Invalid player name",
  invalid_token: "Invalid bot token",
  invalid_request: "Invalid request",
};

export function gameSaveErrorMessage(value: unknown, fallback: string): string {
  if (
    typeof value === "object" &&
    value !== null &&
    "code" in value &&
    typeof value.code === "string"
  ) {
    return SAVE_MESSAGES[value.code] ?? fallback;
  }
  return fallback;
}

async function saveRequest(url: string, init: RequestInit, fallback: string): Promise<Response> {
  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new Error("Connection failed. Try again.");
  }
  if (!response.ok) {
    const body: unknown = await response.json().catch(() => null);
    throw new Error(gameSaveErrorMessage(body, fallback));
  }
  return response;
}

export class ApiService {
  private baseUrl: string;

  constructor(baseUrl: string = config.apiUrl) {
    this.baseUrl = baseUrl;
  }

  async validateGameSave(raw: string, signal?: AbortSignal): Promise<GameSaveSummaryDto> {
    const response = await saveRequest(
      `${this.baseUrl}/game-saves/validate`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: raw,
        signal,
      },
      "Couldn’t check save. Try again.",
    );
    return response.json().catch(() => {
      throw new Error("Invalid server response");
    });
  }

  async importGameSave(
    raw: string,
    seatId: string,
    playerName: string,
    botToken: string,
  ): Promise<ImportGameSaveResponse> {
    const query = new URLSearchParams({ seatId, playerName });
    const response = await saveRequest(
      `${this.baseUrl}/game-saves/import?${query}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json", "X-Bot-Token": botToken },
        body: raw,
      },
      "Couldn’t load game. Try again.",
    );
    return response.json().catch(() => {
      throw new Error("Invalid server response");
    });
  }

  async downloadGameSave(gameId: string, playerId: string): Promise<void> {
    const query = new URLSearchParams({ playerId });
    const response = await saveRequest(
      `${this.baseUrl}/games/${encodeURIComponent(gameId)}/save?${query}`,
      { cache: "no-store" },
      "Couldn’t save game. Try again.",
    );
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download =
      response.headers.get("Content-Disposition")?.match(/filename="([^"]+)"/)?.[1] ??
      "openmars-game.json";
    anchor.click();
    URL.revokeObjectURL(url);
  }

  async getMeta(): Promise<MetaResponse> {
    const response = await fetch(`${this.baseUrl}/meta`, { cache: "no-store" });
    if (!response.ok) {
      throw new Error(`Could not load server info: ${response.status}`);
    }
    return response.json();
  }

  async getChangelog(signal?: AbortSignal): Promise<ChangelogResponse> {
    const response = await fetch(`${this.baseUrl}/changelog`, { cache: "no-store", signal });
    if (!response.ok) {
      throw new Error(`Could not load changelog: ${response.status}`);
    }
    return response.json();
  }

  changelogImageUrl(version: string, file: string): string {
    return `${this.baseUrl}/changelog/${encodeURIComponent(version)}/${encodeURIComponent(file)}`;
  }

  async getGameOptions(signal?: AbortSignal): Promise<GameOptionsDto> {
    const response = await fetch(`${this.baseUrl}/game-options`, { signal });
    if (!response.ok) {
      throw new Error("Could not load game options");
    }
    return response.json();
  }

  async getGameModes(signal?: AbortSignal): Promise<GameModeDto[]> {
    const response = await fetch(`${this.baseUrl}/game-modes`, { signal });
    if (!response.ok) {
      throw new Error("Could not load game modes");
    }
    const data: { gameModes: GameModeDto[] } = await response.json();
    return data.gameModes;
  }

  async createGame(settings: GameSetupDto): Promise<GameDto> {
    try {
      const response = await fetch(`${this.baseUrl}/games`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ settings }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const gameResponse: CreateGameResponse = await response.json();
      return gameResponse.game;
    } catch (error) {
      console.error("Failed to create game:", error);
      throw error;
    }
  }

  async getGame(gameId: string, playerId?: string): Promise<GameDto | null> {
    try {
      const url = new URL(`${this.baseUrl}/games/${gameId}`, window.location.origin);
      if (playerId) {
        url.searchParams.set("playerId", playerId);
      }

      const response = await fetch(url.toString());

      if (response.status === 404) {
        return null;
      }

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const gameResponse: GetGameResponse = await response.json();
      return gameResponse.game;
    } catch (error) {
      console.error("Failed to get game:", error);
      throw error;
    }
  }

  async listGames(status?: string): Promise<GameDto[]> {
    try {
      const url = new URL(`${this.baseUrl}/games`, window.location.origin);
      if (status) {
        url.searchParams.set("status", status);
      }

      const response = await fetch(url.toString());

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const data: ListGamesResponse = await response.json();
      return data.games || [];
    } catch (error) {
      console.error("Failed to list games:", error);
      throw error;
    }
  }

  async listCards(
    offset: number = 0,
    limit: number = 50,
    gameModeId = "",
    preview = "connected",
  ): Promise<ListCardsResponse> {
    try {
      const url = new URL(`${this.baseUrl}/cards`, window.location.origin);
      url.searchParams.set("offset", offset.toString());
      url.searchParams.set("limit", limit.toString());
      if (gameModeId) {
        url.searchParams.set("gameModeId", gameModeId);
        url.searchParams.set("preview", preview);
      }

      const response = await fetch(url.toString());

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const data: ListCardsResponse = await response.json();
      return data;
    } catch (error) {
      console.error("Failed to list cards:", error);
      throw error;
    }
  }

  async getGameHistory(
    gameId: string,
    options?: { phases?: string[]; policy?: string },
  ): Promise<GameHistoryEntryDto[]> {
    try {
      const url = new URL(`${this.baseUrl}/games/${gameId}/history`, window.location.origin);
      if (options?.phases && options.phases.length > 0) {
        url.searchParams.set("phases", options.phases.join(","));
      }
      if (options?.policy) {
        url.searchParams.set("policy", options.policy);
      }

      const response = await fetch(url.toString());

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const data: GetGameHistoryResponse = await response.json();
      return data.entries || [];
    } catch (error) {
      console.error("Failed to get game history:", error);
      throw error;
    }
  }

  async listMilestonesAndAwards(): Promise<ListMilestonesAwardsResponse> {
    try {
      const response = await fetch(`${this.baseUrl}/milestones-awards`);

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      console.error("Failed to list milestones and awards:", error);
      throw error;
    }
  }

  async getBugReportStatus(): Promise<{ available: boolean; reason?: string }> {
    try {
      const response = await fetch(`${this.baseUrl}/bugs/status`);

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      console.error("Failed to get bug report status:", error);
      throw error;
    }
  }

  async submitFeedback(request: {
    title: string;
    description: string;
    tags: string[];
    author?: string;
    gameState?: GameDto;
  }): Promise<FeedbackDto> {
    try {
      const response = await fetch(`${this.baseUrl}/bugs`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(
          errorData.message || errorData.error || `HTTP error! status: ${response.status}`,
        );
      }

      const data: FeedbackResponse = await response.json();
      return data.report;
    } catch (error) {
      console.error("Failed to submit feedback:", error);
      throw error;
    }
  }

  async getBugReport(id: string): Promise<FeedbackDto> {
    try {
      const response = await fetch(`${this.baseUrl}/bugs/${id}`);

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `HTTP error! status: ${response.status}`);
      }

      const data: FeedbackResponse = await response.json();
      return data.report;
    } catch (error) {
      console.error("Failed to get bug report:", error);
      throw error;
    }
  }
}

// Singleton instance
export const apiService = new ApiService();
