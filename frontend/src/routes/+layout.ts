import { get } from 'svelte/store';
import { redirect } from '@sveltejs/kit';
import { browser } from '$app/environment';
import { authStore } from '$lib/stores/auth';

export const ssr = false;

export const load = ({ url }: { url: URL }) => {
	if (!browser) return;
	if (url.pathname === '/login') return;

	const { accessToken } = get(authStore);
	if (!accessToken) {
		redirect(307, '/login');
	}
};
