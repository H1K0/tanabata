# Tanabata File Manager — Frontend Structure

## Stack

- **Framework**: SvelteKit (SPA mode, `ssr: false`)
- **Language**: TypeScript
- **CSS**: Tailwind CSS + CSS custom properties (hybrid)
- **API types**: Auto-generated via openapi-typescript
- **PWA**: Service worker + web manifest
- **Font**: Epilogue (variable weight)
- **Package manager**: npm

## SPA mode — why SvelteKit without the server

This frontend runs as a **pure client-side SPA**: `adapter-static` with
`fallback: 'index.html'` and `ssr = false` globally (see
`src/routes/+layout.ts`). There is no Node server in production — the build is
static assets, and the only backend is the Go API. SvelteKit is used here
purely as an SPA framework: file-based routing, the client router, and build
tooling.

**SvelteKit features we *do* use:**

- File-based routing with nested layouts (`admin/` has its own guard) and
  dynamic segments (`[id]`).
- The client router: `goto`, the `page` store/state, `afterNavigate`,
  `navigating`.
- **Shallow routing** — `pushState`/`replaceState` + `page.state`. The
  Immich-style file viewer in `files/` and `pools/[id]/` opens as an overlay
  over the still-mounted list via shallow routing, so the browser back button
  dismisses it without reloading the grid. This is the single biggest reason we
  stay on SvelteKit rather than a plain router.
- `load` functions, used *only* as client-side route guards (auth redirect,
  admin redirect, `/` → `/files`).
- `$lib` alias, generated `./$types`, Vite/HMR integration.

**SvelteKit features we deliberately do *not* use** (the "server half"):

- SSR / hydration.
- `+page.server.ts`, `+server.ts` endpoints, form actions — all data goes
  through the Go API via the `$lib/api` client.
- `hooks.*`, prerendering, server-only modules — no `hooks.server.ts` /
  `hooks.client.ts` files exist.

**Decision: stay on SvelteKit, do not migrate to a bare Svelte + router SPA.**
The project already *is* an SPA, so there is no runtime gain from switching
(adapter-static tree-shakes the unused server bits; the client-runtime size
difference is negligible). A migration would mean re-implementing nested
layouts, guards, dynamic params, and — most painfully — shallow routing /
history-state overlays by hand, for zero benefit. New contributors should not
expect SSR, endpoints, or hooks to do anything here; that is intentional.

## Monorepo Layout

```
tanabata/
├── backend/                    ← Go project (go.mod in here)
│   ├── cmd/
│   ├── internal/
│   ├── migrations/
│   ├── go.mod
│   └── go.sum
│
├── frontend/                   ← SvelteKit project (package.json in here)
│   └── (see below)
│
├── openapi.yaml                ← Shared API contract (root level)
├── docker-compose.yml
├── Dockerfile
├── .env.example
└── README.md
```

`openapi.yaml` lives at repository root — both backend and frontend
reference it. The frontend generates types from it; the backend
validates its handlers against it.

## Frontend Directory Layout

