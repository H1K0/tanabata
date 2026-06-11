import { get } from 'svelte/store';
import { redirect } from '@sveltejs/kit';
import { browser } from '$app/environment';
import { authStore } from '$lib/stores/auth';

export const load = () => {
	if (!browser) return;
	const { user } = get(authStore);
	if (!user?.isAdmin) {
		redirect(307, '/files');
	}
};
