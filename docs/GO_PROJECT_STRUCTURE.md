# Tanabata File Manager — Go Project Structure

> Backend counterpart of [ARCHITECTURE.md](ARCHITECTURE.md). This document
> details the Go layout, the layer rules, and the key backend decisions.

## Stack

- **Router**: Gin
- **Database**: pgx v5 (pgxpool)
- **Migrations**: goose v3 + `go:embed` (auto-applied on startup)
- **Auth**: JWT (golang-jwt/jwt/v5), Bearer access tokens + rotating refresh tokens
- **Config**: environment variables via `.env` (joho/godotenv)
- **Logging**: slog (stdlib)
- **Metadata**: exiftool (external, preferred) with a pure-Go EXIF fallback
  (rwcarlsen/goexif)
- **Thumbnails / previews**: vipsthumbnail (external, shrink-on-load) and ffmpeg
  (video frames), with a pure-Go fallback (disintegration/imaging)
- **Near-duplicate detection**: 64-bit dHash perceptual hashing + a BK-tree /
  Hamming-distance pairing (`internal/imagehash`, `internal/service/duplicate_*`)
- **Architecture**: Clean Architecture (domain → service → repository/handler)

The binary is fully static (`CGO_ENABLED=0`). External tools are invoked as
subprocesses when present and are optional — the pure-Go paths keep the server
working without them.

## Monorepo Layout

```
tanabata/
├── backend/                            ← Go project
├── frontend/                           ← SvelteKit project
├── openapi.yaml                        ← Shared API contract
├── docker-compose.yml
├── Dockerfile
├── .env.example
└── README.md
```

## Backend Directory Layout

```
backend/
├── cmd/
│   ├── server/
│   │   └── main.go                     # Entrypoint: config → DB → migrate → bootstrap admin → wire → serve
│   └── dedup/
│       └── main.go                     # Offline maintenance CLI: perceptual-hash backfill + duplicate-pairs rescan
│
├── internal/
│   │
│   ├── domain/                         # Pure business entities & value objects (stdlib only)
│   │   ├── file.go                     # File, FileFilter, FileListParams, FilePage
│   │   ├── tag.go                      # Tag, TagRule
│   │   ├── category.go                 # Category
│   │   ├── pool.go                     # Pool, PoolFile
│   │   ├── user.go                     # User, Session
│   │   ├── acl.go                      # Permission, ObjectType
│   │   ├── audit.go                    # AuditEntry, ActionType
│   │   ├── duplicate.go                # DuplicatePair, PHashEntry
│   │   ├── context.go                  # WithUser / UserFromContext (identity + session in ctx)
│   │   └── errors.go                   # Domain error types (ErrNotFound, ErrForbidden, …)
│   │
│   ├── port/                           # Interfaces (ports) — contracts between layers
│   │   ├── repository.go               # Transactor, FileRepo, TagRepo, TagRuleRepo, CategoryRepo,
│   │   │                               # PoolRepo, UserRepo, SessionRepo, ACLRepo, AuditRepo,
│   │   │                               # MimeRepo, DuplicatePairRepo, DismissalRepo
│   │   └── storage.go                  # FileStorage (originals + thumbnail/preview cache)
│   │
│   ├── service/                        # Business logic (use cases)
│   │   ├── file_service.go             # Upload, update, delete, trash/restore, replace, import, filter/list
│   │   ├── tag_service.go              # CRUD + auto-tag (rule) application
│   │   ├── category_service.go         # CRUD (thin: repo + ACL + audit)
│   │   ├── pool_service.go             # CRUD + file ordering, add/remove files
│   │   ├── auth_service.go             # Login, logout, JWT issue/refresh, content tokens, sessions
│   │   ├── acl_service.go              # Permission checks, grant/revoke
│   │   ├── audit_service.go            # Log actions, query audit log
│   │   ├── user_service.go             # Profile update, admin CRUD, block/unblock, EnsureAdmin
│   │   ├── duplicate_service.go        # Cluster / resolve (merge) / dismiss + rescan orchestration
│   │   ├── duplicate_index.go          # BK-tree, Hamming pairing, connected-component clustering
│   │   └── metadata.go                 # EXIF / media metadata extraction (exiftool + pure-Go fallback)
│   │
│   ├── handler/                        # HTTP layer (Gin handlers)
│   │   ├── router.go                   # Route registration, middleware, security headers, SPA fallback
│   │   ├── middleware.go               # Auth middleware (JWT / content token → context)
│   │   ├── ratelimit.go                # Per-IP token-bucket limiter for /auth
│   │   ├── response.go                 # Error/success builders, domain-error → HTTP mapping
│   │   ├── static.go                   # Built SPA serving + index.html fallback
│   │   ├── file_handler.go             # /files endpoints
│   │   ├── duplicate_handler.go        # /files/duplicates endpoints
│   │   ├── tag_handler.go              # /tags endpoints (+ file–tag relations)
│   │   ├── category_handler.go         # /categories endpoints
│   │   ├── pool_handler.go             # /pools endpoints
│   │   ├── auth_handler.go             # /auth endpoints
│   │   ├── acl_handler.go              # /acl endpoints
│   │   ├── user_handler.go             # /users endpoints
│   │   └── audit_handler.go            # /audit endpoint
│   │
│   ├── db/                             # Database adapters
│   │   ├── db.go                       # Shared helpers: Querier, tx-from-context, ScanRow, limit/offset clamps
│   │   └── postgres/                   # PostgreSQL implementation
│   │       ├── postgres.go             # pgxpool init, Transactor, conn-or-tx helper
│   │       ├── file_repo.go            # FileRepo (incl. perceptual-hash projections)
│   │       ├── tag_repo.go             # TagRepo + TagRuleRepo
│   │       ├── category_repo.go        # CategoryRepo
│   │       ├── pool_repo.go            # PoolRepo
│   │       ├── user_repo.go            # UserRepo
│   │       ├── session_repo.go         # SessionRepo
│   │       ├── acl_repo.go             # ACLRepo
│   │       ├── audit_repo.go           # AuditRepo
│   │       ├── mime_repo.go            # MimeRepo
│   │       ├── duplicate_repo.go       # DuplicatePairRepo + DismissalRepo
│   │       └── filter_parser.go        # Filter DSL → SQL WHERE clause builder
│   │
│   ├── storage/                        # File storage adapter
│   │   └── disk.go                     # FileStorage on disk: originals + thumbnail/preview cache
│   │                                   # (vipsthumbnail / ffmpeg / pure-Go imaging)
│   │
│   ├── imagehash/                      # Perceptual hashing (64-bit dHash) for near-duplicate detection
│   │   └── imagehash.go
│   │
│   ├── integration/                    # End-to-end HTTP tests against a disposable Postgres
│   │   └── server_test.go
│   │
│   └── config/                         # Configuration
│       └── config.go                   # Config struct + loader from env vars
│
├── migrations/                         # SQL migration files (goose format), embedded via go:embed
│   ├── 001_init_schemas.sql
│   ├── 002_core_tables.sql
│   ├── 003_data_tables.sql
│   ├── 004_acl_tables.sql
│   ├── 005_activity_tables.sql
│   ├── 006_indexes.sql
│   ├── 007_seed_data.sql
│   └── embed.go                        # //go:embed *.sql → migrations.FS
│
├── go.mod
└── go.sum
```

