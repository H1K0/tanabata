<script lang="ts">
	// Recursive editor for one level of the metadata object. Nested objects render
	// another instance of this component (self-import), indented under their key.
	import Self from './MetadataEditor.svelte';
	import {
		type MetaNode,
		newValueNode,
		newObjectNode,
		nodesToObject,
		objectToNodes,
		parseObject
	} from '$lib/utils/metadata';

	interface Props {
		/** Entries at this level; bound so nested edits flow back to the parent. */
		nodes: MetaNode[];
		/** Fired on any structural or value change (parent marks the form dirty). */
		onchange: () => void;
	}

	let { nodes = $bindable(), onchange }: Props = $props();

	function addValue() {
		nodes = [...nodes, newValueNode()];
		onchange();
	}

	function addObject() {
		nodes = [...nodes, newObjectNode()];
		onchange();
	}

	function remove(id: number) {
		nodes = nodes.filter((n) => n.id !== id);
		onchange();
	}

	// Flip a leaf to a nested object and back. Converting keeps the data where it
	// can: a leaf whose text is a JSON object expands into rows; a group collapses
	// back to its JSON text.
	function toggleKind(node: MetaNode) {
		if (node.kind === 'value') {
			const obj = parseObject(node.value);
			node.children = obj ? objectToNodes(obj) : [];
			node.value = '';
			node.kind = 'object';
		} else {
			node.value = node.children.length ? JSON.stringify(nodesToObject(node.children)) : '';
			node.children = [];
			node.kind = 'value';
		}
		onchange();
	}
</script>

<div class="meta-editor">
	{#each nodes as node (node.id)}
		<div class="node">
			<div class="node-head">
				<input class="key" placeholder="key" bind:value={node.key} oninput={onchange} />
				<button
					class="kind"
					class:obj={node.kind === 'object'}
					onclick={() => toggleKind(node)}
					title={node.kind === 'object'
						? 'Nested object — click for a plain value'
						: 'Plain value — click to nest an object'}
					aria-label="Toggle value / nested object"
				>
					{node.kind === 'object' ? '{ }' : 'a'}
				</button>
				{#if node.kind === 'value'}
					<input class="val" placeholder="value" bind:value={node.value} oninput={onchange} />
				{/if}
				<button
					class="del"
					onclick={() => remove(node.id)}
					aria-label="Remove field"
					title="Remove field"
				>
					<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
						<path
							d="M3 3l8 8M11 3l-8 8"
							stroke="currentColor"
							stroke-width="1.6"
							stroke-linecap="round"
						/>
					</svg>
				</button>
			</div>
			{#if node.kind === 'object'}
				<div class="children">
					<Self bind:nodes={node.children} {onchange} />
				</div>
			{/if}
		</div>
	{/each}

	<div class="add-row">
		<button class="add" onclick={addValue}>+ Field</button>
		<button class="add" onclick={addObject}>+ Group</button>
	</div>
</div>

<style>
	.meta-editor {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.node {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.node-head {
		display: flex;
		gap: 6px;
		align-items: center;
	}

	.key,
	.val {
		box-sizing: border-box;
		height: 34px;
		padding: 0 9px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.85rem;
		font-family: inherit;
		outline: none;
		min-width: 0;
	}

	.key:focus,
	.val:focus {
		border-color: var(--color-accent);
	}

	.key {
		flex: 0 0 38%;
	}

	.val {
		flex: 1 1 auto;
	}

	/* Type toggle: shows 'a' for a plain value, '{ }' for a nested object. */
	.kind {
		flex-shrink: 0;
		width: 34px;
		height: 34px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		font-size: 0.8rem;
		font-family: inherit;
		cursor: pointer;
	}

	.kind.obj {
		color: var(--color-accent);
		border-color: var(--color-accent);
	}

	.kind:hover {
		border-color: var(--color-accent);
	}

	.del {
		flex-shrink: 0;
		width: 30px;
		height: 30px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 6px;
		border: none;
		background: none;
		color: var(--color-text-muted);
		cursor: pointer;
	}

	.del:hover {
		color: var(--color-danger);
		background-color: color-mix(in srgb, var(--color-danger) 12%, transparent);
	}

	/* Nested level: indent and hang a rail off the parent key. */
	.children {
		margin-left: 12px;
		padding-left: 12px;
		border-left: 2px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
	}

	.add-row {
		display: flex;
		gap: 6px;
	}

	.add {
		padding: 5px 11px;
		border-radius: 6px;
		border: 1px dashed color-mix(in srgb, var(--color-accent) 40%, transparent);
		background: none;
		color: var(--color-accent);
		font-size: 0.78rem;
		font-family: inherit;
		cursor: pointer;
	}

	.add:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
	}
</style>
