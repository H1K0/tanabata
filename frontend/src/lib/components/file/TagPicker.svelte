<script lang="ts">
	import type { Tag } from '$lib/api/types';
	import { fetchAllTags, sortTags } from '$lib/api/tags';
	import { tagSorting } from '$lib/stores/sorting';

	interface Props {
		fileTags: Tag[];
		onAdd: (tagId: string) => Promise<void>;
		onRemove: (tagId: string) => Promise<void>;
	}

	let { fileTags, onAdd, onRemove }: Props = $props();

	let allTags = $state<Tag[]>([]);
	let search = $state('');
	let busy = $state(false);

	$effect(() => {
		fetchAllTags().then((all) => {
			allTags = all;
		});
	});

	let assignedIds = $derived(new Set(fileTags.map((t) => t.id)));

	let filteredAvailable = $derived(
		allTags.filter(
			(t) =>
				!assignedIds.has(t.id) &&
				(!search.trim() || t.name?.toLowerCase().includes(search.toLowerCase()))
		)
	);

	// Show a file's already-assigned tags in the user's chosen tag order too.
	let sortedAssigned = $derived(sortTags(fileTags, $tagSorting));

	let filteredAssigned = $derived(
		search.trim()
			? sortedAssigned.filter((t) => t.name?.toLowerCase().includes(search.toLowerCase()))
			: sortedAssigned
	);

	async function handleAdd(tagId: string) {
		if (busy) return;
		busy = true;
		try {
			await onAdd(tagId);
		} finally {
			busy = false;
		}
	}

	async function handleRemove(tagId: string) {
		if (busy) return;
		busy = true;
		try {
			await onRemove(tagId);
		} finally {
			busy = false;
		}
	}

	function tagStyle(tag: Tag) {
		const color = tag.color ?? tag.category_color;
		return color ? `background-color: #${color}` : '';
	}

	// ---- Keyboard navigation (from the search input) ----
	// ↓/↑ highlight a suggestion, Enter adds it (focus stays for chaining); with the
	// input empty, ←/→ walk the assigned pills and Del removes the focused one.
	let highlightIdx = $state(0);
	let assignedFocusIdx = $state(-1);

	$effect(() => {
		if (highlightIdx > filteredAvailable.length - 1) {
			highlightIdx = Math.max(0, filteredAvailable.length - 1);
		}
	});

	function onSearchKeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			assignedFocusIdx = -1;
			if (filteredAvailable.length) {
				highlightIdx = Math.min(highlightIdx + 1, filteredAvailable.length - 1);
			}
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			assignedFocusIdx = -1;
			highlightIdx = Math.max(highlightIdx - 1, 0);
		} else if (e.key === 'Enter') {
			const tag = filteredAvailable[highlightIdx];
			if (tag?.id) {
				e.preventDefault();
				void handleAdd(tag.id);
			}
		} else if (e.key === 'ArrowRight' && search === '') {
			e.preventDefault();
			const n = filteredAssigned.length;
			if (n) assignedFocusIdx = assignedFocusIdx < 0 ? 0 : Math.min(assignedFocusIdx + 1, n - 1);
		} else if (e.key === 'ArrowLeft' && search === '') {
			e.preventDefault();
			const n = filteredAssigned.length;
			if (n) assignedFocusIdx = assignedFocusIdx < 0 ? n - 1 : Math.max(assignedFocusIdx - 1, 0);
		} else if (e.key === 'Delete' && assignedFocusIdx >= 0) {
			const tag = filteredAssigned[assignedFocusIdx];
			if (tag?.id) {
				e.preventDefault();
				void handleRemove(tag.id);
				assignedFocusIdx = Math.min(assignedFocusIdx, filteredAssigned.length - 2);
			}
		}
	}
</script>

<div class="picker" class:busy>
	<!-- Assigned tags -->
	{#if fileTags.length > 0}
		<div class="section-label">Assigned</div>
		<div class="tag-row">
			{#each filteredAssigned as tag, i (tag.id)}
				<button
					class="tag assigned"
					class:kbfocus={assignedFocusIdx === i}
					style={tagStyle(tag)}
					onclick={() => handleRemove(tag.id!)}
					title="Remove tag"
				>
					{tag.name}
					<span class="remove">×</span>
				</button>
			{/each}
		</div>
	{/if}

	<!-- Search -->
	<div class="search-wrap">
		<input
			class="search"
			type="search"
			placeholder="Search tags…"
			bind:value={search}
			onkeydown={onSearchKeydown}
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

	<!-- Available tags -->
	{#if filteredAvailable.length > 0}
		<div class="section-label">Add tag</div>
		<div class="tag-row available-row">
			{#each filteredAvailable as tag, i (tag.id)}
				<button
					class="tag available"
					class:hl={highlightIdx === i}
					style={tagStyle(tag)}
					onclick={() => handleAdd(tag.id!)}
					title="Add tag"
				>
					{tag.name}
				</button>
			{/each}
		</div>
	{:else if search.trim()}
		<p class="empty">No matching tags</p>
	{/if}
</div>

<style>
	.picker {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.picker.busy {
		opacity: 0.6;
		pointer-events: none;
	}

	.section-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.tag-row {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
	}

	.available-row {
		max-height: 140px;
		overflow-y: auto;
	}

	.tag {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		height: 26px;
		padding: 0 9px;
		border-radius: 5px;
		font-size: 0.8rem;
		font-family: inherit;
		cursor: pointer;
		border: none;
		background-color: var(--color-tag-default);
		color: rgba(255, 255, 255, 0.9);
		user-select: none;
	}

	.tag.assigned {
		opacity: 0.95;
	}

	.tag.assigned:hover {
		filter: brightness(1.1);
	}

	.remove {
		font-size: 1rem;
		line-height: 1;
		opacity: 0.7;
	}

	.tag.available {
		opacity: 0.75;
	}

	.tag.available:hover {
		opacity: 1;
		filter: brightness(1.1);
	}

	.tag.available.hl {
		opacity: 1;
		outline: 2px solid var(--color-accent);
		outline-offset: 1px;
	}

	.tag.assigned.kbfocus {
		outline: 2px solid var(--color-danger);
		outline-offset: 1px;
	}

	.search-wrap {
		position: relative;
		display: flex;
		align-items: center;
	}

	.search {
		width: 100%;
		box-sizing: border-box;
		height: 32px;
		padding: 0 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.85rem;
		font-family: inherit;
		outline: none;
	}

	.search:focus {
		border-color: var(--color-accent);
	}

	.search-clear {
		position: absolute;
		right: 6px;
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

	.empty {
		font-size: 0.8rem;
		color: var(--color-text-muted);
		margin: 0;
	}
</style>
