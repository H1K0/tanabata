<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { ApiError } from '$lib/api/client';
	import FileCard from '$lib/components/file/FileCard.svelte';
	import FilterBar from '$lib/components/file/FilterBar.svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import { fileSorting, type FileSortField } from '$lib/stores/sorting';
	import { parseDslFilter } from '$lib/utils/dsl';
	import type { File, FileCursorPage } from '$lib/api/types';

	const LIMIT = 50;

	const FILE_SORT_OPTIONS = [
		{ value: 'created', label: 'Created' },
		{ value: 'content_datetime', label: 'Date taken' },
		{ value: 'original_name', label: 'Name' },
		{ value: 'mime', label: 'Type' },
	];

	let files = $state<File[]>([]);
	let nextCursor = $state<string | null>(null);
	let loading = $state(false);
	let hasMore = $state(true);
	let error = $state('');
	let filterOpen = $state(false);

	// Derive current filter from URL ?filter= param
	let filterParam = $derived($page.url.searchParams.get('filter'));
	let activeTokens = $derived(parseDslFilter(filterParam));

	// Track sort/order from store
	let sortState = $derived($fileSorting);

	// Reset + reload whenever sort or filter changes
	let resetKey = $derived(`${sortState.sort}|${sortState.order}|${filterParam ?? ''}`);
	let prevKey = $state('');

	$effect(() => {
		if (resetKey !== prevKey) {
			prevKey = resetKey;
			files = [];
			nextCursor = null;
			hasMore = true;
			error = '';
		}
	});

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		error = '';

		try {
			const params = new URLSearchParams({
				limit: String(LIMIT),
				sort: sortState.sort,
				order: sortState.order,
			});
			if (nextCursor) params.set('cursor', nextCursor);
			if (filterParam) params.set('filter', filterParam);

			const res = await api.get<FileCursorPage>(`/files?${params}`);
			files = [...files, ...(res.items ?? [])];
			nextCursor = res.next_cursor ?? null;
			hasMore = !!res.next_cursor;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load files';
			hasMore = false;
		} finally {
			loading = false;
		}
	}

	function applyFilter(filter: string | null) {
		const url = new URL($page.url);
		if (filter) {
			url.searchParams.set('filter', filter);
		} else {
			url.searchParams.delete('filter');
		}
		goto(url.toString(), { replaceState: true });
		filterOpen = false;
	}
</script>

<svelte:head>
	<title>Files | Tanabata</title>
</svelte:head>

<div class="page">
	<Header
		sortOptions={FILE_SORT_OPTIONS}
		sort={sortState.sort}
		order={sortState.order}
		filterActive={activeTokens.length > 0 || filterOpen}
		onSortChange={(s) => fileSorting.setSort(s as FileSortField)}
		onOrderToggle={() => fileSorting.toggleOrder()}
		onFilterToggle={() => (filterOpen = !filterOpen)}
	/>

	{#if filterOpen}
		<FilterBar
			value={filterParam}
			onApply={applyFilter}
			onClose={() => (filterOpen = false)}
		/>
	{/if}

	<main>
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="grid">
			{#each files as file (file.id)}
				<FileCard {file} />
			{/each}
		</div>

		<InfiniteScroll {loading} {hasMore} onLoadMore={loadMore} />

		{#if !loading && !hasMore && files.length === 0}
			<div class="empty">No files yet.</div>
		{/if}
	</main>
</div>

<style>
	.page {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}

	main {
		flex: 1;
		overflow-y: auto;
		padding: 10px 10px calc(60px + 10px); /* clear fixed navbar */
	}

	.grid {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-content: flex-start;
		align-items: flex-start;
		gap: 2px;
	}

	/* phantom last item so justify-content doesn't stretch final row */
	.grid::after {
		content: '';
		flex: auto;
	}

	.error {
		color: var(--color-danger);
		padding: 12px;
		font-size: 0.875rem;
	}

	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 60px 20px;
		font-size: 0.95rem;
	}
</style>