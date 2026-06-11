import { writable, derived } from 'svelte/store';

export interface AuthUser {
	id: number;
	name: string;
	isAdmin: boolean;
}

export interface AuthState {
	accessToken: string | null;
	refreshToken: string | null;
	user: AuthUser | null;
}

const initial: AuthState = { accessToken: null, refreshToken: null, user: null };

function loadStored(): AuthState {
	if (typeof localStorage === 'undefined') return initial;
	try {
		return JSON.parse(localStorage.getItem('auth') ?? 'null') ?? initial;
	} catch {
		return initial;
	}
}

export const authStore = writable<AuthState>(loadStored());

authStore.subscribe((state) => {
	if (typeof localStorage !== 'undefined') {
		localStorage.setItem('auth', JSON.stringify(state));
	}
});

export const isAuthenticated = derived(authStore, ($auth) => !!$auth.accessToken);
