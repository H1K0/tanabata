# Tanabata File Manager — Go Project Structure

## Stack

- **Router**: Gin
- **Database**: pgx v5 (pgxpool)
- **Migrations**: goose v3 + go:embed (auto-migrate on startup)
- **Auth**: JWT (golang-jwt/jwt/v5)
- **Config**: environment variables via .env (joho/godotenv)
- **Logging**: slog (stdlib, Go 1.21+)
- **Validation**: go-playground/validator/v10
- **EXIF**: rwcarlsen/goexif or dsoprea/go-exif
- **Image processing**: disintegration/imaging (thumbnails, previews)
- **Architecture**: Clean Architecture (domain → service → repository/handler)

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
│   └── server/
│       └── main.go                     # Entrypoint: config → DB → migrate → wire → run
│
├── internal/
│   │
│   ├── domain/                         # Pure business entities & value objects
│   │   ├── file.go                     # File, FileFilter, FilePage
│   │   ├── tag.go                      # Tag, TagRule
│   │   ├── category.go                 # Category
│   │   ├── pool.go                     # Pool, PoolFile
│   │   ├── user.go                     # User, Session
│   │   ├── acl.go                      # Permission, ObjectType
│   │   ├── audit.go                    # AuditEntry, ActionType
│   │   └── errors.go                   # Domain error types (ErrNotFound, ErrForbidden, etc.)
│   │
│   ├── port/                           # Interfaces (ports) — contracts between layers
│   │   ├── repository.go              # FileRepo, TagRepo, CategoryRepo, PoolRepo,
│   │   │                               # UserRepo, SessionRepo, ACLRepo, AuditRepo,
│   │   │                               # MimeRepo, TagRuleRepo
│   │   └── storage.go                 # FileStorage interface (disk operations)
│   │
│   ├── service/                        # Business logic (use cases)
│   │   ├── file_service.go             # Upload, update, delete, trash/restore, replace,
│   │   │                               # import, filter/list, duplicate detection
│   │   ├── tag_service.go              # CRUD + auto-tag application logic
│   │   ├── category_service.go         # CRUD (thin, delegates to repo + ACL + audit)
│   │   ├── pool_service.go             # CRUD + file ordering, add/remove files
│   │   ├── auth_service.go             # Login, logout, JWT issue/refresh, session management
│   │   ├── acl_service.go              # Permission checks, grant/revoke
│   │   ├── audit_service.go            # Log actions, query audit log
│   │   └── user_service.go             # Profile update, admin CRUD, block/unblock
│   │
│   ├── handler/                        # HTTP layer (Gin handlers)
│   │   ├── router.go                   # Route registration, middleware wiring
│   │   ├── middleware.go               # Auth middleware (JWT extraction → context)
│   │   ├── request.go                  # Common request parsing helpers
│   │   ├── response.go                 # Error/success response builders
│   │   ├── file_handler.go             # /files endpoints
│   │   ├── tag_handler.go              # /tags endpoints
│   │   ├── category_handler.go         # /categories endpoints
│   │   ├── pool_handler.go             # /pools endpoints
│   │   ├── auth_handler.go             # /auth endpoints
│   │   ├── acl_handler.go              # /acl endpoints
│   │   ├── user_handler.go             # /users endpoints
│   │   └── audit_handler.go            # /audit endpoints
│   │
│   ├── db/                             # Database adapters
│   │   ├── db.go                       # Common helpers: pagination, repo factory, transactor base
│   │   └── postgres/                   # PostgreSQL implementation
│   │       ├── postgres.go             # pgxpool init, tx-from-context helpers
│   │       ├── file_repo.go            # FileRepo implementation
│   │       ├── tag_repo.go             # TagRepo + TagRuleRepo implementation
│   │       ├── category_repo.go        # CategoryRepo implementation
│   │       ├── pool_repo.go            # PoolRepo implementation
│   │       ├── user_repo.go            # UserRepo implementation
│   │       ├── session_repo.go         # SessionRepo implementation
│   │       ├── acl_repo.go             # ACLRepo implementation
│   │       ├── audit_repo.go           # AuditRepo implementation
│   │       ├── mime_repo.go            # MimeRepo implementation
│   │       └── filter_parser.go        # DSL → SQL WHERE clause builder
│   │
│   ├── storage/                        # File storage adapter
│   │   └── disk.go                     # FileStorage implementation (read/write/delete on disk)
│   │
│   └── config/                         # Configuration
│       └── config.go                   # Struct + loader from env vars
│
├── migrations/                         # SQL migration files (goose format)
│   ├── 001_init_schemas.sql
│   ├── 002_core_tables.sql
│   ├── 003_data_tables.sql
│   ├── 004_acl_tables.sql
│   ├── 005_activity_tables.sql
│   ├── 006_indexes.sql
│   └── 007_seed_data.sql
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

No layer may import a layer above it. No circular dependencies.

## Key Design Decisions

### Dependency Injection (Wiring)

Manual wiring in `cmd/server/main.go`. No DI frameworks.

