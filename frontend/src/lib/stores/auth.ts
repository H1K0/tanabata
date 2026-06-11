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

// Persist on change. Compare first so a value that just arrived from another tab
// (applied by the storage listener below) isn't written straight back, which
// would risk a storage-event echo between tabs.
authStore.subscribe((state) => {
	if (typeof localStorage === 'undefined') return;
	const serialized = JSON.stringify(state);
	if (localStorage.getItem('auth') !== serialized) {
		localStorage.setItem('auth', serialized);
	}
});

// Keep tabs in sync. Refresh tokens rotate on every use (each refresh deletes the
// old session server-side), so when one tab logs in, refreshes, or logs out, the
// others must pick up the new tokens — or the cleared session — immediately.
// Otherwise a second tab would later refresh with a token that's already been
// rotated away and get bounced to the login screen.
if (typeof window !== 'undefined') {
	window.addEventListener('storage', (e) => {
		if (e.key !== 'auth') return;
		let next: AuthState = initial;
		if (e.newValue) {
			try {
				next = (JSON.parse(e.newValue) as AuthState) ?? initial;
			} catch {
				next = initial;
			}
		}
		authStore.set(next);
	});
}

export const isAuthenticated = derived(authStore, ($auth) => !!$auth.accessToken);
