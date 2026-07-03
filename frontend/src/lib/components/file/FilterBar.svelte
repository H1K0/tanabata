<script lang="ts">
	import { untrack } from 'svelte';
	import type { Tag } from '$lib/api/types';
	import { fetchAllTags } from '$lib/api/tags';
	import { buildDslFilter, parseDslFilter, tokenLabel } from '$lib/utils/dsl';

	interface Props {
		/** Current DSL filter string (e.g. "{t=uuid1,&,t=uuid2}"). */
		value?: string | null;
		onApply: (filter: string | null) => void;
		onClose: () => void;
	}

	let { value = null, onApply, onClose }: Props = $props();

	const OPERATORS = ['(', ')', '&', '|', '!'] as const;

	let tags = $state<Tag[]>([]);
	let search = $state('');
	// Seed from the prop once; the $effect below keeps it in sync afterwards, so
	// read it untracked to avoid the state-referenced-locally warning.
	let tokens = $state<string[]>(untrack(() => parseDslFilter(value)));
	let tagNames = $derived(
		new Map(tags.filter((t) => t.id && t.name).map((t) => [t.id as string, t.name as string]))
	);

	$effect(() => {
		tokens = parseDslFilter(value ?? null);
	});

	$effect(() => {
		fetchAllTags().then((all) => {
			tags = all;
		});
	});

	let filteredTags = $derived(
		search.trim() ? tags.filter((t) => t.name?.toLowerCase().includes(search.toLowerCase())) : tags
	);

	function addToken(t: string) {
		tokens = [...tokens, t];
	}

	// Free-text MIME filter. Matches against the type name (mt.name) via LIKE, so
	// "image/png" is an exact-ish match and "image/%" / "%mp4" act as patterns.
	// (m=<id> targets the numeric mime_id, which the UI doesn't expose.)
	let mimeInput = $state('');

	function addMime() {
		const v = mimeInput.trim();
		if (!v) return;
		addToken(`m~${v}`);
		mimeInput = '';
	}

	function removeToken(i: number) {
		tokens = tokens.filter((_, idx) => idx !== i);
	}

	// Review status is a single, mutually-exclusive r=1 / r=0 token; null = "any".
	let reviewToken = $derived(tokens.find((t) => t === 'r=1' || t === 'r=0') ?? null);

	function setReview(value: 'r=1' | 'r=0' | null) {
		const rest = tokens.filter((t) => t !== 'r=1' && t !== 'r=0');
		tokens = value ? [...rest, value] : rest;
	}

	function apply() {
		onApply(buildDslFilter(tokens));
	}

	function reset() {
		tokens = [];
		search = '';
		onApply(null);
	}

	// ---- Keyboard navigation (from the search input) ----
	// ↓/↑ highlight a tag, Enter adds it as a token; the operator chars insert an
	// operator token; with the input empty ←/→ walk the active tokens and Del
	// removes the focused one. Mod+Enter applies, Mod+Backspace resets, Esc closes.
	let highlightIdx = $state(0);
	let tokenFocusIdx = $state(-1);
	const OP_KEYS = ['&', '|', '!', '(', ')'];

	$effect(() => {
		if (highlightIdx > filteredTags.length - 1) {
			highlightIdx = Math.max(0, filteredTags.length - 1);
		}
	});

	function onSearchKeydown(e: KeyboardEvent) {
		if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
			e.preventDefault();
			apply();
			return;
		}
		if ((e.ctrlKey || e.metaKey) && e.key === 'Backspace') {
			e.preventDefault();
			reset();
			return;
		}
		if (e.ctrlKey || e.metaKey || e.altKey) return;

		if (OP_KEYS.includes(e.key)) {
			e.preventDefault();
			addToken(e.key);
			return;
		}

		if (e.key === 'ArrowDown') {
			e.preventDefault();
			tokenFocusIdx = -1;
			if (filteredTags.length) highlightIdx = Math.min(highlightIdx + 1, filteredTags.length - 1);
		} else if (e.key === 'ArrowUp') {
			e.preventDefault();
			tokenFocusIdx = -1;
			highlightIdx = Math.max(highlightIdx - 1, 0);
		} else if (e.key === 'Enter') {
			const tag = filteredTags[highlightIdx];
			if (tag?.id) {
				e.preventDefault();
				addToken(`t=${tag.id}`);
			}
		} else if (e.key === 'ArrowRight' && search === '') {
			e.preventDefault();
			const n = tokens.length;
			if (n) tokenFocusIdx = tokenFocusIdx < 0 ? 0 : Math.min(tokenFocusIdx + 1, n - 1);
		} else if (e.key === 'ArrowLeft' && search === '') {
			e.preventDefault();
			const n = tokens.length;
			if (n) tokenFocusIdx = tokenFocusIdx < 0 ? n - 1 : Math.max(tokenFocusIdx - 1, 0);
		} else if (e.key === 'Delete' && tokenFocusIdx >= 0) {
			e.preventDefault();
			removeToken(tokenFocusIdx);
			tokenFocusIdx = Math.min(tokenFocusIdx, tokens.length - 2);
		} else if (e.key === 'Escape') {
			e.preventDefault();
			onClose();
		}
	}

	// --- Drag-and-drop reordering ---
	let dragIndex = $state<number | null>(null);
	let dropIndex = $state<number | null>(null);

	function onDragStart(i: number, e: DragEvent) {
		dragIndex = i;
		e.dataTransfer!.effectAllowed = 'move';
		// Set minimal drag image so the token itself acts as the ghost
		e.dataTransfer!.setData('text/plain', String(i));
	}

	function onDragOver(i: number, e: DragEvent) {
		e.preventDefault();
		e.dataTransfer!.dropEffect = 'move';
		dropIndex = i;
	}

	function onDrop(i: number, e: DragEvent) {
		e.preventDefault();
		if (dragIndex === null || dragIndex === i) return;
		const next = [...tokens];
		const [moved] = next.splice(dragIndex, 1);
		next.splice(i, 0, moved);
		tokens = next;
		dragIndex = null;
		dropIndex = null;
	}

	function onDragEnd() {
		dragIndex = null;
		dropIndex = null;
	}
