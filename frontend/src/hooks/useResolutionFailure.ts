import { useEffect, useState } from "react";
import type { ErrorPayload, MessageType } from "@/types/generated/api-types";
import { webSocketService } from "@/services/webSocketService";

export interface ResolutionFailure {
  message: string;
  requestType?: MessageType;
  resolutionId?: string;
  selectionId?: string;
  disconnected?: boolean;
}

// Live transport stays in controllers; the corresponding views accept failure data.
export function useResolutionFailure(): ResolutionFailure | null {
  const [failure, setFailure] = useState<ResolutionFailure | null>(null);
  useEffect(() => {
    const failed = (payload: unknown) => {
      const result = payload as Partial<ErrorPayload>;
      setFailure({ ...result, message: result.message ?? "Could not resolve effect" });
    };
    const disconnected = () =>
      setFailure({
        disconnected: true,
        message: "Connection lost. Reconnect to check whether this effect was resolved.",
      });
    webSocketService.on("error", failed);
    webSocketService.on("disconnect", disconnected);
    return () => {
      webSocketService.off("error", failed);
      webSocketService.off("disconnect", disconnected);
    };
  }, []);
  return failure;
}
