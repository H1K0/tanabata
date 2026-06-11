import { get } from 'svelte/store';
import { api } from '$lib/api/client';
import type { Category, CategoryOffsetPage } from '$lib/api/types';
import { categorySorting } from '$lib/stores/sorting';

// The /categories endpoint caps limit at 200 per request. Category dropdowns
// show the whole list, so page through to get them all — otherwise categories
// past the first 200 are missing from the picker.
const PAGE = 200;

/**
 * Fetches every category, paging past the server's per-request cap. Ordered by
 * the sort the user picked on the categories page (categorySorting).
 */
export async function fetchAllCategories(): Promise<Category[]> {
	const { sort, order } = get(categorySorting);
	const all: Category[] = [];
	for (let offset = 0; ; offset += PAGE) {
		const page = await api.get<CategoryOffsetPage>(
			`/categories?limit=${PAGE}&offset=${offset}&sort=${sort}&order=${order}`
		);
		const items = page.items ?? [];
		all.push(...items);
		const total = page.total ?? all.length;
		if (items.length < PAGE || all.length >= total) break;
	}
	return all;
}