## Layer Dependency Rules

```
handler  →  service  →  port (interfaces)  ←  db/postgres / storage
                ↓
              domain (entities, value objects, errors)
```

- **domain/**: zero imports from other internal packages. Only stdlib.
- **port/**: imports only domain/. Defines interfaces.
- **service/**: imports domain/ and port/. Never imports db/ or handler/.
- **handler/**: imports domain/ and service/. Never imports db/.
- **db/postgres/**: imports domain/, port/, and db/ (common helpers). Implements port interfaces.
- **db/**: imports domain/ and port/. Shared utilities for all DB adapters.
- **storage/**: imports domain/ and port/. Implements FileStorage.
- **imagehash/**: leaf package (stdlib + image libs); used by service/ and storage/.

No layer may import a layer above it. No circular dependencies.

## Key Design Decisions

### Dependency Injection (Wiring)

Manual wiring in `cmd/server/main.go`. No DI frameworks. Constructors take their
collaborators explicitly; the shape below matches the real signatures.

```go
// Pseudocode — see cmd/server/main.go for the exact calls.
pool := postgres.NewPool(ctx, cfg.DatabaseURL)
goose.Up(stdlib.OpenDBFromPool(pool), ".")   // migrations.FS embedded

// Storage
diskStorage := storage.NewDiskStorage(
    cfg.FilesPath, cfg.ThumbsCachePath,
    cfg.ThumbWidth, cfg.ThumbHeight, cfg.PreviewWidth, cfg.PreviewHeight,
    cfg.ThumbMaxPixels, cfg.ThumbConcurrency,
)

// Repos (all from internal/db/postgres/)
fileRepo := postgres.NewFileRepo(pool)
// … tag, tagRule, category, pool, user, session, acl, audit, mime,
//    duplicatePair, dismissal repos + transactor

// Services
authSvc := service.NewAuthService(userRepo, sessionRepo,
    cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL, cfg.ContentTokenTTL)
aclSvc  := service.NewACLService(aclRepo, fileRepo, tagRepo, categoryRepo, poolRepo, transactor)
auditSvc := service.NewAuditService(auditRepo)
tagSvc  := service.NewTagService(tagRepo, tagRuleRepo, aclSvc, auditSvc, transactor)
dupSvc  := service.NewDuplicateService(fileRepo, duplicatePairRepo, dismissalRepo,
    aclSvc, auditSvc, transactor, cfg.DuplicateHashThreshold)
fileSvc := service.NewFileService(fileRepo, mimeRepo, diskStorage,
    aclSvc, auditSvc, tagSvc, transactor, cfg.ImportPath)
// … category, pool, user services

// Bootstrap the initial admin from env (idempotent).
userSvc.EnsureAdmin(ctx, cfg.AdminUsername, cfg.AdminPassword)

// Handlers → router (also wires trusted proxies + optional static SPA dir)
router, _ := handler.NewRouter(authMiddleware, authHandler, fileHandler,
    duplicateHandler, tagHandler, categoryHandler, poolHandler,
    userHandler, aclHandler, auditHandler, cfg.StaticDir, cfg.TrustedProxies)
srv.ListenAndServe()
```

### Context Propagation

Every service method receives `context.Context` as the first argument.
The auth middleware parses the JWT and puts the caller's identity (user id,
admin flag, session id) into the context. Services read it for ACL checks and
audit logging.

```go
// handler/middleware.go
claims := parseJWT(c.GetHeader("Authorization"))
ctx := domain.WithUser(c.Request.Context(), claims.UserID, claims.IsAdmin, claims.SessionID)
c.Request = c.Request.WithContext(ctx)

// domain/context.go
func WithUser(ctx context.Context, userID int16, isAdmin bool, sessionID int) context.Context
func UserFromContext(ctx context.Context) (userID int16, isAdmin bool, sessionID int)
```

### Transaction Management

The `Transactor` port lets services compose multiple repo calls atomically.
The postgres implementation stores the active `pgx.Tx` in the context; repo
methods pick it up via a conn-or-tx helper, so the same method works inside or
outside a transaction.

```go
// port/repository.go
type Transactor interface {
    WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// service/file_service.go (sketch)
func (s *FileService) Upload(ctx context.Context, p UploadParams) (*domain.File, error) {
    return s.tx.WithTx(ctx, func(ctx context.Context) error {
        created, err := s.files.Create(ctx, f)   // uses tx from ctx
        // apply initial tags, etc., in the same tx
        return err
    })
}
```

### ACL Check Pattern

ACL logic is centralized in `ACLService`. Other services call it before any
mutation or retrieval. The model is private-by-default: admins see everything;
otherwise a `public` flag, creator ownership, or an explicit `acl.permissions`
grant is required.

```go
// service/acl_service.go (shape)
func (s *ACLService) CanView(ctx context.Context, userID int16, isAdmin bool,
    creatorID int16, isPublic bool, objectType int16, objectID uuid.UUID) (bool, error)
func (s *ACLService) CanEdit(ctx context.Context, userID int16, isAdmin bool,
    creatorID int16, objectType int16, objectID uuid.UUID) (bool, error)
```

### Error Mapping

Domain errors → HTTP status codes (handled in handler/response.go):

| Domain Error       | HTTP Status | Error Code       |
| ------------------ | ----------- | ---------------- |
| ErrNotFound        | 404         | not_found        |
| ErrForbidden       | 403         | forbidden        |
| ErrUnauthorized    | 401         | unauthorized     |
| ErrConflict        | 409         | conflict         |
| ErrValidation      | 400         | validation_error |
| ErrUnsupportedMIME | 415         | unsupported_mime |
| (unexpected)       | 500         | internal_error   |

### Filter DSL

The DSL parser lives in `db/postgres/filter_parser.go` because it produces SQL
WHERE clauses — a PostgreSQL-specific adapter concern. The service layer passes
the raw DSL string down; the repository parses it and builds the query. For a
different DBMS, a corresponding parser would live in `db/<dbms>/filter_parser.go`.

```go
// domain/file.go
type FileListParams struct {
    Filter    string    // raw DSL string
    Sort      string
    Order     string
    Cursor    string
    Anchor    *uuid.UUID
    Direction string    // "forward" or "backward"
    Limit     int
    Trash     bool
    Search    string
}
```

The DSL grammar itself is documented in `openapi.yaml` (the `filter` query
parameter), so the contract stays in one place.

### JWT Structure

```go
type Claims struct {
    jwt.RegisteredClaims
    UserID    int16 `json:"uid"`
    IsAdmin   bool  `json:"adm"`
    SessionID int   `json:"sid"`
}
```

Access token: short-lived (15 min default). Refresh token: long-lived (30 days
default), rotated on use, stored as a hash in `activity.sessions`. A separate
**content token** (default 6 h) is a single-file capability minted for media
URLs, so a long video keeps streaming past access-token expiry — see
`CONTENT_TOKEN_TTL` in `.env.example`.

### Perceptual Duplicate Detection

Images are dHash-ed inline on upload (`internal/imagehash`); video hashes are
backfilled by the `dedup` CLI (ffmpeg stays off the upload path). A rescan
rebuilds `data.duplicate_pairs` by inserting every pair within
`DUPLICATE_HASH_THRESHOLD` Hamming distance (BK-tree lookups, not O(N²)); the
duplicates API then groups pairs into connected-component clusters. See
`service/duplicate_service.go` and `service/duplicate_index.go`.

### Configuration (.env)

Every variable the server reads is documented in `.env.example` (1:1 with
`config.Config`). Required at startup: `JWT_SECRET`, `ADMIN_PASSWORD`,
`DATABASE_URL`, `FILES_PATH`, `THUMBS_CACHE_PATH`, `IMPORT_PATH`. Everything
else has a sensible default (see `config.go`).