```
frontend/
├── package.json
├── svelte.config.js
├── vite.config.ts
├── tsconfig.json
├── tailwind.config.ts
├── postcss.config.js
│
├── src/
│   ├── app.html                    # Shell HTML (PWA meta, font preload)
│   ├── app.css                     # Tailwind directives + CSS custom properties
│   │                               # (no hooks.* — see "SPA mode" above)
│   │
│   ├── lib/                        # Shared code ($lib/ alias)
│   │   │
│   │   ├── api/                    # API client layer
│   │   │   ├── client.ts           # Base fetch wrapper: auth headers, token refresh,
│   │   │   │                       # error parsing, base URL
│   │   │   ├── files.ts            # listFiles, getFile, uploadFile, deleteFile, etc.
│   │   │   ├── tags.ts             # listTags, createTag, getTag, updateTag, etc.
│   │   │   ├── categories.ts       # Category API functions
│   │   │   ├── pools.ts            # Pool API functions
│   │   │   ├── auth.ts             # login, logout, refresh, listSessions
│   │   │   ├── acl.ts              # getPermissions, setPermissions
│   │   │   ├── users.ts            # getMe, updateMe, admin user CRUD
│   │   │   ├── audit.ts            # queryAuditLog
│   │   │   ├── schema.ts           # AUTO-GENERATED from openapi.yaml (do not edit)
│   │   │   └── types.ts            # Friendly type aliases:
│   │   │                           #   export type File = components["schemas"]["File"]
│   │   │                           #   export type Tag = components["schemas"]["Tag"]
│   │   │
│   │   ├── components/             # Reusable UI components
│   │   │   │
│   │   │   ├── layout/             # App shell
│   │   │   │   ├── Navbar.svelte       # Bottom navigation bar (mobile-first)
│   │   │   │   ├── Header.svelte       # Section header with sorting controls
│   │   │   │   ├── SelectionBar.svelte # Floating bar for multi-select actions
│   │   │   │   └── Loader.svelte       # Full-screen loading overlay
│   │   │   │
│   │   │   ├── file/               # File-related components
│   │   │   │   ├── FileGrid.svelte     # Thumbnail grid with infinite scroll
│   │   │   │   ├── FileCard.svelte     # Single thumbnail (160×160, selectable)
│   │   │   │   ├── FileViewer.svelte   # Full-screen preview with prev/next navigation
│   │   │   │   ├── FileUpload.svelte   # Upload form + drag-and-drop zone
│   │   │   │   ├── FileDetail.svelte   # Metadata editor (notes, datetime, tags)
│   │   │   │   └── FilterBar.svelte    # DSL filter builder UI
│   │   │   │
│   │   │   ├── tag/                # Tag-related components
│   │   │   │   ├── TagBadge.svelte     # Colored pill with tag name
│   │   │   │   ├── TagPicker.svelte    # Searchable tag selector (add/remove)
│   │   │   │   ├── TagList.svelte      # Tag grid for section view
│   │   │   │   └── TagRuleEditor.svelte # Auto-tag rule management
│   │   │   │
│   │   │   ├── pool/               # Pool-related components
│   │   │   │   ├── PoolCard.svelte     # Pool preview card
│   │   │   │   ├── PoolFileList.svelte # Ordered file list with drag reorder
│   │   │   │   └── PoolDetail.svelte   # Pool metadata editor
│   │   │   │
│   │   │   ├── acl/                # Access control components
│   │   │   │   └── PermissionEditor.svelte  # User permission grid
│   │   │   │
│   │   │   └── common/             # Shared primitives
│   │   │       ├── Button.svelte
│   │   │       ├── Modal.svelte
│   │   │       ├── ConfirmDialog.svelte
│   │   │       ├── Toast.svelte
│   │   │       ├── InfiniteScroll.svelte
│   │   │       ├── Pagination.svelte
│   │   │       ├── SortDropdown.svelte
│   │   │       ├── SearchInput.svelte
│   │   │       ├── ColorPicker.svelte
│   │   │       ├── Checkbox.svelte     # Three-state: checked, unchecked, partial
│   │   │       └── EmptyState.svelte
│   │   │
│   │   ├── stores/                 # Svelte stores (global state)
│   │   │   ├── auth.ts             # Current user, JWT tokens, isAuthenticated
│   │   │   ├── selection.ts        # Selected item IDs, selection mode toggle
│   │   │   ├── sorting.ts          # Per-section sort key + order (persisted to localStorage)
│   │   │   ├── theme.ts            # Dark/light mode (persisted, respects prefers-color-scheme)
│   │   │   └── toast.ts            # Notification queue (success, error, info)
│   │   │
│   │   └── utils/                  # Pure helper functions
│   │       ├── format.ts           # formatDate, formatFileSize, formatDuration
│   │       ├── dsl.ts              # Filter DSL builder: UI state → query string
│   │       ├── pwa.ts              # PWA reset, cache clear, update prompt
│   │       └── keyboard.ts         # Keyboard shortcut helpers (Ctrl+A, Escape, etc.)
│   │
│   ├── routes/                     # SvelteKit file-based routing
│   │   │
│   │   ├── +layout.svelte          # Root layout: Navbar, theme wrapper, toast container
│   │   ├── +layout.ts              # Root load: auth guard → redirect to /login if no token
│   │   │
│   │   ├── +page.svelte            # / → redirect to /files
│   │   │
│   │   ├── login/
│   │   │   └── +page.svelte        # Login form (decorative Tanabata images)
│   │   │
│   │   ├── files/
│   │   │   ├── +page.svelte        # File grid: filter bar, sort, multi-select, upload
│   │   │   ├── +page.ts            # Load: initial file list (cursor page)
│   │   │   ├── [id]/
│   │   │   │   ├── +page.svelte    # File view: preview, metadata, tags, ACL
│   │   │   │   └── +page.ts        # Load: file detail + tags
│   │   │   └── trash/
│   │   │       ├── +page.svelte    # Trash: restore / permanent delete
│   │   │       └── +page.ts
│   │   │
│   │   ├── tags/
│   │   │   ├── +page.svelte        # Tag list: search, sort, multi-select
│   │   │   ├── +page.ts
│   │   │   ├── new/
│   │   │   │   └── +page.svelte    # Create tag form
│   │   │   └── [id]/
│   │   │       ├── +page.svelte    # Tag detail: edit, category, rules, parent tags
│   │   │       └── +page.ts
│   │   │
│   │   ├── categories/
│   │   │   ├── +page.svelte        # Category list
│   │   │   ├── +page.ts
│   │   │   ├── new/
│   │   │   │   └── +page.svelte
│   │   │   └── [id]/
│   │   │       ├── +page.svelte    # Category detail: edit, view tags
│   │   │       └── +page.ts
│   │   │
│   │   ├── pools/
│   │   │   ├── +page.svelte        # Pool list
│   │   │   ├── +page.ts
│   │   │   ├── new/
│   │   │   │   └── +page.svelte
│   │   │   └── [id]/
│   │   │       ├── +page.svelte    # Pool detail: files (reorderable), filter, edit
│   │   │       └── +page.ts
│   │   │
│   │   ├── settings/
│   │   │   ├── +page.svelte        # Profile: name, password, active sessions
│   │   │   └── +page.ts
│   │   │
│   │   └── admin/
│   │       ├── +layout.svelte      # Admin layout: restrict to is_admin
│   │       ├── users/
│   │       │   ├── +page.svelte    # User management list
│   │       │   ├── +page.ts
│   │       │   └── [id]/
│   │       │       ├── +page.svelte # User detail: role, block/unblock
│   │       │       └── +page.ts
│   │       └── audit/
│   │           ├── +page.svelte    # Audit log with filters
│   │           └── +page.ts
│   │
│   └── service-worker.ts          # PWA: offline cache for pinned files, app shell caching
│
└── static/
    ├── favicon.png
    ├── favicon.ico
    ├── manifest.webmanifest        # PWA manifest (name, icons, theme_color)
    ├── images/
    │   ├── tanabata-left.png       # Login page decorations (from current design)
    │   ├── tanabata-right.png
    │   └── icons/                  # PWA icons (192×192, 512×512, etc.)
    └── fonts/
        └── Epilogue-VariableFont_wght.ttf
```

