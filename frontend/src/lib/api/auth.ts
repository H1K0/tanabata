import { get } from 'svelte/store';
import { authStore } from '$lib/stores/auth';
import { api } from './client';
import type { TokenPair, SessionList } from './types';

export async function login(name: string, password: string): Promise<void> {
	const tokens = await api.post<TokenPair>('/auth/login', { name, password });
	authStore.update((s) => ({
		...s,
		accessToken: tokens.access_token ?? null,
		refreshToken: tokens.refresh_token ?? null
	}));
}

export async function logout(): Promise<void> {
	try {
		await api.post('/auth/logout');
	} finally {
		authStore.set({ accessToken: null, refreshToken: null, user: null });
	}
}

export async function refresh(): Promise<void> {
	const { refreshToken } = get(authStore);
	if (!refreshToken) throw new Error('No refresh token');

	const tokens = await api.post<TokenPair>('/auth/refresh', { refresh_token: refreshToken });
	authStore.update((s) => ({
		...s,
		accessToken: tokens.access_token ?? null,
		refreshToken: tokens.refresh_token ?? null
	}));
}

export function listSessions(params?: { offset?: number; limit?: number }): Promise<SessionList> {
	const entries = Object.entries(params ?? {})
		.filter(([, v]) => v !== undefined)
		.map(([k, v]) => [k, String(v)]);
	const qs = entries.length ? '?' + new URLSearchParams(entries).toString() : '';
	return api.get<SessionList>(`/auth/sessions${qs}`);
}

export function terminateSession(sessionId: number): Promise<void> {
	return api.delete<void>(`/auth/sessions/${sessionId}`);
}
