# syntax=docker/dockerfile:1

# =============================================================================
# Tanabata File Manager — single-image build
#
# Produces one container that serves the SvelteKit SPA (built to static files)
# and the Go API on the same port. There is no Node runtime in the final image:
# the frontend uses adapter-static, so stage 1 emits plain HTML/CSS/JS that the
# Go binary serves directly (see STATIC_DIR).
# =============================================================================

# -----------------------------------------------------------------------------
# Stage 1 — build the frontend (static SPA)
# -----------------------------------------------------------------------------
FROM node:22-alpine AS frontend

WORKDIR /src/frontend

# Install dependencies first so this layer is cached unless the lockfile changes.
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# `npm run build` runs `generate:types`, which reads ../openapi.yaml relative to
# the frontend directory — place the spec one level up to match the repo layout.
COPY openapi.yaml /src/openapi.yaml
COPY frontend/ ./

RUN npm run build
# Output: /src/frontend/build (index.html, _app/, fonts, service-worker.js, …)

# -----------------------------------------------------------------------------
# Stage 2 — build the Go server (static binary)
# -----------------------------------------------------------------------------
FROM golang:1.26-alpine AS backend

WORKDIR /src/backend

# Download modules first so this layer is cached unless go.mod/go.sum changes.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

# CGO is disabled: image processing is pure Go (disintegration/imaging) and
# video thumbnails shell out to the ffmpeg binary at runtime, so the resulting
# binary is fully static and portable across base images.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# -----------------------------------------------------------------------------
# Stage 3 — minimal runtime
#
# Alpine (not distroless/scratch) because video thumbnailing invokes ffmpeg as
# an external process; it must be present on the runtime image.
# -----------------------------------------------------------------------------
FROM alpine:3.21 AS runtime

# ffmpeg: video frame extraction. ca-certificates/tzdata: TLS + time zones.
RUN apk add --no-cache ffmpeg ca-certificates tzdata

# Run as an unprivileged user.
RUN addgroup -S app && adduser -S -G app -u 10001 app

WORKDIR /app

# The built SPA, served by the Go binary (matches STATIC_DIR below).
COPY --from=frontend --chown=app:app /src/frontend/build /app/static
# The server binary.
COPY --from=backend --chown=app:app /out/server /app/server

# Data directories (overridable via FILES_PATH/THUMBS_CACHE_PATH/IMPORT_PATH).
# Created and owned by the app user so a fresh named volume inherits write access.
RUN mkdir -p /data/files /data/thumbs /data/import && chown -R app:app /data

# Non-secret defaults mirroring .env.example. Secrets (JWT_SECRET, ADMIN_PASSWORD,
# DATABASE_URL) are intentionally NOT baked in — pass them at `docker run`.
ENV LISTEN_ADDR=:42776 \
    STATIC_DIR=/app/static \
    FILES_PATH=/data/files \
    THUMBS_CACHE_PATH=/data/thumbs \
    IMPORT_PATH=/data/import

EXPOSE 42776
VOLUME ["/data"]
USER app

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:42776/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["/app/server"]
