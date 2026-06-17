# Tanabata File Manager

Multi-user, tag-based web file manager for images and video.

## Architecture

Monorepo: `backend/` (Go) + `frontend/` (SvelteKit).

- Backend: Go + Gin + pgx v5 + goose migrations. Clean Architecture.
- Frontend: SvelteKit SPA + Tailwind CSS + CSS custom properties.
- DB: PostgreSQL 14+.
- Auth: JWT Bearer tokens.

## Key documents (read before coding)

- `openapi.yaml` — full REST API specification (36 paths, 58 operations)
- `docs/GO_PROJECT_STRUCTURE.md` — backend architecture, layer rules, DI pattern
- `docs/FRONTEND_STRUCTURE.md` — frontend architecture, CSS approach, API client
- `docs/Описание.md` — product requirements in Russian
- `backend/migrations/001_init.sql` — database schema (4 schemas, 16 tables)

## Design reference

Visual design tokens for the frontend (carried over from the previous
Python/Flask version):
- Color palette: #312F45 (bg), #9592B5 (accent), #444455 (tag default), #111118 (elevated)
- Font: Epilogue (variable weight)
- Dark theme is primary
- Mobile-first layout with bottom navbar
- 160×160 thumbnail grid for files
- Colored tag pills
- Floating selection bar for multi-select

## Backend commands
```bash
cd backend
go run ./cmd/server          # run dev server
go test ./...                # run all tests
```

## Frontend commands
```bash
cd frontend
npm run dev                  # vite dev server
npm run build                # production build
npm run generate:types       # regenerate API types from openapi.yaml
```

## Conventions

- Go: gofmt, no global state, context.Context as first param in all service methods
- TypeScript: strict mode, named exports
- SQL: snake_case, all migrations via goose
- API errors: { code, message, details? }
- Git: conventional commits with scope — `type(scope): message`
  - `(backend)` for Go backend code
  - `(frontend)` for SvelteKit/TypeScript code
  - `(project)` for root-level files (.gitignore, docs, structure)
