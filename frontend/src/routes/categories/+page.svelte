<script lang="ts">
	import { goto, beforeNavigate, afterNavigate } from '$app/navigation';
	import { get } from 'svelte/store';
	import { api, ApiError } from '$lib/api/client';
	import { categorySorting, type CategorySortField } from '$lib/stores/sorting';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import { saveSection, takeSection, type OffsetListSnapshot } from '$lib/stores/sectionCache';
	import { restoreListScroll } from '$lib/stores/listScroll';
	import type { Category, CategoryOffsetPage } from '$lib/api/types';

	const LIMIT = 100;

	const SORT_OPTIONS: { value: CategorySortField; label: string }[] = [
		{ value: 'name', label: 'Name' },
		{ value: 'color', label: 'Color' },
		{ value: 'created', label: 'Created' }
	];

	let categories = $state<Category[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(false);
	let initialLoaded = $state(false);
	let error = $state('');
	let search = $state('');

	let sortState = $derived($categorySorting);

	let resetKey = $derived(`${sortState.sort}|${sortState.order}|${search}`);
	let prevKey = $state('');

	let scrollEl = $state<HTMLElement>();
	let pendingScroll: number | null = null;

	// Rehydrate the loaded list, search and scroll from the cache on return (same
	// sort/order/search), during init so the matching prevKey/initialLoaded
	// suppress the reset + initial load below.
	const cached = takeSection<OffsetListSnapshot<Category>>('categories');
	if (cached) {
		const s0 = get(categorySorting);
		const wouldKey = `${s0.sort}|${s0.order}|${cached.data.search}`;
		if (wouldKey === cached.data.resetKey && cached.data.items.length > 0) {
			search = cached.data.search;
			categories = cached.data.items;
			total = cached.data.total;
			offset = cached.data.offset;
			initialLoaded = true;
			prevKey = wouldKey;
			pendingScroll = cached.scrollTop;
		}
	}

	beforeNavigate(() => {
		if (categories.length === 0) return;
		saveSection<OffsetListSnapshot<Category>>('categories', scrollEl?.scrollTop ?? 0, {
			resetKey,
			search,
			items: categories,
			total,
			offset
		});
	});

	afterNavigate(() => {
		if (pendingScroll == null) return;
		restoreListScroll(() => scrollEl, pendingScroll);
		pendingScroll = null;
	});

	$effect(() => {
		if (resetKey !== prevKey) {
			prevKey = resetKey;
			categories = [];
			offset = 0;
			total = 0;
			initialLoaded = false;
		}
	});

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
				order: sortState.order
			});
			if (search.trim()) params.set('search', search.trim());
			const page = await api.get<CategoryOffsetPage>(`/categories?${params}`);
			categories = offset === 0 ? (page.items ?? []) : [...categories, ...(page.items ?? [])];
			total = page.total ?? 0;
			offset = categories.length;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load categories';
		} finally {
			loading = false;
			initialLoaded = true;
		}
	}

	let hasMore = $derived(categories.length < total);
</script>

<svelte:head>
	<title>Categories | Tanabata</title>
</svelte:head>

<div class="page">
	<header class="top-bar">
		<h1 class="page-title">Categories</h1>

		<div class="controls">
			<select
				class="sort-select"
				value={sortState.sort}
				onchange={(e) =>
					categorySorting.setSort(
						(e.currentTarget as HTMLSelectElement).value as CategorySortField
					)}
			>
				{#each SORT_OPTIONS as opt}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>

			<button
				class="icon-btn"
				onclick={() => categorySorting.toggleOrder()}
				title={sortState.order === 'asc' ? 'Ascending' : 'Descending'}
			>
				{#if sortState.order === 'asc'}
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path
							d="M3 9L7 5L11 9"
							stroke="currentColor"
							stroke-width="1.8"
							stroke-linecap="round"
							stroke-linejoin="round"
						/>
					</svg>
				{:else}
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path
							d="M3 5L7 9L11 5"
							stroke="currentColor"
							stroke-width="1.8"
							stroke-linecap="round"
							stroke-linejoin="round"
						/>
					</svg>
				{/if}
			</button>

			<button class="new-btn" onclick={() => goto('/categories/new')}>+ New</button>
		</div>
	</header>

	<div class="search-bar">
		<div class="search-wrap">
			<input
				class="search-input"
				type="search"
				placeholder="Search categories…"
				value={search}
				oninput={(e) => (search = (e.currentTarget as HTMLInputElement).value)}
				autocomplete="off"
			/>
			{#if search}
				<button class="search-clear" onclick={() => (search = '')} aria-label="Clear search">
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path
							d="M2 2l10 10M12 2L2 12"
							stroke="currentColor"
							stroke-width="1.8"
							stroke-linecap="round"
						/>
					</svg>
				</button>
			{/if}
		</div>
	</div>

	<main bind:this={scrollEl}>
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="category-grid">
			{#each categories as cat (cat.id)}
				<button
					class="category-pill"
					style={cat.color ? `background-color: #${cat.color}` : ''}
					onclick={() => goto(`/categories/${cat.id}`)}
				>
					{cat.name}
				</button>
			{/each}
		</div>

		<InfiniteScroll {loading} {hasMore} onLoadMore={load} />

		{#if !loading && categories.length === 0}
			<div class="empty">
				{search ? 'No categories match your search.' : 'No categories yet.'}
				{#if !search}
					<a href="/categories/new">Create one</a>
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

	.category-grid {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		align-content: flex-start;
	}

	.category-pill {
		display: inline-flex;
		align-items: center;
		height: 32px;
		padding: 0 14px;
		border-radius: 6px;
		border: none;
		background-color: var(--color-tag-default);
		color: rgba(255, 255, 255, 0.9);
		font-size: 0.875rem;
		font-family: inherit;
		font-weight: 500;
		white-space: nowrap;
		cursor: pointer;
	}

	.category-pill:hover {
		filter: brightness(1.15);
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
