<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import { tagSorting, type TagSortField } from '$lib/stores/sorting';
	import TagBadge from '$lib/components/tag/TagBadge.svelte';
	import type { Tag, TagOffsetPage } from '$lib/api/types';

	const LIMIT = 100;

	const SORT_OPTIONS = [
		{ value: 'name', label: 'Name' },
		{ value: 'created', label: 'Created' },
		{ value: 'color', label: 'Color' },
		{ value: 'category_name', label: 'Category' },
	];

	let tags = $state<Tag[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(false);
	let initialLoaded = $state(false); // true once first page loaded for current key
	let error = $state('');
	let search = $state('');
	let searchDebounce: ReturnType<typeof setTimeout>;

	let sortState = $derived($tagSorting);

	// Reset + reload on sort or search change
	let resetKey = $derived(`${sortState.sort}|${sortState.order}|${search}`);
	let prevKey = $state('');

	$effect(() => {
		if (resetKey !== prevKey) {
			prevKey = resetKey;
			tags = [];
			offset = 0;
			total = 0;
			initialLoaded = false;
		}
	});

	// Trigger load after reset (only once per key)
	$effect(() => {
		if (!initialLoaded && !loading) void load();
	});

	async function load() {
		if (loading) return;
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				limit: String(LIMIT),
				offset: String(offset),
				sort: sortState.sort,
				order: sortState.order,
			});
			if (search.trim()) params.set('search', search.trim());
			const page = await api.get<TagOffsetPage>(`/tags?${params}`);
			tags = offset === 0 ? (page.items ?? []) : [...tags, ...(page.items ?? [])];
			total = page.total ?? 0;
			offset = tags.length;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load tags';
		} finally {
			loading = false;
			initialLoaded = true;
		}
	}

	function onSearch(e: Event) {
		search = (e.currentTarget as HTMLInputElement).value;
		clearTimeout(searchDebounce);
		searchDebounce = setTimeout(() => {}, 0); // reactive reset already handles it
	}

	let hasMore = $derived(tags.length < total);
</script>

<svelte:head>
	<title>Tags | Tanabata</title>
</svelte:head>

<div class="page">
	<header class="top-bar">
		<h1 class="page-title">Tags</h1>

		<div class="controls">
			<select
				class="sort-select"
				value={sortState.sort}
				onchange={(e) => tagSorting.setSort((e.currentTarget as HTMLSelectElement).value as TagSortField)}
			>
				{#each SORT_OPTIONS as opt}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>

			<button
				class="icon-btn"
				onclick={() => tagSorting.toggleOrder()}
				title={sortState.order === 'asc' ? 'Ascending' : 'Descending'}
			>
				{#if sortState.order === 'asc'}
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path d="M3 9L7 5L11 9" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
					</svg>
				{:else}
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path d="M3 5L7 9L11 5" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
					</svg>
				{/if}
			</button>

			<button class="new-btn" onclick={() => goto('/tags/new')}>+ New</button>
		</div>
	</header>

	<div class="search-bar">
		<div class="search-wrap">
			<input
				class="search-input"
				type="search"
				placeholder="Search tags…"
				value={search}
				oninput={onSearch}
				autocomplete="off"
			/>
			{#if search}
				<button class="search-clear" onclick={() => (search = '')} aria-label="Clear search">
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path d="M2 2l10 10M12 2L2 12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
					</svg>
				</button>
			{/if}
		</div>
	</div>

	<main>
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="tag-grid">
			{#each tags as tag (tag.id)}
				<TagBadge {tag} onclick={() => goto(`/tags/${tag.id}`)} />
			{/each}
		</div>

		{#if loading}
			<div class="loading-row">
				<span class="spinner" role="status" aria-label="Loading"></span>
			</div>
		{/if}

		{#if hasMore && !loading}
			<button class="load-more" onclick={load}>Load more</button>
		{/if}

		{#if !loading && tags.length === 0}
			<div class="empty">
				{search ? 'No tags match your search.' : 'No tags yet.'}
				{#if !search}
					<a href="/tags/new">Create one</a>
				{/if}
			</div>
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

	.top-bar {
		position: sticky;
		top: 0;
		z-index: 10;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 12px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		flex-shrink: 0;
	}

	.page-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0;
		flex: 1;
	}

	.controls {
		display: flex;
		align-items: center;
		gap: 4px;
	}

	.sort-select {
		height: 28px;
		padding: 0 8px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.82rem;
		font-family: inherit;
		cursor: pointer;
		outline: none;
	}

	.icon-btn {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		cursor: pointer;
	}

	.icon-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-accent);
	}

	.new-btn {
		height: 28px;
		padding: 0 12px;
		border-radius: 6px;
		border: none;
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-size: 0.82rem;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
	}

	.new-btn:hover {
		background-color: var(--color-accent-hover);
	}

	.search-bar {
		padding: 8px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 10%, transparent);
		flex-shrink: 0;
	}

	.search-wrap {
		position: relative;
		display: flex;
		align-items: center;
	}

	.search-input {
		width: 100%;
		box-sizing: border-box;
		height: 34px;
		padding: 0 12px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		outline: none;
	}

	.search-input:focus {
		border-color: var(--color-accent);
	}

	.search-clear {
		position: absolute;
		right: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		border: none;
		background: none;
		color: var(--color-text-muted);
		cursor: pointer;
		padding: 0;
	}

	.search-clear:hover {
		color: var(--color-text-primary);
		background-color: color-mix(in srgb, var(--color-accent) 20%, transparent);
	}

	main {
		flex: 1;
		overflow-y: auto;
		padding: 12px 12px calc(60px + 12px);
	}

	.tag-grid {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-content: flex-start;
	}

	.loading-row {
		display: flex;
		justify-content: center;
		padding: 20px;
	}

	.spinner {
		display: block;
		width: 28px;
		height: 28px;
		border: 3px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-top-color: var(--color-accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin { to { transform: rotate(360deg); } }

	.load-more {
		display: block;
		margin: 16px auto 0;
		padding: 8px 24px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 40%, transparent);
		background: none;
		color: var(--color-accent);
		font-family: inherit;
		font-size: 0.85rem;
		cursor: pointer;
	}

	.load-more:hover {
		background-color: color-mix(in srgb, var(--color-accent) 10%, transparent);
	}

	.error {
		color: var(--color-danger);
		font-size: 0.875rem;
		padding: 8px 0;
	}

	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 60px 20px;
		font-size: 0.95rem;
		display: flex;
		flex-direction: column;
		gap: 8px;
		align-items: center;
	}

	.empty a {
		color: var(--color-accent);
		text-decoration: none;
	}
</style>