/// <reference types="@sveltejs/kit" />
/// <reference lib="webworker" />

import { build, files, version } from '$service-worker';

declare const self: ServiceWorkerGlobalScope;

// Cache name is versioned so a new deploy invalidates the old shell.
const CACHE = `app-shell-${version}`;

// App shell: the SPA entry HTML ('/'), all Vite-emitted JS/CSS chunks, and the
// static assets (fonts, icons, manifest). Pre-caching '/' makes the shell — and
// therefore the whole offline fallback below — available from the very first
// visit, before any navigation has been seen by the runtime cache.
const SHELL = ['/', ...build, ...files];

// ---- Install: pre-cache the app shell ----
self.addEventListener('install', (event) => {
	event.waitUntil(
		caches.open(CACHE).then((cache) => cache.addAll(SHELL))
	);
	// Activate immediately without waiting for old tabs to close.
	self.skipWaiting();
});

// ---- Activate: remove stale caches from previous versions ----
self.addEventListener('activate', (event) => {
	event.waitUntil(
		caches.keys().then((keys) =>
			Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))
		)
	);
	self.clients.claim();
});

// ---- Fetch: cache-first for shell assets, network-only for API ----
self.addEventListener('fetch', (event) => {
	const { request } = event;
	const url = new URL(request.url);

	// Only handle same-origin GET requests.
	if (request.method !== 'GET' || url.origin !== self.location.origin) return;

	// API and authentication calls must always go to the network.
	if (url.pathname.startsWith('/api/')) return;

	event.respondWith(respond(request));
});

async function respond(request: Request): Promise<Response> {
	const cache = await caches.open(CACHE);

	// Shell assets are pre-cached — serve from cache immediately.
	const cached = await cache.match(request);
	if (cached) return cached;

	// Everything else (navigation, dynamic routes): network first.
	try {
		const response = await fetch(request);
		// Cache successful responses for navigation so the app works offline.
		if (response.status === 200) {
			cache.put(request, response.clone());
		}
		return response;
	} catch {
		// Offline fallback: return the cached SPA shell for navigation requests.
		const fallback = await cache.match('/');
		if (fallback) return fallback;
		return new Response('Offline', { status: 503 });
	}
}