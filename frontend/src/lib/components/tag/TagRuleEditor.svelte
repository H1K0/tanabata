<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import type { Tag, TagOffsetPage, TagRule } from '$lib/api/types';
	import TagBadge from './TagBadge.svelte';

	interface Props {
		tagId: string;
		rules: TagRule[];
		onRulesChange: (rules: TagRule[]) => void;
	}

	let { tagId, rules, onRulesChange }: Props = $props();

	let allTags = $state<Tag[]>([]);
	let search = $state('');
	let busy = $state(false);
	let error = $state('');

	$effect(() => {
		api.get<TagOffsetPage>('/tags?limit=200&sort=name&order=asc').then((p) => {
			allTags = p.items ?? [];
		});
	});

	// IDs already used in rules
	let usedIds = $derived(new Set(rules.map((r) => r.then_tag_id)));

	let filteredTags = $derived(
		allTags.filter(
			(t) =>
				t.id !== tagId &&
				!usedIds.has(t.id) &&
				(!search.trim() || t.name?.toLowerCase().includes(search.toLowerCase())),
		),
	);

	function tagForId(id: string | undefined) {
		return allTags.find((t) => t.id === id);
	}

	async function addRule(thenTagId: string) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const rule = await api.post<TagRule>(`/tags/${tagId}/rules`, {
				then_tag_id: thenTagId,
				is_active: true,
				apply_to_existing: false,
			});
			onRulesChange([...rules, rule]);
			search = '';
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to add rule';
		} finally {
			busy = false;
		}
	}

	async function removeRule(thenTagId: string) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await api.delete(`/tags/${tagId}/rules/${thenTagId}`);
			onRulesChange(rules.filter((r) => r.then_tag_id !== thenTagId));
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to remove rule';
		} finally {
			busy = false;
		}
	}
</script>

<div class="editor" class:busy>
	<p class="desc">
		When this tag is applied, also apply:
	</p>

	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	<!-- Current rules -->
	{#if rules.length > 0}
		<div class="rule-list">
			{#each rules as rule (rule.then_tag_id)}
				{@const t = tagForId(rule.then_tag_id)}
				<div class="rule-row">
					{#if t}
						<TagBadge tag={t} size="sm" />
					{:else}
						<span class="unknown">{rule.then_tag_name ?? rule.then_tag_id}</span>
					{/if}
					<button
						class="remove-btn"
						onclick={() => removeRule(rule.then_tag_id!)}
						aria-label="Remove rule"
					>×</button>
				</div>
			{/each}
		</div>
	{:else}
		<p class="empty">No rules — when this tag is applied, nothing extra happens.</p>
	{/if}

	<!-- Add rule -->
	<div class="add-section">
		<div class="section-label">Add rule</div>
		<input
			class="search"
			type="search"
			placeholder="Search tags to add…"
			bind:value={search}
			autocomplete="off"
		/>
		{#if search.trim()}
			<div class="tag-pick">
				{#each filteredTags as t (t.id)}
					<TagBadge tag={t} size="sm" onclick={() => addRule(t.id!)} />
				{:else}
					<span class="empty">No matching tags</span>
				{/each}
			</div>
		{/if}
	</div>
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

	.desc {
		font-size: 0.82rem;
		color: var(--color-text-muted);
		margin: 0;
	}

	.rule-list {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-items: center;
	}

	.rule-row {
		display: inline-flex;
		align-items: center;
		gap: 2px;
	}

	.remove-btn {
		background: none;
		border: none;
		color: var(--color-text-muted);
		font-size: 1rem;
		line-height: 1;
		cursor: pointer;
		padding: 1px 3px;
		border-radius: 3px;
	}

	.remove-btn:hover {
		color: var(--color-danger);
	}

	.unknown {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		font-family: monospace;
	}

	.section-label {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 4px;
	}

	.add-section {
		display: flex;
		flex-direction: column;
		gap: 6px;
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

	.tag-pick {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
		max-height: 100px;
		overflow-y: auto;
	}

	.empty {
		font-size: 0.8rem;
		color: var(--color-text-muted);
		margin: 0;
	}

	.error {
		font-size: 0.8rem;
		color: var(--color-danger);
		margin: 0;
	}
</style>