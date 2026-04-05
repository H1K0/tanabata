import { writable } from 'svelte/store';
import { browser } from '$app/environment';

export type FileSortField = 'content_datetime' | 'created' | 'original_name' | 'mime';
export type TagSortField = 'name' | 'color' | 'category_name' | 'created';
export type SortOrder = 'asc' | 'desc';

export interface SortState<F extends string> {
	sort: F;
	order: SortOrder;
}

function makeSortStore<F extends string>(key: string, defaults: SortState<F>) {
	const stored = browser ? localStorage.getItem(key) : null;
	const initial: SortState<F> = stored ? (JSON.parse(stored) as SortState<F>) : defaults;
	const store = writable<SortState<F>>(initial);

	store.subscribe((v) => {
		if (browser) localStorage.setItem(key, JSON.stringify(v));
	});

	return {
		subscribe: store.subscribe,
		setSort(sort: F) {
			store.update((s) => ({ ...s, sort }));
		},
		setOrder(order: SortOrder) {
			store.update((s) => ({ ...s, order }));
		},
		toggleOrder() {
			store.update((s) => ({ ...s, order: s.order === 'asc' ? 'desc' : 'asc' }));
		},
	};
}

export const fileSorting = makeSortStore<FileSortField>('sort:files', {
	sort: 'created',
	order: 'desc',
});

export const tagSorting = makeSortStore<TagSortField>('sort:tags', {
	sort: 'created',
	order: 'desc',
});

export type CategorySortField = 'name' | 'color' | 'created';

export const categorySorting = makeSortStore<CategorySortField>('sort:categories', {
	sort: 'name',
	order: 'asc',
});