</script>

<div class="bar">
	<!-- Active tokens -->
	<div class="active" class:empty={tokens.length === 0}>
		{#if tokens.length === 0}
			<span class="hint">No filter — tap a tag or operator below to build one</span>
		{:else}
			{#each tokens as token, i (i)}
				<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
				<div
					class="token active-token"
					class:dragging={dragIndex === i}
					class:drop-before={dropIndex === i && dragIndex !== null && dragIndex !== i}
					class:kbfocus={tokenFocusIdx === i}
					draggable="true"
					role="button"
					tabindex="0"
					title="Drag to reorder · Click to remove"
					ondragstart={(e) => onDragStart(i, e)}
					ondragover={(e) => onDragOver(i, e)}
					ondrop={(e) => onDrop(i, e)}
					ondragend={onDragEnd}
					onclick={() => removeToken(i)}
					onkeydown={(e) => e.key === 'Delete' && removeToken(i)}
				>
					{tokenLabel(token, tagNames)}
				</div>
			{/each}
		{/if}
	</div>

	<!-- Operator buttons -->
	<div class="ops">
		{#each OPERATORS as op}
			<button class="token op-token" onclick={() => addToken(op)}>{op}</button>
		{/each}
	</div>

	<!-- MIME / media type — appends an m~ token like a tag/operator -->
	<div class="mime">
		<button class="token mime-token" onclick={() => addToken('m~image/%')}>Images</button>
		<button class="token mime-token" onclick={() => addToken('m~video/%')}>Video</button>
		<input
			class="mime-input"
			type="text"
			placeholder="MIME, e.g. image/png"
			bind:value={mimeInput}
			onkeydown={(e) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					addMime();
				}
			}}
			autocomplete="off"
		/>
		<button class="token op-token mime-add" onclick={addMime} disabled={!mimeInput.trim()}>
			+ MIME
		</button>
	</div>

	<!-- Review status (mutually-exclusive r=1 / r=0) -->
	<div class="review-seg" role="group" aria-label="Review status">
		<button class="seg" class:on={reviewToken === null} onclick={() => setReview(null)}>Any</button>
		<button class="seg" class:on={reviewToken === 'r=1'} onclick={() => setReview('r=1')}>
			Needs review
		</button>
		<button class="seg" class:on={reviewToken === 'r=0'} onclick={() => setReview('r=0')}>
			Reviewed
		</button>
	</div>

	<!-- Tag search -->
	<input
		class="search"
		type="search"
		placeholder="Search tags…"
		bind:value={search}
		onkeydown={onSearchKeydown}
		autocomplete="off"
	/>

	<!-- Tag list -->
	<div class="tag-list">
		{#each filteredTags as tag, i (tag.id)}
			<button
				class="token tag-token"
				class:hl={highlightIdx === i}
				style="background-color: {tag.color
					? '#' + tag.color
					: tag.category_color
						? '#' + tag.category_color
						: 'var(--color-tag-default)'}"
				onclick={() => addToken(`t=${tag.id}`)}
			>
				{tag.name}
			</button>
		{:else}
			<span class="no-tags">{search ? 'No matching tags' : 'No tags yet'}</span>
		{/each}
	</div>

	<!-- Actions -->
	<div class="actions">
		<button class="btn btn-reset" onclick={reset}>Reset</button>
		<button class="btn btn-apply" onclick={apply}>Apply</button>
		<button class="btn btn-close" onclick={onClose}>Close</button>
	</div>
</div>

<style>
	.bar {
		background-color: var(--color-bg-elevated);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
		padding: 8px 10px;
		display: flex;
		flex-direction: column;
		gap: 8px;
		position: sticky;
		top: 43px; /* header height */
		z-index: 9;
	}

	.active {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		min-height: 32px;
		align-items: center;
	}

	.active.empty {
		opacity: 0.5;
	}

	.hint {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.ops {
		display: flex;
		gap: 4px;
	}

	.mime {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		align-items: center;
	}

	.mime-token {
		background-color: color-mix(in srgb, var(--color-accent) 18%, var(--color-bg-elevated));
		color: var(--color-text-primary);
		font-weight: 600;
	}

	.mime-token:hover {
		background-color: color-mix(in srgb, var(--color-accent) 35%, var(--color-bg-elevated));
	}

	.mime-input {
		flex: 1;
		min-width: 120px;
		box-sizing: border-box;
		height: 26px;
		padding: 0 8px;
		border-radius: 5px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.8rem;
		font-family: inherit;
		outline: none;
	}

	.mime-input:focus {
		border-color: var(--color-accent);
	}

	.mime-add:disabled {
		opacity: 0.4;
		cursor: default;
	}

	.review-seg {
		display: flex;
		gap: 2px;
		padding: 2px;
		border-radius: 7px;
		background-color: var(--color-bg-elevated);
		align-self: flex-start;
	}

	.seg {
		height: 24px;
		padding: 0 10px;
		border: none;
		border-radius: 5px;
		background: none;
		color: var(--color-text-muted);
		font-family: inherit;
		font-size: 0.78rem;
		font-weight: 600;
		cursor: pointer;
	}

	.seg:hover {
		color: var(--color-text-primary);
	}

	.seg.on {
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
	}

	.token {
		display: inline-flex;
		align-items: center;
		height: 26px;
		padding: 0 8px;
		border-radius: 5px;
		font-size: 0.8rem;
		cursor: pointer;
		border: none;
		font-family: inherit;
	}

	.active-token {
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-weight: 600;
		cursor: grab;
		user-select: none;
		transition:
			opacity 0.15s,
			outline 0.1s;
		outline: 2px solid transparent;
	}

	.active-token:hover {
		background-color: var(--color-accent-hover);
	}

	.active-token.dragging {
		opacity: 0.4;
		cursor: grabbing;
	}

	.active-token.drop-before {
		outline: 2px solid var(--color-accent);
		outline-offset: 2px;
	}

	.active-token.kbfocus {
		outline: 2px solid var(--color-danger);
		outline-offset: 2px;
	}

	.op-token {
		background-color: color-mix(in srgb, var(--color-accent) 18%, var(--color-bg-elevated));
		color: var(--color-text-primary);
		font-weight: 700;
		min-width: 30px;
		justify-content: center;
	}

	.op-token:hover {
		background-color: color-mix(in srgb, var(--color-accent) 35%, var(--color-bg-elevated));
	}

	.search {
		width: 100%;
		box-sizing: border-box;
		height: 30px;
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

	.tag-list {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
		max-height: 120px;
		overflow-y: auto;
	}

	.tag-token {
		color: rgba(255, 255, 255, 0.9);
	}

	.tag-token:hover {
		filter: brightness(1.15);
	}

	.tag-token.hl {
		outline: 2px solid var(--color-text-primary);
		outline-offset: 1px;
	}

	.no-tags {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.actions {
		display: flex;
		gap: 6px;
		justify-content: flex-end;
	}

	.btn {
		height: 30px;
		padding: 0 14px;
		border-radius: 6px;
		border: none;
		font-size: 0.85rem;
		font-family: inherit;
		cursor: pointer;
	}

	.btn-apply {
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-weight: 600;
	}

	.btn-apply:hover {
		background-color: var(--color-accent-hover);
	}

	.btn-reset {
		background-color: color-mix(in srgb, var(--color-danger) 20%, var(--color-bg-elevated));
		color: var(--color-text-primary);
	}

	.btn-reset:hover {
		background-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-bg-elevated));
	}

	.btn-close {
		background-color: color-mix(in srgb, var(--color-accent) 15%, var(--color-bg-elevated));
		color: var(--color-text-muted);
	}

	.btn-close:hover {
		color: var(--color-text-primary);
	}
</style>
