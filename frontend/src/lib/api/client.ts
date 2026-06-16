import { get } from 'svelte/store';
import { goto } from '$app/navigation';
import { browser } from '$app/environment';
import { authStore } from '$lib/stores/auth';
import { clearSection, type SectionKey } from '$lib/stores/sectionCache';

const BASE = '/api/v1';

// The tags/categories/pools lists are edited on their own detail/new pages, so a
// cached list snapshot goes stale after a write there. Drop the matching
// section's snapshot on any successful mutation so the list refetches on return.
// (Files isn't included — its grid keeps itself consistent via optimistic
// updates, and over-invalidating would needlessly lose the scroll position.)
function invalidateSectionCache(path: string, method: string): void {
	if (method === 'GET') return;
	const sections: SectionKey[] = ['tags', 'categories', 'pools'];
	for (const s of sections) {
		if (path === `/${s}` || path.startsWith(`/${s}/`) || path.startsWith(`/${s}?`)) {
			clearSection(s);
			return;
		}
	}
}

/** Clear the session and bounce to the login screen. Called when the refresh
 *  token is missing or rejected, so an expired session doesn't strand the user
 *  on a page that only shows errors. */
function endSession(): void {
	authStore.set({ accessToken: null, refreshToken: null, user: null });
	if (browser) void goto('/login');
}

export class ApiError extends Error {
	constructor(
		public readonly status: number,
		public readonly code: string,
		message: string,
		public readonly details?: Array<{ field?: string; message?: string }>
	) {
		super(message);
		this.name = 'ApiError';
	}
}

// Deduplicates concurrent 401 refresh attempts into a single in-flight request.
let refreshPromise: Promise<void> | null = null;

async function refreshTokens(): Promise<void> {
	const attempted = get(authStore).refreshToken;
	if (!attempted) {
		endSession();
		throw new ApiError(401, 'unauthorized', 'Session expired');
	}

	const res = await fetch(`${BASE}/auth/refresh`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ refresh_token: attempted })
	});

	if (!res.ok) {
		// Refresh tokens rotate, so another tab may have already refreshed and
		// rotated ours out. If a newer token has since synced in from that tab (via
		// the auth store's storage listener), adopt it and let the caller retry
		// rather than ending a session that's actually still alive.
		if (get(authStore).refreshToken !== attempted) return;
		endSession();
		throw new ApiError(401, 'unauthorized', 'Session expired');
	}

	const data = await res.json();
	authStore.update((s) => ({
		...s,
		accessToken: data.access_token ?? null,
		refreshToken: data.refresh_token ?? null
	}));
}

function buildHeaders(init: RequestInit | undefined, accessToken: string | null): HeadersInit {
	const isFormData = init?.body instanceof FormData;
	const base: Record<string, string> = isFormData ? {} : { 'Content-Type': 'application/json' };
	if (accessToken) base['Authorization'] = `Bearer ${accessToken}`;
	return { ...base, ...(init?.headers as Record<string, string> | undefined) };
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	let res = await fetch(BASE + path, {
		...init,
		headers: buildHeaders(init, get(authStore).accessToken)
	});

	if (res.status === 401) {
		if (!refreshPromise) {
			refreshPromise = refreshTokens().finally(() => {
				refreshPromise = null;
			});
		}
		try {
			await refreshPromise;
		} catch {
			throw new ApiError(401, 'unauthorized', 'Session expired');
		}

		res = await fetch(BASE + path, {
			...init,
			headers: buildHeaders(init, get(authStore).accessToken)
		});
	}

	if (!res.ok) {
		let body: {
			code?: string;
			message?: string;
			details?: Array<{ field?: string; message?: string }>;
		} = {};
		try {
			body = await res.json();
		} catch {
			// ignore parse failure
		}
		throw new ApiError(
			res.status,
			body.code ?? 'error',
			body.message ?? res.statusText,
			body.details
		);
	}

	invalidateSectionCache(path, (init?.method ?? 'GET').toUpperCase());

	// A success doesn't guarantee a JSON body: 204 never has one, and some 200/201
	// responses (e.g. POST /pools/:id/files) complete with an empty body. Parsing
	// those as JSON throws, so read the text first and only parse when present —
	// otherwise an empty 201 would surface as a spurious "failed" error.
	if (res.status === 204) return undefined as T;
	const text = await res.text();
	return (text ? JSON.parse(text) : undefined) as T;
}

/** Upload with XHR so we can track progress via onProgress(0–100). */
export function uploadWithProgress<T>(
	path: string,
	formData: FormData,
	onProgress: (pct: number) => void
): Promise<T> {
	return new Promise((resolve, reject) => {
		const token = get(authStore).accessToken;
		const xhr = new XMLHttpRequest();
		xhr.open('POST', BASE + path);
		if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`);

		xhr.upload.onprogress = (e) => {
			if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
		};

		xhr.onload = () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				try {
					resolve(JSON.parse(xhr.responseText) as T);
				} catch {
					resolve(undefined as T);
				}
			} else {
				let body: { code?: string; message?: string } = {};
				try {
					body = JSON.parse(xhr.responseText);
				} catch {
					/* ignore */
				}
				reject(new ApiError(xhr.status, body.code ?? 'error', body.message ?? xhr.statusText));
			}
		};

		xhr.onerror = () => reject(new ApiError(0, 'network_error', 'Network error'));
		xhr.send(formData);
	});
}

/** POST that consumes a streamed newline-delimited JSON (NDJSON) response,
 *  invoking onEvent once per parsed line. Used by the server-side import so the
 *  UI can render live per-file progress. Reuses the bearer token and a single
 *  401 refresh+retry, but (unlike request()) keeps the body as a stream. */
export async function postStream(
	path: string,
	body: unknown,
	onEvent: (ev: Record<string, unknown>) => void
): Promise<void> {
	const init: RequestInit = { method: 'POST', body: JSON.stringify(body) };
	const send = () =>
		fetch(BASE + path, { ...init, headers: buildHeaders(init, get(authStore).accessToken) });

	let res = await send();
	if (res.status === 401) {
		if (!refreshPromise) {
			refreshPromise = refreshTokens().finally(() => {
				refreshPromise = null;
			});
		}
		try {
			await refreshPromise;
		} catch {
			throw new ApiError(401, 'unauthorized', 'Session expired');
		}
		res = await send();
	}

	if (!res.ok || !res.body) {
		let b: { code?: string; message?: string } = {};
		try {
			b = await res.json();
		} catch {
			// ignore parse failure
		}
		throw new ApiError(res.status, b.code ?? 'error', b.message ?? res.statusText);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	let buf = '';
	const flushLine = (line: string) => {
		const trimmed = line.trim();
		if (trimmed) onEvent(JSON.parse(trimmed));
	};

	for (;;) {
		const { done, value } = await reader.read();
		if (done) break;
		buf += decoder.decode(value, { stream: true });
		let nl: number;
		while ((nl = buf.indexOf('\n')) >= 0) {
			flushLine(buf.slice(0, nl));
			buf = buf.slice(nl + 1);
		}
	}
	buf += decoder.decode();
	flushLine(buf);
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
		request<T>(path, { method: 'POST', body: formData })
};
