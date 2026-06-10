<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import { poolSorting, type PoolSortField } from '$lib/stores/sorting';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import type { Pool, PoolOffsetPage } from '$lib/api/types';

	const LIMIT = 50;

	const SORT_OPTIONS: { value: PoolSortField; label: string }[] = [
		{ value: 'name', label: 'Name' },
		{ value: 'created', label: 'Created' },
	];

	let pools = $state<Pool[]>([]);
	let total = $state(0);
	let offset = $state(0);
	let loading = $state(false);
	let initialLoaded = $state(false);
	let error = $state('');
	let search = $state('');

	let sortState = $derived($poolSorting);

	let resetKey = $derived(`${sortState.sort}|${sortState.order}|${search}`);
	let prevKey = $state('');

	$effect(() => {
		if (resetKey !== prevKey) {
			prevKey = resetKey;
			pools = [];
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
				order: sortState.order,
			});
			if (search.trim()) params.set('search', search.trim());
			const page = await api.get<PoolOffsetPage>(`/pools?${params}`);
			pools = offset === 0 ? (page.items ?? []) : [...pools, ...(page.items ?? [])];
			total = page.total ?? 0;
			offset = pools.length;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load pools';
		} finally {
			loading = false;
			initialLoaded = true;
		}
	}

	let hasMore = $derived(pools.length < total);

	function formatCount(n: number): string {
		return n === 1 ? '1 file' : `${n} files`;
	}
</script>

<svelte:head>
	<title>Pools | Tanabata</title>
</svelte:head>

<div class="page">
	<header class="top-bar">
		<h1 class="page-title">Pools</h1>

		<div class="controls">
			<select
				class="sort-select"
				value={sortState.sort}
				onchange={(e) => poolSorting.setSort((e.currentTarget as HTMLSelectElement).value as PoolSortField)}
			>
				{#each SORT_OPTIONS as opt}
					<option value={opt.value}>{opt.label}</option>
				{/each}
			</select>

			<button
				class="icon-btn"
				onclick={() => poolSorting.toggleOrder()}
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

			<button class="new-btn" onclick={() => goto('/pools/new')}>+ New</button>
		</div>
	</header>

	<div class="search-bar">
		<div class="search-wrap">
			<input
				class="search-input"
				type="search"
				placeholder="Search pools…"
				value={search}
				oninput={(e) => (search = (e.currentTarget as HTMLInputElement).value)}
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

		<div class="pool-list">
			{#each pools as pool (pool.id)}
				<button class="pool-card" onclick={() => goto(`/pools/${pool.id}`)}>
					<div class="pool-icon" aria-hidden="true">
						<svg width="20" height="20" viewBox="0 0 20 20" fill="none">
							<rect x="2" y="2" width="7" height="7" rx="1.5" fill="currentColor" opacity="0.7"/>
							<rect x="11" y="2" width="7" height="7" rx="1.5" fill="currentColor" opacity="0.5"/>
							<rect x="2" y="11" width="7" height="7" rx="1.5" fill="currentColor" opacity="0.5"/>
							<rect x="11" y="11" width="7" height="7" rx="1.5" fill="currentColor" opacity="0.3"/>
						</svg>
					</div>
					<div class="pool-info">
						<span class="pool-name">{pool.name}</span>
						<span class="pool-meta">
							{formatCount(pool.file_count ?? 0)}
							{#if pool.creator_name}· {pool.creator_name}{/if}
							{#if pool.is_public}<span class="badge-public">public</span>{/if}
						</span>
					</div>
					<svg class="chevron" width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
						<path d="M6 4l4 4-4 4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
					</svg>
				</button>
			{/each}
		</div>

		<InfiniteScroll {loading} {hasMore} onLoadMore={load} />

		{#if !loading && pools.length === 0}
			<div class="empty">
				{search ? 'No pools match your search.' : 'No pools yet.'}
				{#if !search}
					<a href="/pools/new">Create one</a>
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

	.pool-list {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.pool-card {
		display: flex;
		align-items: center;
		gap: 12px;
		padding: 12px 14px;
		border-radius: 10px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-family: inherit;
		cursor: pointer;
		text-align: left;
		transition: border-color 0.15s, background-color 0.15s;
	}

	.pool-card:hover {
		border-color: color-mix(in srgb, var(--color-accent) 40%, transparent);
		background-color: color-mix(in srgb, var(--color-accent) 8%, var(--color-bg-elevated));
	}

	.pool-icon {
		color: var(--color-accent);
		flex-shrink: 0;
		display: flex;
	}

	.pool-info {
		flex: 1;
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.pool-name {
		font-size: 0.95rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.pool-meta {
		font-size: 0.78rem;
		color: var(--color-text-muted);
		display: flex;
		align-items: center;
		gap: 5px;
	}

	.badge-public {
		display: inline-block;
		padding: 1px 5px;
		border-radius: 4px;
		background-color: color-mix(in srgb, var(--color-accent) 20%, transparent);
		color: var(--color-accent);
		font-size: 0.7rem;
		font-weight: 600;
		letter-spacing: 0.04em;
	}

	.chevron {
		color: var(--color-text-muted);
		flex-shrink: 0;
		opacity: 0.5;
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