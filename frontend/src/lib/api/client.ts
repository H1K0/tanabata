import { get } from 'svelte/store';
import { authStore } from '$lib/stores/auth';

const BASE = '/api/v1';

export class ApiError extends Error {
	constructor(
		public readonly status: number,
		public readonly code: string,
		message: string,
		public readonly details?: Array<{ field?: string; message?: string }>,
	) {
		super(message);
		this.name = 'ApiError';
	}
}

// Deduplicates concurrent 401 refresh attempts into a single in-flight request.
let refreshPromise: Promise<void> | null = null;

async function refreshTokens(): Promise<void> {
	const { refreshToken } = get(authStore);
	if (!refreshToken) {
		authStore.set({ accessToken: null, refreshToken: null, user: null });
		throw new ApiError(401, 'unauthorized', 'Session expired');
	}

	const res = await fetch(`${BASE}/auth/refresh`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ refresh_token: refreshToken }),
	});

	if (!res.ok) {
		authStore.set({ accessToken: null, refreshToken: null, user: null });
		throw new ApiError(401, 'unauthorized', 'Session expired');
	}

	const data = await res.json();
	authStore.update((s) => ({
		...s,
		accessToken: data.access_token ?? null,
		refreshToken: data.refresh_token ?? null,
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
		headers: buildHeaders(init, get(authStore).accessToken),
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
			headers: buildHeaders(init, get(authStore).accessToken),
		});
	}

	if (!res.ok) {
		let body: { code?: string; message?: string; details?: Array<{ field?: string; message?: string }> } = {};
		try {
			body = await res.json();
		} catch {
			// ignore parse failure
		}
		throw new ApiError(res.status, body.code ?? 'error', body.message ?? res.statusText, body.details);
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
		request<T>(path, { method: 'POST', body: formData }),
};