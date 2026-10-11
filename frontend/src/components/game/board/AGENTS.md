# Board rendering instructions

## Required visual review

Before completing a board rendering change, run `just frontend inspect --card ID` or `--tile TYPE` from the repository root and open the screenshots. Review gameplay and close cameras beside the comparison city, greenery, and ocean, on desktop plus iPhone and Android landscape.

For animation changes, capture placement stages such as `--state placement --frames 0,30,120`. Repeat captures after visual fixes. Judge grounding, scale, silhouette, texture detail, lighting, overlap, and smoke density in the real board scene.

An isolated model, custom preview lighting, geometry assertions, or successful screenshot generation does not replace this review. Report the images actually inspected and any verification gap. See [inspector instructions](../../../../scripts/README.md) for frame controls and reports.

## Shared structure and vegetation

- Use [structureLayout.ts](structureLayout.ts), [StructureRenderer](StructureRenderer.tsx), and [StructureBatchRenderer](StructureBatchRenderer.tsx) for built structures, including cities and factories.
- Author reusable feature requests through `style.tile` in [cards.json](../../../../../backend/assets/cards.json). Inspect the existing request vocabulary, geometry, materials, and placement logic before adding a feature.
- Extend the shared generator for missing features such as factory halls, smokestacks, or tanks. Do not introduce a renderer or fixed layout for one card, or select bespoke geometry by card ID/name.
- Keep gameplay tile types separate from visual composition: shared rendering must preserve each tile's placement, adjacency, and scoring rules.
- Place trees and bushes through [GreeneryRenderer](GreeneryRenderer.tsx), not standalone meshes in individual tile components. Preserve deterministic placement, stable slots/ranks, terrain grounding, and structure/volcano exclusions.

## Climate and landscape

Read the mutable climate values through `useClimate()` in `useFrame` and update uniforms. Do not trigger React renders every frame. Plant gating uses target state and stable ranks so rising gates add plants without reshuffling them.

When changing landscape rendering:

- Preserve shared terrain heights between ground and vegetation.
- Keep lake and Mohole stencil masks independent.
- Update bake invalidation in `LandscapeSurface.updateBake` whenever adding an input to the ground material. Include the corresponding material result in bake outputs and fall back to live shading immediately when inputs change.
- Preserve placement ordering: upload changed layers before activation, cover the union of previous/current basin cutouts during interpolation, and release removed slots after the transition.

See [LandscapeSurface](landscapeSurface.ts) and [groundBake](groundBake.ts) for current passes, uniforms, and timing rather than duplicating their constants.

## GPU and shader constraints

- Opaque ground writes actual surface depth; blended ground edges do not. Keep shared geometry and alpha behavior aligned between passes.
- Foliage depth and shading passes must use identical vertex transforms and polygon offsets, including `invariant gl_Position`, so equal-depth shading matches its prepass.
- Set `forceSinglePass: true` on transparent DoubleSide materials to avoid extra draws and program-update churn.
- Avoid allocations in `useFrame` and `onBeforeRender`, including new arrays, closures, and string-based color conversion.
- Check fragment texture-unit use before adding samplers. Add compatible terrain layers to the existing texture arrays instead of consuming another sampler; preserve their sampling orientation.
- Keep volcano shader height generation synchronized with CPU sampling in [volcanoFlowMap.ts](volcanoFlowMap.ts). Lava flow and rendered terrain must agree.
- Keep shader code in `.glsl` files loaded through the existing shader utilities; use uniform offsets rather than baking varying values into shader strings.

## React material lifetime

Keep `useMemo` material factories free of ref-assignment side effects. Strict Mode can invoke the factory more than once, leaving a ref pointing at a material that the mesh does not render. Update uniforms on the material returned by `useMemo`, rather than on a ref assigned inside that factory.

## Projection and assets

Use existing hex and sphere projection utilities: cube coordinates satisfy `q + r + s = 0`, and projection must use the target celestial body's radius. Reuse shared constants rather than copying numeric offsets or sizes.

Load models and textures through [useModels](../../../hooks/useModels.ts) and [useTextures](../../../hooks/useTextures.ts). Do not add direct `useGLTF`, `useTexture`, or `useLoader(TextureLoader)` calls to board components. Include new rendering paths in GPU warmup where needed to avoid first-use compilation stalls.
