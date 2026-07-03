<script lang="ts">
	import type { Tag } from '$lib/api/types';

	interface Props {
		tag: Tag;
		onclick?: () => void;
		size?: 'sm' | 'md';
		/** Roving keyboard-focus ring (shown only during keyboard navigation). */
		focused?: boolean;
		/** Position in a roving-focus grid; exposed as data-item-index for nav. */
		index?: number;
	}

	let { tag, onclick, size = 'md', focused = false, index }: Props = $props();

	let color = $derived(tag.color ?? tag.category_color);
	let style = $derived(color ? `background-color: #${color}` : '');
</script>

{#if onclick}
	<button
		class="badge {size}"
		class:focused
		{style}
		{onclick}
		type="button"
		data-item-index={index}
	>
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

	.badge.focused {
		outline: 2px solid var(--color-text-primary);
		outline-offset: 2px;
		/* Keep the ring clear of the fixed bottom navbar when scrolled into view. */
		scroll-margin-bottom: calc(72px + env(safe-area-inset-bottom, 0px));
		scroll-margin-top: 52px;
	}
</style>