## Key Architecture Decisions

### CSS Hybrid: Tailwind + Custom Properties

Theme colors defined as CSS custom properties in `app.css`:

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

:root {
  --color-bg-primary: #312F45;
  --color-bg-secondary: #181721;
  --color-bg-elevated: #111118;
  --color-accent: #9592B5;
  --color-accent-hover: #7D7AA4;
  --color-text-primary: #f0f0f0;
  --color-text-muted: #9999AD;
  --color-danger: #DB6060;
  --color-info: #4DC7ED;
  --color-warning: #F5E872;
  --color-tag-default: #444455;
}

:root[data-theme="light"] {
  --color-bg-primary: #f5f5f5;
  --color-bg-secondary: #ffffff;
  /* ... */
}
```

Tailwind references them in `tailwind.config.ts`:

```ts
export default {
  theme: {
    extend: {
      colors: {
        bg: {
          primary: 'var(--color-bg-primary)',
          secondary: 'var(--color-bg-secondary)',
          elevated: 'var(--color-bg-elevated)',
        },
        accent: {
          DEFAULT: 'var(--color-accent)',
          hover: 'var(--color-accent-hover)',
        },
        // ...
      },
      fontFamily: {
        sans: ['Epilogue', 'sans-serif'],
      },
    },
  },
  darkMode: 'class', // controlled via data-theme attribute
};
```

Usage in components: `<div class="bg-bg-primary text-text-primary rounded-xl p-4">`.
Complex cases use scoped `<style>` inside `.svelte` files.

### API Client Pattern

`client.ts` — thin wrapper around fetch:

```ts
// $lib/api/client.ts
import { authStore } from '$lib/stores/auth';

const BASE = '/api/v1';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const token = get(authStore).accessToken;
  const res = await fetch(BASE + path, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...init?.headers,
    },
  });
  if (res.status === 401) {
    // attempt refresh, retry once
  }
  if (!res.ok) {
    const err = await res.json();
    throw new ApiError(res.status, err.code, err.message, err.details);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'POST', body: JSON.stringify(body) }),
  patch: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PATCH', body: JSON.stringify(body) }),
  put: <T>(path: string, body?: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
  upload: <T>(path: string, formData: FormData) =>
    request<T>(path, { method: 'POST', body: formData, headers: {} }),
};
```

Domain-specific modules use it:

```ts
// $lib/api/files.ts
import { api } from './client';
import type { File, FileCursorPage } from './types';

export function listFiles(params: Record<string, string>) {
  const qs = new URLSearchParams(params).toString();
  return api.get<FileCursorPage>(`/files?${qs}`);
}

export function uploadFile(formData: FormData) {
  return api.upload<File>('/files', formData);
}
```

### Type Generation

Script in `package.json`:

```json
{
  "scripts": {
    "generate:types": "openapi-typescript ../openapi.yaml -o src/lib/api/schema.ts",
    "dev": "npm run generate:types && vite dev",
    "build": "npm run generate:types && vite build"
  }
}
```

Friendly aliases in `types.ts`:

```ts
import type { components } from './schema';

export type File = components['schemas']['File'];
export type Tag = components['schemas']['Tag'];
export type Category = components['schemas']['Category'];
export type Pool = components['schemas']['Pool'];
export type FileCursorPage = components['schemas']['FileCursorPage'];
export type TagOffsetPage = components['schemas']['TagOffsetPage'];
export type Error = components['schemas']['Error'];
// ...
```

### SPA Mode

`svelte.config.js`:

```js
import adapter from '@sveltejs/adapter-static';

export default {
  kit: {
    adapter: adapter({ fallback: 'index.html' }),
    // SPA: all routes handled client-side
  },
};
```

The Go backend serves `index.html` for all non-API routes (SPA fallback).
In development, Vite dev server proxies `/api` to the Go backend.

### PWA

`service-worker.ts` handles:
- App shell caching (HTML, CSS, JS, fonts)
- User-pinned file caching (explicit, via UI button)
- Cache versioning and cleanup on update
- Reset function (clear all caches except pinned files)
