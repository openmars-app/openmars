import { useEffect, useRef, useState } from "react";
import type { MessageType } from "@/types/generated/api-types";
import type { ResolutionFailure } from "./useResolutionFailure";

// Sending a WebSocket message is not acknowledgement. Keep the prompt busy until
// its authoritative selection changes, or a matching rejection permits a retry.
export function usePendingSubmission(
  failure: ResolutionFailure | null | undefined,
  requestType: MessageType,
  selectionKey?: string,
) {
  const pending = useRef(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    pending.current = false;
    setSubmitting(false);
    setError(null);
  }, [selectionKey]);
  useEffect(() => {
    if (
      pending.current &&
      failure &&
      (failure.disconnected || failure.requestType === requestType)
    ) {
      pending.current = false;
      setSubmitting(false);
      setError(failure.message);
    }
  }, [failure, requestType]);
  const submit = async (action: () => unknown | Promise<unknown>) => {
    if (pending.current) return;
    pending.current = true;
    setSubmitting(true);
    setError(null);
    try {
      await action();
    } catch (reason) {
      pending.current = false;
      setSubmitting(false);
      setError(reason instanceof Error ? reason.message : "Could not confirm selection");
    }
  };
  return { submitting, error, submit };
}
