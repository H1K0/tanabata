# Tanabata File Manager — Architecture

System-level overview of Tanabata File Manager (TFM). For the product-level
requirements see [REQUIREMENTS.md](REQUIREMENTS.md); for per-side detail see
[GO_PROJECT_STRUCTURE.md](GO_PROJECT_STRUCTURE.md) (backend) and
[FRONTEND_STRUCTURE.md](FRONTEND_STRUCTURE.md) (frontend). The full HTTP contract
lives in [`openapi.yaml`](../openapi.yaml).

## System Context

TFM is a multi-user, tag-based web file manager for images and video. It is a
single deployable unit — one Docker image that serves both the REST API and the
built single-page app on one port — plus a PostgreSQL database.

```
                        ┌───────────────────────────────────────────┐
   Browser / installed  │  Reverse proxy (nginx, TLS)               │
   PWA (desktop/mobile) │  host: 443 → 127.0.0.1:${APP_PORT}        │
        │  HTTPS        └─────────────────────┬─────────────────────┘
        └─────────────────────────────────────┼──────────────► 127.0.0.1:42776
                                              │
                    ┌─────────────────────────▼────────────────────────┐
                    │  Tanabata container (single image)               │
                    │                                                  │
                    │   Go server (Gin)                                │
                    │    ├─ /api/v1/*   REST API                       │
                    │    ├─ /health     liveness                       │
                    │    └─ /*          static SPA + index.html        │
                    │                     fallback                     │
                    │                                                  │
                    │   Disk: /data/files   (originals, name = UUID)   │
                    │         /data/thumbs  (thumbnail/preview cache)  │
                    │         /data/import  (server-side import drop)  │
                    └─────────────────────────┬────────────────────────┘
                                              │ pgx (private network)
                                    ┌─────────▼─────────┐
                                    │  PostgreSQL 14+   │
                                    │  (bundled or host)│
                                    └───────────────────┘
```

Optional companion process: a one-shot **dedup CLI** (same image, different
entrypoint) that backfills perceptual hashes and rebuilds the duplicate-pairs
table. It is not a daemon — it is run on demand.

## Components

| Component      | Tech                                                                       | Responsibility                                           |
| -------------- | -------------------------------------------------------------------------- | -------------------------------------------------------- |
| Frontend (SPA) | SvelteKit (adapter-static, `ssr=false`), Svelte 5, Tailwind v4, TypeScript | UI, client routing, PWA/offline, calls the REST API      |
| API server     | Go + Gin, Clean Architecture                                               | REST API, auth, ACL, business logic, thumbnailing, audit |
| Database       | PostgreSQL 14+ (pgx v5, goose)                                             | All structured data across 4 schemas / 19 tables         |
| File storage   | Local disk, flat, keyed by UUID                                            | Originals + a regenerable thumbnail/preview cache        |
| dedup CLI      | Go (same image)                                                            | Offline perceptual-hash backfill + pairs rescan          |
| Reverse proxy  | nginx (host, not shipped)                                                  | TLS termination, large-body/streaming config             |

## Backend Architecture (Clean Architecture)

Dependencies point inward; no layer imports a layer above it.

```
handler  →  service  →  port (interfaces)  ←  db/postgres, storage, imagehash
                ↓
              domain (entities, value objects, errors) — stdlib only
```

- **domain** — entities and errors, zero internal imports.
- **port** — interfaces (repositories, `FileStorage`, `Transactor`).
- **service** — use cases; the only place business rules live.
- **handler** — Gin HTTP layer; maps domain errors to HTTP status codes.
- **db/postgres**, **storage**, **imagehash** — adapters implementing the ports.

Wiring is manual in `cmd/server/main.go` (no DI framework). See
[GO_PROJECT_STRUCTURE.md](GO_PROJECT_STRUCTURE.md) for the file-by-file layout,
the transaction/context patterns, and the DI sketch.

## Request Flow (typical authenticated call)

1. The SPA sends `Authorization: Bearer <access token>` to `/api/v1/...`.
2. Gin middleware runs: security headers → (for `/auth`) per-IP rate limiter →
   auth middleware validates the JWT and puts `(userID, isAdmin, sessionID)`
   into the request context.
3. The handler parses/validates input and calls a service method
   (`ctx` first arg).
4. The service enforces ACL via `ACLService`, performs the use case — composing
   repository calls inside a `Transactor.WithTx` when several writes must be
   atomic — and writes an audit entry.
