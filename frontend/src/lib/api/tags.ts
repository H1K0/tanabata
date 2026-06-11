import { get } from 'svelte/store';
import { api } from '$lib/api/client';
import type { Tag, TagOffsetPage } from '$lib/api/types';
import { tagSorting } from '$lib/stores/sorting';

// The /tags endpoint caps limit at 200 per request. Pickers and the filter bar
// filter the tag list client-side, so they need the *whole* list — otherwise
// tags past the first 200 are invisible and unsearchable. Page through until we
// have them all.
const PAGE = 200;

/**
 * Fetches every tag, paging past the server's per-request cap. Ordered by the
 * sort the user picked on the tags page (tagSorting), so the pickers and filter
 * bar show tags in the same order as that page.
 */
export async function fetchAllTags(): Promise<Tag[]> {
	const { sort, order } = get(tagSorting);
	const all: Tag[] = [];
	for (let offset = 0; ; offset += PAGE) {
		const page = await api.get<TagOffsetPage>(
			`/tags?limit=${PAGE}&offset=${offset}&sort=${sort}&order=${order}`
		);
		const items = page.items ?? [];
		all.push(...items);
		const total = page.total ?? all.length;
		if (items.length < PAGE || all.length >= total) break;
	}
	return all;
}