```go
// Pseudocode
pool := postgres.NewPool(cfg.DatabaseURL)
goose.Up(pool, migrations)

// Repos (all from internal/db/postgres/)
fileRepo   := postgres.NewFileRepo(pool)
tagRepo    := postgres.NewTagRepo(pool)
// ...

// Storage
diskStore  := storage.NewDiskStorage(cfg.FilesPath)

// Services
aclSvc     := service.NewACLService(aclRepo, objectTypeRepo)
auditSvc   := service.NewAuditService(auditRepo, actionTypeRepo)
fileSvc    := service.NewFileService(fileRepo, mimeRepo, tagRepo, diskStore, aclSvc, auditSvc)
tagSvc     := service.NewTagService(tagRepo, tagRuleRepo, aclSvc, auditSvc)
// ...

// Handlers
fileHandler := handler.NewFileHandler(fileSvc, tagSvc)
// ...

router := handler.NewRouter(cfg, fileHandler, tagHandler, ...)
router.Run(cfg.ListenAddr)
```

### Context Propagation

Every service method receives `context.Context` as the first argument.
The handler extracts user info from JWT (via middleware) and puts it
into context. Services read the current user from context for ACL checks
and audit logging.

```go
// middleware.go
func (m *AuthMiddleware) Handle(c *gin.Context) {
    claims := parseJWT(c.GetHeader("Authorization"))
    ctx := domain.WithUser(c.Request.Context(), claims.UserID, claims.IsAdmin)
    c.Request = c.Request.WithContext(ctx)
    c.Next()
}

// domain/context.go
type ctxKey int
const userKey ctxKey = iota
func WithUser(ctx context.Context, userID int16, isAdmin bool) context.Context { ... }
func UserFromContext(ctx context.Context) (userID int16, isAdmin bool) { ... }
```

### Transaction Management

Repository interfaces include a `Transactor`:

```go
// port/repository.go
type Transactor interface {
    WithTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

The postgres implementation wraps `pgxpool.Pool.BeginTx`. Inside `fn`,
all repo calls use the transaction from context. This allows services
to compose multiple repo calls in a single transaction:

```go
// service/file_service.go
func (s *FileService) Upload(ctx context.Context, input UploadInput) (*domain.File, error) {
    return s.tx.WithTx(ctx, func(ctx context.Context) error {
        file, err := s.fileRepo.Create(ctx, ...)  // uses tx
        if err != nil { return err }
        for _, tagID := range input.TagIDs {
            s.tagRepo.AddFileTag(ctx, file.ID, tagID)  // same tx
        }
        s.auditRepo.Log(ctx, ...)  // same tx
        return nil
    })
}
```

### ACL Check Pattern

ACL logic is centralized in `ACLService`. Other services call it before
any data mutation or retrieval:

```go
// service/acl_service.go
func (s *ACLService) CanView(ctx context.Context, objectType string, objectID uuid.UUID) error {
    userID, isAdmin := domain.UserFromContext(ctx)
    if isAdmin { return nil }
    // Check is_public on the object
    // If not public, check creator_id == userID
    // If not creator, check acl.permissions
    // Return domain.ErrForbidden if none match
}
```

### Error Mapping

Domain errors → HTTP status codes (handled in handler/response.go):

| Domain Error          | HTTP Status | Error Code        |
|-----------------------|-------------|-------------------|
| ErrNotFound           | 404         | not_found         |
| ErrForbidden          | 403         | forbidden         |
| ErrUnauthorized       | 401         | unauthorized      |
| ErrConflict           | 409         | conflict          |
| ErrValidation         | 400         | validation_error  |
| ErrUnsupportedMIME    | 415         | unsupported_mime  |
| (unexpected)          | 500         | internal_error    |

### Filter DSL

The DSL parser lives in `db/postgres/filter_parser.go` because it produces
SQL WHERE clauses — it is a PostgreSQL-specific adapter concern.
The service layer passes the raw DSL string to the repository; the
repository parses it and builds the query.

For a different DBMS, a corresponding parser would live in
`db/<dbms>/filter_parser.go`.

The interface:
```go
// port/repository.go
type FileRepo interface {
    List(ctx context.Context, params FileListParams) (*domain.FilePage, error)
    // ...
}

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

### JWT Structure

```go
type Claims struct {
    jwt.RegisteredClaims
    UserID   int16 `json:"uid"`
    IsAdmin  bool  `json:"adm"`
    SessionID int  `json:"sid"`
}
```

Access token: short-lived (15 min). Refresh token: long-lived (30 days),
stored as hash in `activity.sessions.token_hash`.

### Configuration (.env)

```env
# Server
LISTEN_ADDR=:42776
JWT_SECRET=<random-32-bytes>
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h

# Database
DATABASE_URL=postgres://user:pass@host:5432/tanabata?sslmode=disable

# Storage
FILES_PATH=/data/files
THUMBS_CACHE_PATH=/data/thumbs

# Thumbnails
THUMB_WIDTH=160
THUMB_HEIGHT=160
PREVIEW_WIDTH=1920
PREVIEW_HEIGHT=1080

# Import
IMPORT_PATH=/data/import
```
