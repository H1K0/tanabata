# Tanabata File Manager — Frontend Structure

> Frontend counterpart of [ARCHITECTURE.md](ARCHITECTURE.md). This document
> details the SvelteKit layout, the CSS approach, and the API client.

## Stack

- **Framework**: SvelteKit in **SPA mode** (`adapter-static`, `ssr = false`),
  Svelte 5 (runes)
- **Language**: TypeScript (strict)
- **Build**: Vite 7
- **CSS**: Tailwind CSS **v4** via `@tailwindcss/vite` + CSS custom properties
  (`@theme` in `app.css`) — no `tailwind.config.*` / `postcss.config.*` file
- **API types**: auto-generated via `openapi-typescript` (`src/lib/api/schema.ts`)
- **PWA**: service worker + web manifest
- **Font**: Epilogue (variable weight)
- **Dev**: `vite-mock-plugin.ts` serves a mock API so the UI can run without the
  Go backend
- **Package manager**: npm

## SPA mode — why SvelteKit without the server

This frontend runs as a **pure client-side SPA**: `adapter-static` with
`fallback: 'index.html'` and `ssr = false` globally (see
`src/routes/+layout.ts`). There is no Node server in production — the build is
static assets, and the only backend is the Go API. SvelteKit is used here
purely as an SPA framework: file-based routing, the client router, and build
tooling.

**SvelteKit features we _do_ use:**

- File-based routing with nested layouts (`admin/` has its own guard) and
  dynamic segments (`[id]`).
- The client router: `goto`, the `page` store/state, `afterNavigate`,
  `navigating`.
- **Shallow routing** — `pushState`/`replaceState` + `page.state`. The
  Immich-style file viewer in `files/` and `pools/[id]/` opens as an overlay
  over the still-mounted list via shallow routing, so the browser back button
  dismisses it without reloading the grid. This is the single biggest reason we
  stay on SvelteKit rather than a plain router.
- `load` functions, used _only_ as client-side route guards (auth redirect,
  admin redirect, `/` → `/files`).
- `$lib` alias, generated `./$types`, Vite/HMR integration.

**SvelteKit features we deliberately do _not_ use** (the "server half"):

- SSR / hydration.
- `+page.server.ts`, `+server.ts` endpoints, form actions — all data goes
  through the Go API via the `$lib/api` client.
- `hooks.*`, prerendering, server-only modules — no `hooks.server.ts` /
  `hooks.client.ts` files exist.

**Decision: stay on SvelteKit, do not migrate to a bare Svelte + router SPA.**
The project already _is_ an SPA, so there is no runtime gain from switching
(adapter-static tree-shakes the unused server bits; the client-runtime size
difference is negligible). A migration would mean re-implementing nested
layouts, guards, dynamic params, and — most painfully — shallow routing /
history-state overlays by hand, for zero benefit. New contributors should not
expect SSR, endpoints, or hooks to do anything here; that is intentional.

## Monorepo Layout

```
tanabata/
├── backend/                    ← Go project (go.mod in here)
├── frontend/                   ← SvelteKit project (package.json in here)
├── openapi.yaml                ← Shared API contract (root level)
├── docker-compose.yml
├── Dockerfile
├── .env.example
└── README.md
```

`openapi.yaml` lives at repository root — both backend and frontend reference
it. The frontend generates types from it; the backend implements it.

## Frontend Directory Layout

