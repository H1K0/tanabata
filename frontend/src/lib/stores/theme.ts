import { writable } from 'svelte/store';
import { browser } from '$app/environment';

type Theme = 'dark' | 'light';

function loadTheme(): Theme {
	if (!browser) return 'dark';
	return (localStorage.getItem('theme') as Theme) ?? 'dark';
}

function applyTheme(theme: Theme) {
	if (!browser) return;
	if (theme === 'light') {
		document.documentElement.setAttribute('data-theme', 'light');
	} else {
		document.documentElement.removeAttribute('data-theme');
	}
}

export const themeStore = writable<Theme>(loadTheme());

themeStore.subscribe((theme) => {
	applyTheme(theme);
	if (browser) localStorage.setItem('theme', theme);
});

export function toggleTheme() {
	themeStore.update((t) => (t === 'dark' ? 'light' : 'dark'));
}
