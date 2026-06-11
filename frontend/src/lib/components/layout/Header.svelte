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
		onUpload?: () => void;
		onTrash?: () => void;
	}

	let {
		sortOptions,
		sort,
		order,
		filterActive = false,
		onSortChange,
		onOrderToggle,
		onFilterToggle,
		onUpload,
		onTrash
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

	{#if onUpload}
		<button class="upload-btn icon-btn" onclick={onUpload} title="Upload files">
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<path
					d="M8 2v9M4 6l4-4 4 4"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
				<path d="M2 13h12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
			</svg>
		</button>
	{/if}

	{#if onTrash}
		<button class="icon-btn trash-btn" onclick={onTrash} title="Trash">
			<svg width="15" height="15" viewBox="0 0 15 15" fill="none" aria-hidden="true">
				<path
					d="M2 4h11M5 4V2.5h5V4M5.5 7v4.5M9.5 7v4.5M3 4l.8 9h7.4l.8-9"
					stroke="currentColor"
					stroke-width="1.6"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</button>
	{/if}

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

		<button
			class="icon-btn order-btn"
			onclick={onOrderToggle}
			title={order === 'asc' ? 'Ascending' : 'Descending'}
		>
			{#if order === 'asc'}
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path
						d="M4 10L8 6L12 10"
						stroke="currentColor"
						stroke-width="1.8"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			{:else}
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path
						d="M4 6L8 10L12 6"
						stroke="currentColor"
						stroke-width="1.8"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
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
				<path
					d="M2 4h12M4 8h8M6 12h4"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
				/>
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
