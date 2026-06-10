<script lang="ts">
	interface Props {
		loading?: boolean;
		hasMore?: boolean;
		onLoadMore: () => void;
	}

	let { loading = false, hasMore = true, onLoadMore }: Props = $props();

	// Lookahead distance below the viewport at which we start loading.
	const MARGIN = 300;

	let sentinel = $state<HTMLDivElement | undefined>();

	// Fire onLoadMore while the sentinel is within MARGIN px of the viewport
	// bottom. Measuring the sentinel's viewport rect (rather than a scroll
	// container's scrollHeight/clientHeight) makes this correct whether the page
	// scrolls on <main> or on the window — and it loads exactly enough pages to
	// reach past the viewport, instead of eagerly loading everything.
	function maybeLoad() {
		if (loading || !hasMore || !sentinel) return;
		const rect = sentinel.getBoundingClientRect();
		if (rect.top <= window.innerHeight + MARGIN) {
			onLoadMore();
		}
	}

	// Load on scroll: the observer notifies us when the sentinel nears the viewport.
	$effect(() => {
		if (!sentinel) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) maybeLoad();
			},
			{ rootMargin: `${MARGIN}px` },
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});

	// After each load settles (loading → false), re-check synchronously: if the
	// freshly appended content still didn't push the sentinel past the viewport,
	// load again. This fills short pages without the throttled observer lagging
	// and over-fetching.
	$effect(() => {
		if (!loading) maybeLoad();
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
