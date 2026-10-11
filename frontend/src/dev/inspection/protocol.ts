export interface InspectionBridge {
  ready: boolean;
  error: string | null;
  seek: (frame: number) => Promise<void>;
  setView: (view: "gameplay" | "close") => Promise<void>;
  report: () => object;
}
declare global {
  interface Window {
    __openMarsInspection?: InspectionBridge;
  }
}