```
frontend/
├── package.json
├── svelte.config.js                # adapter-static, fallback: index.html
├── vite.config.ts                  # plugins: tailwindcss(), sveltekit(), mockApiPlugin()
├── vite-mock-plugin.ts             # dev-only mock API (run the UI without the Go backend)
├── tsconfig.json
│
├── static/                         # Copied verbatim into the build
│   ├── manifest.webmanifest        # PWA manifest
│   ├── browserconfig.xml
│   ├── robots.txt
│   ├── favicon.ico
│   ├── fonts/
│   │   └── Epilogue-VariableFont_wght.ttf
│   └── images/                     # PWA icons, section icons (svg), login decorations
│
└── src/
    ├── app.html                    # Shell HTML (PWA meta, font preload)
    ├── app.css                     # `@import 'tailwindcss'` + `@theme` custom properties
    ├── app.d.ts                    # Ambient types
    ├── service-worker.ts           # PWA: app-shell + pinned-file offline cache
    │
    ├── lib/                        # Shared code ($lib alias)
    │   ├── index.ts
    │   │
    │   ├── api/                    # API client layer
    │   │   ├── client.ts           # fetch wrapper: bearer auth, 401 refresh+retry, error parsing,
    │   │   │                       # upload-with-progress (XHR), NDJSON streaming; exports `api`
    │   │   ├── auth.ts             # login, refresh, logout, sessions
    │   │   ├── tags.ts             # tag + tag-rule calls
    │   │   ├── categories.ts       # category calls
    │   │   ├── duplicates.ts       # duplicate list / dismiss / resolve
    │   │   ├── schema.ts           # AUTO-GENERATED from openapi.yaml (gitignored; do not edit)
    │   │   └── types.ts            # Friendly aliases: components['schemas'][...]
    │   │
    │   ├── components/
    │   │   ├── layout/
    │   │   │   ├── Header.svelte        # Section header with sorting controls
    │   │   │   ├── SelectionBar.svelte  # Floating bar for multi-select actions
    │   │   │   └── KeyboardHelp.svelte  # Keyboard-shortcut overlay
    │   │   │
    │   │   ├── file/
    │   │   │   ├── FileCard.svelte       # Single thumbnail (160×160, selectable)
    │   │   │   ├── Thumb.svelte          # Lazy-loaded thumbnail (IntersectionObserver)
    │   │   │   ├── FileViewer.svelte     # Full-screen viewer with prev/next
    │   │   │   ├── FileUpload.svelte     # Upload form + drag-and-drop
    │   │   │   ├── FilterBar.svelte      # DSL filter builder UI
    │   │   │   ├── MetadataEditor.svelte # Notes / datetime / metadata (nested) editor
    │   │   │   ├── TagPicker.svelte      # Searchable tag selector (add/remove)
    │   │   │   ├── PoolPicker.svelte     # Add-to-pool dialog
    │   │   │   ├── BulkTagEditor.svelte  # Multi-select tag add/remove
    │   │   │   └── DuplicateMergeDialog.svelte # Field-by-field duplicate resolution
    │   │   │
    │   │   ├── tag/
    │   │   │   ├── TagBadge.svelte       # Colored pill
    │   │   │   └── TagRuleEditor.svelte  # Auto-tag rule management
    │   │   │
    │   │   └── common/
    │   │       ├── ConfirmDialog.svelte
    │   │       └── InfiniteScroll.svelte # Below-the-fold lazy loading on scroll
    │   │
    │   ├── stores/                  # Svelte stores (global state)
    │   │   ├── auth.ts              # Current user + JWT tokens (persisted, cross-tab sync)
    │   │   ├── selection.ts         # Selected item IDs, selection mode
    │   │   ├── sorting.ts           # Per-section sort key + order (persisted)
    │   │   ├── theme.ts             # Dark/light theme (persisted)
    │   │   ├── appSettings.ts       # Misc client-side settings
    │   │   ├── listScroll.ts        # Restore list scroll position after overlay/back
    │   │   └── sectionCache.ts      # Cached list snapshots, invalidated on mutation
    │   │
    │   └── utils/                   # Pure helpers
    │       ├── dsl.ts               # Filter DSL builder: UI state → query string
    │       ├── metadata.ts          # Nested metadata <-> editor rows
    │       ├── pwa.ts               # PWA reset / update prompt
    │       └── rovingGrid.svelte.ts # Roving-tabindex keyboard grid navigation
    │
    └── routes/                     # SvelteKit file-based routing (guards only in load)
        ├── +layout.svelte          # Root layout: nav, theme
        ├── +layout.ts              # ssr=false; root auth guard
        ├── +page.svelte / +page.ts # / → redirect to /files
        ├── login/+page.svelte
        ├── files/
        │   ├── +page.svelte / +page.ts   # Grid: filter, sort, multi-select, upload
        │   ├── [id]/+page.svelte          # File view (also opened as shallow-routing overlay)
        │   ├── duplicates/+page.svelte    # Duplicate clusters
        │   └── trash/+page.svelte         # Trash: restore / permanent delete
        ├── tags/        { +page.svelte, new/, [id]/ }
        ├── categories/  { +page.svelte, new/, [id]/ }
        ├── pools/       { +page.svelte, new/, [id]/ }
        ├── settings/+page.svelte          # Profile: name, password, sessions, import path
        └── admin/
            ├── +layout.svelte / +layout.ts  # Restrict to admins
            ├── users/{ +page.svelte, [id]/ }
            └── audit/+page.svelte
```

