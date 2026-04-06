import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export interface AppSettings {
	fileLoadLimit: number;
	tagRuleApplyToExisting: boolean;
}

const DEFAULTS: AppSettings = {
	fileLoadLimit: 100,
	tagRuleApplyToExisting: false,
};

function load(): AppSettings {
	if (!browser) return { ...DEFAULTS };
	try {
		const stored = JSON.parse(localStorage.getItem('app-settings') ?? 'null');
		return stored ? { ...DEFAULTS, ...stored } : { ...DEFAULTS };
	} catch {
		return { ...DEFAULTS };
	}
}

export const appSettings = writable<AppSettings>(load());

appSettings.subscribe((v) => {
	if (browser) localStorage.setItem('app-settings', JSON.stringify(v));
});