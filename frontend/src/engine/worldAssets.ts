import { assetUrl } from "@/assets";
import { AssetByteCache } from "./assetCache";
import { ASSET_CACHE_BUDGETS, TEXTURE_LIMITS } from "./graphics";
import { useGraphicsSettingsStore } from "../stores/graphicsSettingsStore";
import {
  rendererAssets,
  MAX_ASSET_FILE_BYTES,
  DISPLAY_SKY_FILE_BYTES,
  type AssetDemand,
} from "./scene.generated";

const downloads = new AssetByteCache();

export function invalidateWorldAssets() {
  downloads.clear();
}

/** Identities and prepared files come from the existing catalog, never copied URLs. */
export async function loadWorldAssets(
  demand: AssetDemand,
  signal: AbortSignal,
  maxTextureSize: number,
  progress: (message: string, loaded: number, total: number | null) => void,
  install: (id: string, filename: string, bytes: Uint8Array) => void,
) {
  if (
    demand.assets.length > rendererAssets.length ||
    new Set(demand.assets.map((a) => a.id)).size !== demand.assets.length
  ) {
    throw new Error("Invalid renderer asset request");
  }
  const quality = useGraphicsSettingsStore.getState().settings.textureQuality;
  downloads.setCapacity(ASSET_CACHE_BUDGETS[quality]);
  const textureLimit = TEXTURE_LIMITS[quality];
  const assets = demand.assets.map((requested) => {
    const requirement = rendererAssets.find(({ id }) => id === requested.id);
    if (!requirement) throw new Error("Unknown renderer asset request");
    const { id } = requirement;
    let pixels =
      requirement.pixels > 0
        ? Math.min(requirement.pixels, maxTextureSize)
        : Math.min(maxTextureSize, 4096);
    if (id.startsWith("textures/planets/")) {
      pixels = Math.min(pixels, textureLimit);
    }
    if (id === "textures/skyboxes/starmap") {
      pixels = Math.min(maxTextureSize, textureLimit / 2);
    }
    if (requirement.arrayLayers > 0) {
      pixels = Math.min(maxTextureSize, textureLimit / 4);
    }
    return {
      id,
      cache: requirement.cache,
      limit: id === "textures/skyboxes/starmap" ? DISPLAY_SKY_FILE_BYTES : MAX_ASSET_FILE_BYTES,
      url: assetUrl(id, pixels),
    };
  });
  const amounts = assets.map(() => ({ loaded: 0, total: null as number | null }));
  for (const [index, asset] of assets.entries()) {
    const bytes = await downloads.load(
      asset.url,
      signal,
      asset.cache,
      (loaded, total) => {
        amounts[index] = { loaded, total };
        progress(
          `Loading world assets ${index + 1}/${assets.length}`,
          amounts.reduce((sum, item) => sum + item.loaded, 0),
          amounts.every((item) => item.total !== null)
            ? amounts.reduce((sum, item) => sum + item.total!, 0)
            : null,
        );
      },
      asset.limit,
    );
    signal.throwIfAborted();
    install(asset.id, new URL(asset.url).pathname.split("/").at(-1)!, bytes);
  }
}
