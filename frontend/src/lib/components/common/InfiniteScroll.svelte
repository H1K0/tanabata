<script lang="ts">
	interface Props {
		loading?: boolean;
		hasMore?: boolean;
		onLoadMore: () => void;
	}

	let { loading = false, hasMore = true, onLoadMore }: Props = $props();

	let sentinel = $state<HTMLDivElement | undefined>();

	$effect(() => {
		if (!sentinel) return;

		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting && !loading && hasMore) {
					onLoadMore();
				}
			},
			{ rootMargin: '300px' },
		);

		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<div bind:this={sentinel} class="sentinel" aria-hidden="true"></div>

{#if loading}
	<div class="loading-row">
		<span class="spinner" role="status" aria-label="Loading"></span>
	</div>
{/if}

<style>
	.sentinel {
		height: 1px;
	}

	.loading-row {
		display: flex;
		justify-content: center;
		align-items: center;
		padding: 24px 0;
	}

	.spinner {
		display: block;
		width: 32px;
		height: 32px;
		border: 3px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-top-color: var(--color-accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}
</style>