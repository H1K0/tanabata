<script lang="ts">
	import { api } from '$lib/api/client';
	import type { Tag, TagOffsetPage } from '$lib/api/types';
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
	let tokens = $state<string[]>(parseDslFilter(value));
	let tagNames = $derived(new Map(tags.filter((t) => t.id && t.name).map((t) => [t.id as string, t.name as string])));

	$effect(() => {
		tokens = parseDslFilter(value ?? null);
	});

	$effect(() => {
		api.get<TagOffsetPage>('/tags?limit=200&sort=name&order=asc').then((page) => {
			tags = page.items ?? [];
		});
	});

	let filteredTags = $derived(
		search.trim()
			? tags.filter((t) => t.name?.toLowerCase().includes(search.toLowerCase()))
			: tags,
	);

	function addToken(t: string) {
		tokens = [...tokens, t];
	}

	function removeToken(i: number) {
		tokens = tokens.filter((_, idx) => idx !== i);
	}

	function apply() {
		onApply(buildDslFilter(tokens));
	}

	function reset() {
		tokens = [];
		search = '';
		onApply(null);
	}
</script>

<div class="bar">
	<!-- Active tokens -->
	<div class="active" class:empty={tokens.length === 0}>
		{#if tokens.length === 0}
			<span class="hint">No filter — tap a tag or operator below to build one</span>
		{:else}
			{#each tokens as token, i (i)}
				<button class="token active-token" onclick={() => removeToken(i)} title="Remove">
					{tokenLabel(token, tagNames)}
				</button>
			{/each}
		{/if}
	</div>

	<!-- Operator buttons -->
	<div class="ops">
		{#each OPERATORS as op}
			<button class="token op-token" onclick={() => addToken(op)}>{op}</button>
		{/each}
	</div>

	<!-- Tag search -->
	<input
		class="search"
		type="search"
		placeholder="Search tags…"
		bind:value={search}
		autocomplete="off"
	/>

	<!-- Tag list -->
	<div class="tag-list">
		{#each filteredTags as tag (tag.id)}
			<button
				class="token tag-token"
				style="background-color: {tag.color ? '#' + tag.color : tag.category_color ? '#' + tag.category_color : 'var(--color-tag-default)'}"
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
	}

	.active-token:hover {
		background-color: var(--color-accent-hover);
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