5. Repositories run SQL through pgx (pool or the tx carried in `ctx`).
6. The handler serializes the result; domain errors are mapped to
   `{ code, message, details? }` with the right HTTP status.

## Cross-Cutting Concerns

### Authentication & sessions

JWT bearer auth. A short-lived **access token** (15 min default) authorizes API
calls; a long-lived **refresh token** (30 days default) rotates on use and is
stored as a hash in `activity.sessions`. A separate **content token** (6 h
default) is a single-file capability embedded in media URLs so long video keeps
streaming past access-token expiry. The `/auth` endpoints are rate-limited per
client IP.

### Authorization (ACL)

Private-by-default. Admins see everything; otherwise access requires a `public`
flag, creator ownership, or an explicit grant in `acl.permissions` (read / edit).
All checks are centralized in `ACLService` and applied before reads and writes.

### File storage

Originals are stored flat under `FILES_PATH`, each named by its file UUID (no
directory tree, no original-name collisions). Thumbnails and previews are a
**regenerable cache** under `THUMBS_CACHE_PATH`: still images via vipsthumbnail
(shrink-on-load) with a pure-Go `imaging` fallback, video frames via ffmpeg;
metadata/EXIF via exiftool with a pure-Go fallback. Uploads are rejected unless
their sniffed MIME type is whitelisted in `core.mime_types`.

### Near-duplicate detection

Images are dHash-ed (64-bit perceptual hash) inline on upload; video hashes are
backfilled by the dedup CLI. A rescan rebuilds `data.duplicate_pairs` using a
BK-tree over Hamming distance (within `DUPLICATE_HASH_THRESHOLD`), and the API
groups pairs into connected-component clusters. Dismissed pairs are remembered so
they stop resurfacing. See the duplicate sections in
[GO_PROJECT_STRUCTURE.md](GO_PROJECT_STRUCTURE.md).

### Audit logging

User-visible actions (file/tag/category/pool CRUD, relations, ACL changes,
auth, session termination, admin user actions) are recorded in
`activity.audit_log` against a seeded set of action types.

### Frontend / PWA

Pure client-side SPA: static assets served by the Go binary, with `index.html`
as the fallback for client routes. Installable PWA with a service worker for
app-shell caching and optional offline viewing of pinned files. See
[FRONTEND_STRUCTURE.md](FRONTEND_STRUCTURE.md).

## Data Model

PostgreSQL, four schemas (see `backend/migrations/`):

- **core** — users, MIME whitelist, object types.
- **data** — categories, tags, tag rules, files, file–tag, pools, file–pool,
  duplicate pairs, duplicate dismissals.
- **acl** — per-object permission grants.
- **activity** — sessions, file/pool views, tag uses, audit log, action types.

Migrations are goose files embedded via `go:embed` and applied automatically on
server startup, so a fresh database bootstraps itself.

## Deployment

- **One image, one port.** The multi-stage `Dockerfile` builds the SPA (Node
  stage) and the static Go binary (Go stage), then ships an Alpine runtime with
  vips-tools / ffmpeg / exiftool and a non-root user. The server serves both the
  API and the SPA on port **42776** (the sum of the code points of 七夕).
- **Compose.** `docker-compose.yml` runs the app plus, optionally, a bundled
  PostgreSQL (`with-db` profile); a host Postgres is supported by leaving the
  profile empty. The app is published on loopback only and expects a host
  reverse proxy; the DB sits on a private `internal` network with no route
  off-host. The dedup CLI is a `tools`-profile, run-on-demand service.
- **Config.** All runtime config is environment variables, fully documented in
  [`.env.example`](../.env.example) (1:1 with `config.Config`). Secrets
  (`JWT_SECRET`, `ADMIN_PASSWORD`, `DATABASE_URL`) are never baked into the image.
- **First run.** Migrations auto-apply and the initial admin is bootstrapped
  from `ADMIN_USERNAME` / `ADMIN_PASSWORD`, so setup is: fill `.env`,
  `docker compose up`.

See [DEPLOY.md](DEPLOY.md) for the production deploy (Gitea Actions → host) and
the reverse-proxy notes in [README.md](../README.md).

## Design Constraints & Future Direction

- **DDD / Clean Architecture** on the server keeps business rules independent of
  Gin and pgx.
- **PostgreSQL-specific adapters are isolated** behind the `port` interfaces (the
  filter DSL → SQL translation lives in `db/postgres`), leaving room for other
  database engines in a future version without touching the service layer.
