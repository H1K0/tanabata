<script lang="ts">
	import type { Tag } from '$lib/api/types';

	interface Props {
		tag: Tag;
		onclick?: () => void;
		size?: 'sm' | 'md';
	}

	let { tag, onclick, size = 'md' }: Props = $props();

	const color = tag.color ?? tag.category_color;
	const style = color ? `background-color: #${color}` : '';
</script>

{#if onclick}
	<button class="badge {size}" {style} {onclick} type="button">
		{tag.name}
	</button>
{:else}
	<span class="badge {size}" {style}>{tag.name}</span>
{/if}

<style>
	.badge {
		display: inline-flex;
		align-items: center;
		border-radius: 5px;
		font-family: inherit;
		background-color: var(--color-tag-default);
		color: rgba(255, 255, 255, 0.9);
		white-space: nowrap;
		border: none;
		cursor: default;
	}

	.badge.md {
		height: 28px;
		padding: 0 10px;
		font-size: 0.85rem;
	}

	.badge.sm {
		height: 22px;
		padding: 0 7px;
		font-size: 0.75rem;
	}

	button.badge {
		cursor: pointer;
	}

	button.badge:hover {
		filter: brightness(1.15);
	}
</style>