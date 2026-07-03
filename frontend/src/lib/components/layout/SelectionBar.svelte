<script lang="ts">
	interface Props {
		count: number;
		onClear: () => void;
		onEditTags: () => void;
		onAddToPool: () => void;
		onMarkReviewed: () => void;
		onDelete: () => void;
		// Optional: only shown in a pool context (removes the files from that pool
		// without deleting them). Omitted on the global files list.
		onRemoveFromPool?: () => void;
	}

	let {
		count,
		onClear,
		onEditTags,
		onAddToPool,
		onMarkReviewed,
		onDelete,
		onRemoveFromPool
	}: Props = $props();
</script>

<div class="bar" role="toolbar" aria-label="Selection actions">
	<div class="row">
		<!-- Count / deselect all -->
		<button class="count" onclick={onClear} title="Clear selection">
			<span class="num">{count}</span>
			<span class="label">selected</span>
			<svg
				class="close-icon"
				width="14"
				height="14"
				viewBox="0 0 14 14"
				fill="none"
				aria-hidden="true"
			>
				<path
					d="M2 2l10 10M12 2L2 12"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
				/>
			</svg>
		</button>

		<div class="spacer"></div>

		<button class="action edit-tags" onclick={onEditTags}>Edit tags</button>
		<button class="action add-pool" onclick={onAddToPool}>Add to pool</button>
		<button class="action mark-reviewed" onclick={onMarkReviewed}>Mark reviewed</button>
		{#if onRemoveFromPool}
			<button class="action remove-pool" onclick={onRemoveFromPool}>Remove from pool</button>
		{/if}
		<button class="action delete" onclick={onDelete}>Delete</button>
	</div>
</div>

<style>
	.bar {
		position: fixed;
		left: 10px;
		right: 10px;
		bottom: 65px;
		box-sizing: border-box;
		background-color: var(--color-bg-secondary);
		border-radius: 10px;
		box-shadow: 0 0 12px rgba(0, 0, 0, 0.5);
		padding: 12px 14px;
		z-index: 100;
		animation: slide-up 0.18s ease-out;
	}

	@keyframes slide-up {
		from {
			transform: translateY(12px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}

	.row {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 4px;
	}

	.spacer {
		flex: 1;
	}

	.count {
		display: flex;
		align-items: center;
		gap: 5px;
		background: none;
		border: none;
		cursor: pointer;
		padding: 4px 6px;
		border-radius: 6px;
		color: var(--color-text-muted);
		font-family: inherit;
	}

	.count:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
		color: var(--color-text-primary);
	}

	.num {
		font-size: 1.1rem;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.label {
		font-size: 0.85rem;
	}

	.close-icon {
		opacity: 0.5;
	}

	.count:hover .close-icon {
		opacity: 1;
	}

	.action {
		background: none;
		border: none;
		cursor: pointer;
		padding: 6px 10px;
		border-radius: 6px;
		font-size: 0.85rem;
		font-family: inherit;
		font-weight: 600;
		white-space: nowrap;
	}

	.edit-tags {
		color: var(--color-info);
	}

	.edit-tags:hover {
		background-color: color-mix(in srgb, var(--color-info) 15%, transparent);
	}

	.add-pool {
		color: var(--color-warning);
	}

	.add-pool:hover {
		background-color: color-mix(in srgb, var(--color-warning) 15%, transparent);
	}

	.mark-reviewed {
		color: var(--color-success);
	}

	.mark-reviewed:hover {
		background-color: color-mix(in srgb, var(--color-success) 15%, transparent);
	}

	.remove-pool {
		color: var(--color-text-muted);
	}

	.remove-pool:hover {
		background-color: color-mix(in srgb, var(--color-accent) 15%, transparent);
		color: var(--color-text-primary);
	}

	.delete {
		color: var(--color-danger);
	}

	.delete:hover {
		background-color: color-mix(in srgb, var(--color-danger) 15%, transparent);
	}
</style>
