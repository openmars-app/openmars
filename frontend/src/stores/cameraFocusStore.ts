import { create } from "zustand";

export interface CameraFocusRequest {
  direction: [number, number, number];
  distance: number;
  instant?: boolean;
}

interface CameraFocusState {
  focusRequest: CameraFocusRequest | null;
  requestFocus: (request: CameraFocusRequest) => void;
  consume: () => CameraFocusRequest | null;
}

export const useCameraFocusStore = create<CameraFocusState>((set, get) => ({
  focusRequest: null,
  requestFocus: (request) => set({ focusRequest: request }),
  consume: () => {
    const request = get().focusRequest;
    if (request) {
      set({ focusRequest: null });
    }
    return request;
  },
}));
