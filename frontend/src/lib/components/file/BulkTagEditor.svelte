<script lang="ts">
	import { api } from '$lib/api/client';
	import type { Tag, TagOffsetPage } from '$lib/api/types';

	interface Props {
		fileIds: string[];
		onDone: () => void;
	}

	let { fileIds, onDone }: Props = $props();

	// Tags present on ALL selected files
	let commonIds = $state(new Set<string>());
	// Tags present on SOME but not all selected files
	let partialIds = $state(new Set<string>());
	// All available tags from /tags
	let allTags = $state<Tag[]>([]);

	let search = $state('');
	let busy = $state(false);
	let loading = $state(true);
	let error = $state('');

	$effect(() => {
		load();
	});

	async function load() {
		loading = true;
		error = '';
		try {
			const [tagsRes, commonRes] = await Promise.all([
				api.get<TagOffsetPage>('/tags?limit=200&sort=name&order=asc'),
				api.post<{ common_tag_ids: string[]; partial_tag_ids: string[] }>(
					'/files/bulk/common-tags',
					{ file_ids: fileIds }
				)
			]);
			allTags = tagsRes.items ?? [];
			commonIds = new Set(commonRes.common_tag_ids ?? []);
			partialIds = new Set(commonRes.partial_tag_ids ?? []);
		} catch {
			error = 'Failed to load tags';
		} finally {
			loading = false;
		}
	}

	// Assigned = common + partial (shown in assigned section)
	let assignedIds = $derived(new Set([...commonIds, ...partialIds]));

	let assignedTags = $derived(
		allTags.filter(
			(t) =>
				assignedIds.has(t.id ?? '') &&
				(!search.trim() || t.name?.toLowerCase().includes(search.toLowerCase()))
		)
	);

	let availableTags = $derived(
		allTags.filter(
			(t) =>
				!assignedIds.has(t.id ?? '') &&
				(!search.trim() || t.name?.toLowerCase().includes(search.toLowerCase()))
		)
	);

	function tagStyle(tag: Tag) {
		const color = tag.color ?? tag.category_color;
		return color ? `background-color: #${color}` : '';
	}

	async function add(tagId: string) {
		if (busy) return;
		busy = true;
		try {
			await api.post('/files/bulk/tags', { file_ids: fileIds, action: 'add', tag_ids: [tagId] });
			commonIds = new Set([...commonIds, tagId]);
			partialIds.delete(tagId);
			partialIds = new Set(partialIds);
		} finally {
			busy = false;
		}
	}

	// Clicking a partial tag promotes it to common (adds to all files that don't have it)
	async function promotePartial(tagId: string) {
		if (busy) return;
		busy = true;
		try {
			await api.post('/files/bulk/tags', { file_ids: fileIds, action: 'add', tag_ids: [tagId] });
			commonIds = new Set([...commonIds, tagId]);
			partialIds.delete(tagId);
			partialIds = new Set(partialIds);
		} finally {
			busy = false;
		}
	}

	async function remove(tagId: string) {
		if (busy) return;
		busy = true;
		try {
			await api.post('/files/bulk/tags', { file_ids: fileIds, action: 'remove', tag_ids: [tagId] });
			commonIds.delete(tagId);
			partialIds.delete(tagId);
			commonIds = new Set(commonIds);
			partialIds = new Set(partialIds);
		} finally {
			busy = false;
		}
	}
</script>

<div class="editor" class:busy>
	{#if loading}
		<p class="status">Loading…</p>
	{:else if error}
		<p class="status err">{error}</p>
	{:else}
		<!-- Assigned tags -->
		{#if assignedTags.length > 0}
			<div class="section-label">
				Assigned
				<span class="hint">— partial tags shown with dashed border, click to apply to all</span>
			</div>
			<div class="tag-row">
				{#each assignedTags as tag (tag.id)}
					{@const isPartial = partialIds.has(tag.id ?? '')}
					<div class="tag-wrap">
						<button
							class="tag assigned"
							class:partial={isPartial}
							style={tagStyle(tag)}
							onclick={() => (isPartial ? promotePartial(tag.id!) : remove(tag.id!))}
							title={isPartial
								? 'Partial — click to add to all files'
								: 'Click to remove from all files'}
						>
							{tag.name}
							{#if isPartial}
								<span class="partial-icon" aria-label="partial">~</span>
							{:else}
								<span class="remove" aria-label="remove">×</span>
							{/if}
						</button>
					</div>
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
		{#if availableTags.length > 0}
			<div class="section-label">Add tag</div>
			<div class="tag-row available-row">
				{#each availableTags as tag (tag.id)}
					<button
						class="tag available"
						style={tagStyle(tag)}
						onclick={() => add(tag.id!)}
						title="Add to all selected files"
					>
						{tag.name}
					</button>
				{/each}
			</div>
		{:else if search.trim() && availableTags.length === 0 && assignedTags.length === 0}
			<p class="empty">No matching tags</p>
		{/if}
	{/if}
</div>

<style>
	.editor {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.editor.busy {
		opacity: 0.6;
		pointer-events: none;
	}

	.status {
		font-size: 0.85rem;
		color: var(--color-text-muted);
		margin: 0;
		padding: 8px 0;
	}

	.status.err {
		color: var(--color-danger);
	}

	.section-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.hint {
		font-weight: 400;
		text-transform: none;
		letter-spacing: 0;
		font-size: 0.72rem;
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

	.tag-wrap {
		display: contents;
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
		border: 2px solid transparent;
		background-color: var(--color-tag-default);
		color: rgba(255, 255, 255, 0.9);
		user-select: none;
	}

	/* Common tag — solid, slightly faded ×, full opacity */
	.tag.assigned {
		opacity: 0.95;
	}

	.tag.assigned:hover {
		filter: brightness(1.15);
	}

	/* Partial tag — dashed border, reduced opacity */
	.tag.assigned.partial {
		opacity: 0.65;
		border-style: dashed;
		border-color: rgba(255, 255, 255, 0.55);
	}

	.tag.assigned.partial:hover {
		opacity: 1;
		filter: brightness(1.1);
	}

	.remove {
		font-size: 1rem;
		line-height: 1;
		opacity: 0.7;
	}

	.partial-icon {
		font-size: 0.9rem;
		line-height: 1;
		opacity: 0.85;
	}

	.tag.available {
		opacity: 0.7;
	}

	.tag.available:hover {
		opacity: 1;
		filter: brightness(1.1);
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
