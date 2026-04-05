<script lang="ts">
	import type { SortOrder } from '$lib/stores/sorting';
	import { selectionStore, selectionActive } from '$lib/stores/selection';

	interface Props {
		sortOptions: { value: string; label: string }[];
		sort: string;
		order: SortOrder;
		filterActive?: boolean;
		onSortChange: (sort: string) => void;
		onOrderToggle: () => void;
		onFilterToggle: () => void;
	}

	let {
		sortOptions,
		sort,
		order,
		filterActive = false,
		onSortChange,
		onOrderToggle,
		onFilterToggle,
	}: Props = $props();
</script>

<header>
	<button
		class="select-btn"
		class:active={$selectionActive}
		onclick={() => ($selectionActive ? selectionStore.exit() : selectionStore.enter())}
	>
		{$selectionActive ? 'Cancel' : 'Select'}
	</button>

	<div class="controls">
		<select
			class="sort-select"
			value={sort}
			onchange={(e) => onSortChange((e.currentTarget as HTMLSelectElement).value)}
		>
			{#each sortOptions as opt}
				<option value={opt.value}>{opt.label}</option>
			{/each}
		</select>

		<button class="icon-btn order-btn" onclick={onOrderToggle} title={order === 'asc' ? 'Ascending' : 'Descending'}>
			{#if order === 'asc'}
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path d="M4 10L8 6L12 10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			{:else}
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path d="M4 6L8 10L12 6" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			{/if}
		</button>

		<button
			class="icon-btn filter-btn"
			class:active={filterActive}
			onclick={onFilterToggle}
			title="Filter"
		>
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<path d="M2 4h12M4 8h8M6 12h4" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
			</svg>
		</button>
	</div>
</header>

<style>
	header {
		display: flex;
		align-items: center;
		padding: 6px 10px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		gap: 6px;
		flex-shrink: 0;
		position: sticky;
		top: 0;
		z-index: 10;
	}

	.select-btn {
		height: 30px;
		padding: 0 12px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		font-size: 0.85rem;
		font-family: inherit;
		cursor: pointer;
	}

	.select-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-accent);
	}

	.select-btn.active {
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-elevated));
		color: var(--color-accent);
		border-color: var(--color-accent);
	}

	.controls {
		display: flex;
		align-items: center;
		gap: 4px;
		margin-left: auto;
	}

	.sort-select {
		height: 30px;
		padding: 0 8px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.85rem;
		font-family: inherit;
		cursor: pointer;
		outline: none;
	}

	.sort-select:focus {
		border-color: var(--color-accent);
	}

	.icon-btn {
		width: 30px;
		height: 30px;
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

	.icon-btn.active {
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-elevated));
		color: var(--color-accent);
		border-color: var(--color-accent);
	}
</style>