## Key Architecture Decisions

### CSS: Tailwind v4 + Custom Properties

Tailwind v4 is configured **in CSS**, not in a JS config file. `app.css` imports
Tailwind and declares the theme tokens as CSS custom properties inside `@theme`;
Tailwind then generates utilities (`bg-bg-primary`, `text-text-primary`,
`font-sans`, …) from those tokens automatically.

```css
/* src/app.css */
@import "tailwindcss";

@theme {
	--color-bg-primary: #312f45;
	--color-bg-secondary: #181721;
	--color-bg-elevated: #111118;
	--color-accent: #9592b5;
	--color-accent-hover: #7d7aa4;
	--color-text-primary: #f0f0f0;
	--color-tag-default: #444455;
	/* … info / danger / warning / success / nav tokens … */

	--font-sans: "Epilogue", sans-serif;
}
```

Dark theme is primary; the light theme overrides the same custom properties.
Usage in components: `<div class="bg-bg-primary text-text-primary rounded-xl p-4">`.
Complex cases use scoped `<style>` inside `.svelte` files.

### API Client Pattern

`src/lib/api/client.ts` is a thin fetch wrapper exporting a generic `api`
object. It attaches the bearer token, transparently handles a single `401`
refresh-and-retry (deduplicating concurrent refreshes and syncing rotated
tokens across tabs), parses `{ code, message, details }` errors into `ApiError`,
and invalidates cached list snapshots on mutation. It also provides
`uploadWithProgress` (XHR, for upload progress) and `postStream` (NDJSON, for
the live import progress).

```ts
// $lib/api/client.ts (shape)
export const api = {
	get: <T>(path) => request<T>(path),
	post: <T>(path, body?) =>
		request<T>(path, { method: "POST", body: JSON.stringify(body) }),
	patch: <T>(path, body?) =>
		request<T>(path, { method: "PATCH", body: JSON.stringify(body) }),
	put: <T>(path, body?) =>
		request<T>(path, { method: "PUT", body: JSON.stringify(body) }),
	delete: <T>(path) => request<T>(path, { method: "DELETE" }),
	upload: <T>(path, fd) => request<T>(path, { method: "POST", body: fd }),
};
```

Resource modules (`auth.ts`, `tags.ts`, `categories.ts`, `duplicates.ts`) wrap
`api` with typed helpers. Endpoints without a dedicated module (files, pools,
users, acl, audit) are called through `api.*` directly from their route
components.

### Type Generation

```json
// package.json
{
	"scripts": {
		"generate:types": "openapi-typescript ../openapi.yaml -o src/lib/api/schema.ts",
		"dev": "npm run generate:types && vite dev",
		"build": "npm run generate:types && vite build"
	}
}
```

`schema.ts` is generated (and gitignored) — never edit it by hand. `types.ts`
re-exports friendly aliases:

```ts
import type { components } from "./schema";
export type File = components["schemas"]["File"];
export type Tag = components["schemas"]["Tag"];
// …
```

### SPA Mode (build)

```js
// svelte.config.js
import adapter from "@sveltejs/adapter-static";
export default { kit: { adapter: adapter({ fallback: "index.html" }) } };
```

The Go backend serves `index.html` for all non-API routes (SPA fallback, see
`handler/static.go`). In development the Vite dev server serves the UI and the
mock plugin (or a proxied Go backend) answers `/api`.

### PWA

`service-worker.ts` handles app-shell caching (HTML/CSS/JS/fonts) and optional
user-pinned file caching for offline viewing; `utils/pwa.ts` exposes the reset /
update flow (clear caches and reload from the server, keeping pinned files).
