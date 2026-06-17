<script lang="ts">
	import { api } from '$lib/api/client';
	import { tick } from 'svelte';
	import type { Pool, PoolOffsetPage } from '$lib/api/types';

	interface Props {
		/** Files to add to the chosen pool. */
		fileIds: string[];
		/** Called after a successful add (before close) — e.g. to clear a selection. */
		onAdded?: (poolId: string) => void;
		/** Close the picker without adding. */
		onClose: () => void;
	}

	let { fileIds, onAdded, onClose }: Props = $props();

	let pools = $state<Pool[]>([]);
	let loading = $state(true);
	let loadError = $state('');
	let addError = $state('');
	let search = $state('');
	let busy = $state(false);
	// Index of the keyboard-highlighted pool within `filtered`.
	let highlight = $state(0);
	let searchEl = $state<HTMLInputElement | null>(null);
	let listEl = $state<HTMLUListElement | null>(null);

	$effect(() => {
		void load();
	});

	async function load() {
		loading = true;
		loadError = '';
		try {
			const res = await api.get<PoolOffsetPage>('/pools?limit=200&sort=name&order=asc');
			pools = res.items ?? [];
		} catch {
			loadError = 'Failed to load pools';
		} finally {
			loading = false;
		}
	}

	let filtered = $derived(
		search.trim()
			? pools.filter((p) => p.name?.toLowerCase().includes(search.toLowerCase()))
			: pools
	);

	// Snap the highlight back to the top whenever the result set changes.
	$effect(() => {
		filtered;
		highlight = 0;
	});

	function moveHighlight(delta: number) {
		const n = filtered.length;
		if (n === 0) return;
		highlight = Math.min(n - 1, Math.max(0, highlight + delta));
		void tick().then(() =>
			listEl?.querySelector('.picker-item.highlighted')?.scrollIntoView({ block: 'nearest' })
		);
	}

	// Keyboard control for the open picker: arrows move the highlight, Enter adds
	// to the highlighted pool, "/" jumps to the search box, and Escape clears the
	// search first, then closes.
	function onKeydown(e: KeyboardEvent) {
		switch (e.key) {
			case 'Escape':
				e.preventDefault();
				if (search) search = '';
				else onClose();
				return;
			case '/':
				// Don't steal "/" while typing in the box — let it filter literally.
				if (document.activeElement !== searchEl) {
					e.preventDefault();
					searchEl?.focus();
					searchEl?.select();
				}
				return;
			case 'ArrowDown':
				e.preventDefault();
				moveHighlight(1);
				return;
			case 'ArrowUp':
				e.preventDefault();
				moveHighlight(-1);
				return;
			case 'Enter': {
				const pool = filtered[highlight];
				if (pool?.id) {
					e.preventDefault();
					void add(pool.id);
				}
				return;
			}
		}
	}

	async function add(poolId: string) {
		if (busy) return;
		busy = true;
		addError = '';
		try {
			await api.post(`/pools/${poolId}/files`, { file_ids: fileIds });
			onAdded?.(poolId);
			onClose();
		} catch {
			addError = 'Failed to add to pool';
			busy = false;
		}
	}

	let count = $derived(fileIds.length);
</script>

<svelte:window onkeydown={onKeydown} />

<!-- svelte-ignore a11y_click_events_have_key_events -->
<div class="picker-backdrop" role="presentation" onclick={onClose}></div>
<div class="picker-sheet" class:busy role="dialog" aria-label="Add to pool">
	<div class="picker-header">
		<span class="picker-title">Add {count} file{count !== 1 ? 's' : ''} to pool</span>
		<button class="picker-close" onclick={onClose} aria-label="Close">
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<path
					d="M3 3l10 10M13 3L3 13"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
				/>
			</svg>
		</button>
	</div>

	<div class="picker-search-wrap">
		<input
			class="picker-search"
			type="search"
			placeholder="Search pools…"
			bind:value={search}
			bind:this={searchEl}
			autocomplete="off"
		/>
	</div>

	{#if loading}
		<p class="picker-empty">Loading…</p>
	{:else if loadError}
		<p class="picker-error">{loadError}</p>
	{:else}
		{#if addError}
			<p class="picker-error">{addError}</p>
		{/if}
		{#if filtered.length === 0}
			<p class="picker-empty">No pools found.</p>
		{:else}
			<ul class="picker-list" bind:this={listEl}>
				{#each filtered as pool, i (pool.id)}
					<li>
						<button
							class="picker-item"
							class:highlighted={i === highlight}
							onmouseenter={() => (highlight = i)}
							onclick={() => pool.id && add(pool.id)}
						>
							<span class="picker-item-name">{pool.name}</span>
							<span class="picker-item-count">{pool.file_count ?? 0} files</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	{/if}
</div>

<style>
	.picker-backdrop {
		position: fixed;
		inset: 0;
		z-index: 110;
		background: rgba(0, 0, 0, 0.5);
	}

	.picker-sheet {
		position: fixed;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 111;
		background-color: var(--color-bg-secondary);
		border-radius: 14px 14px 0 0;
		padding-bottom: env(safe-area-inset-bottom, 0px);
		max-height: 70dvh;
		display: flex;
		flex-direction: column;
		animation: slide-up 0.18s ease-out;
	}

	.picker-sheet.busy {
		opacity: 0.6;
		pointer-events: none;
	}

	@keyframes slide-up {
		from {
			transform: translateY(20px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}

	.picker-header {
		display: flex;
		align-items: center;
		padding: 14px 16px 10px;
		gap: 8px;
	}

	.picker-title {
		flex: 1;
		font-size: 0.95rem;
		font-weight: 600;
	}

	.picker-close {
		background: none;
		border: none;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 4px;
		display: flex;
		align-items: center;
	}

	.picker-close:hover {
		color: var(--color-text-primary);
	}

	.picker-search-wrap {
		padding: 0 14px 10px;
	}

	.picker-search {
		width: 100%;
		box-sizing: border-box;
		height: 34px;
		padding: 0 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.9rem;
		font-family: inherit;
		outline: none;
	}

	.picker-search:focus {
		border-color: var(--color-accent);
	}

	.picker-list {
		list-style: none;
		margin: 0;
		padding: 0 8px 12px;
		overflow-y: auto;
		flex: 1;
	}

	.picker-item {
		display: flex;
		align-items: center;
		width: 100%;
		text-align: left;
		padding: 11px 10px;
		border-radius: 8px;
		background: none;
		border: none;
		cursor: pointer;
		font-family: inherit;
		gap: 8px;
	}

	.picker-item:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
	}

	.picker-item.highlighted {
		background-color: color-mix(in srgb, var(--color-accent) 22%, transparent);
	}

	.picker-item-name {
		flex: 1;
		font-size: 0.95rem;
		color: var(--color-text-primary);
	}

	.picker-item-count {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.picker-empty,
	.picker-error {
		text-align: center;
		padding: 20px;
		font-size: 0.9rem;
		color: var(--color-text-muted);
	}

	.picker-error {
		color: var(--color-danger);
	}
</style>
