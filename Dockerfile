# One image per game server: the Go server serves the API, the WebSocket and the
# built frontend. Build stages run natively on the build machine; only the
# runtime stage is per architecture.

FROM --platform=$BUILDPLATFORM oven/bun:1.4.2 AS deps
WORKDIR /build/frontend
COPY frontend/package.json frontend/bun.lock frontend/bunfig.toml ./
RUN bun install --frozen-lockfile

FROM --platform=$BUILDPLATFORM rust:1.99.1-bookworm AS engine-tools
WORKDIR /build/engine
COPY engine/Cargo.toml engine/Cargo.lock engine/rust-toolchain.toml ./
COPY engine/src/ ./src/
COPY engine/scene/ ./scene/
COPY engine/preparation/ ./preparation/
COPY engine/asset-pipeline/ ./asset-pipeline/
RUN cargo build --locked --release -p openmars-engine-assets

# Prepared UI and world assets share one catalog and content-addressed registry.
FROM deps AS assets
COPY frontend/scripts/assets/ ./scripts/assets/
COPY assets/ /build/assets/
COPY engine/asset-pipeline/ /build/engine/asset-pipeline/
COPY engine/Cargo.lock /build/engine/Cargo.lock
COPY engine/Cargo.toml engine/rust-toolchain.toml /build/engine/
COPY --from=engine-tools /build/engine/target/release/openmars-engine-assets /usr/local/bin/openmars-engine-assets
ENV OPENMARS_ASSET_TOOL=/usr/local/bin/openmars-engine-assets
RUN bun run assets

FROM engine-tools AS engine
RUN cargo install --locked --version 0.2.129 wasm-bindgen-cli && \
    for backend in webgpu webgl2; do \
      cargo build --locked --release --target wasm32-unknown-unknown --target-dir "target/$backend" --no-default-features --features "$backend" && \
      wasm-bindgen --target web --out-dir "pkg/$backend" --out-name openmars_engine "target/$backend/wasm32-unknown-unknown/release/openmars_engine.wasm" || exit 1; \
    done && \
    cargo build --locked --release -p openmars-terrain-worker --target wasm32-unknown-unknown --target-dir target/preparation && \
    wasm-bindgen --target web --out-dir pkg/preparation --out-name terrain_worker target/preparation/wasm32-unknown-unknown/release/openmars_terrain_worker.wasm

FROM deps AS web
ARG BUILD_VERSION="localbuild"
ENV VITE_APP_VERSION=${BUILD_VERSION}
COPY frontend/ ./
COPY --from=engine /build/engine/pkg/ /build/engine/pkg/
COPY --from=assets /build/frontend/public/ ./public/
COPY --from=assets /build/frontend/src/assets/generated/ ./src/assets/generated/
RUN bun run --bun vite build && \
    find build -type f \( -name '*.js' -o -name '*.css' -o -name '*.html' -o -name '*.json' \
        -o -name '*.svg' -o -name '*.map' -o -name '*.wasm' \) -exec gzip -k -9 {} +

FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS server
WORKDIR /build
RUN apk add --no-cache git ca-certificates tzdata
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
ARG BUILD_VERSION="localbuild"
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags="-w -s -X main.Version=${BUILD_VERSION}" \
    -o server \
    cmd/server/main.go

FROM alpine:latest
WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata bash curl libstdc++ libgcc && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

RUN curl -fsSL https://claude.ai/install.sh | bash && \
    chmod 755 /root && \
    ln -s /root/.local/bin/claude /usr/local/bin/claude

COPY --from=server /build/server .
COPY --from=server /build/assets ./assets
COPY --from=web /build/frontend/build ./web
COPY changelog/ ./changelog

# Source code for Claude bug report analysis
COPY backend/internal/ /repo/backend/internal/
COPY backend/cmd/ /repo/backend/cmd/
COPY backend/assets/ /repo/backend/assets/
COPY frontend/src/ /repo/frontend/src/

ENV OPENMARS_REPO_PATH=/repo
ENV SHELL=/bin/bash

RUN chown -R appuser:appuser /app /repo
USER appuser

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD addr="${OPENMARS_ADDR:-:3001}"; wget --quiet --tries=1 --output-document=/dev/null "http://localhost:${addr##*:}/api/v1/health" || exit 1

CMD ["./server"]
