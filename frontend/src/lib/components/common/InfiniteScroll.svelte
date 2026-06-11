<script lang="ts">
	interface Props {
		loading?: boolean;
		hasMore?: boolean;
		onLoadMore: () => void;
		/** Which edge to watch: 'bottom' loads on scroll down, 'top' on scroll up. */
		edge?: 'top' | 'bottom';
	}

	let { loading = false, hasMore = true, onLoadMore, edge = 'bottom' }: Props = $props();

	// Lookahead distance past the viewport edge at which we start loading.
	const MARGIN = 300;

	let sentinel = $state<HTMLDivElement | undefined>();

	// True while the sentinel is within MARGIN px of the watched viewport edge.
	// Measuring the sentinel's viewport rect (rather than a scroll container's
	// scrollHeight/clientHeight) makes this correct whether the page scrolls on
	// <main> or on the window, and loads only enough to reach past the viewport.
	function nearViewport(): boolean {
		if (!sentinel) return false;
		const rect = sentinel.getBoundingClientRect();
		return edge === 'bottom'
			? rect.top <= window.innerHeight + MARGIN
			: rect.bottom >= -MARGIN;
	}

	function maybeLoad() {
		if (loading || !hasMore || !sentinel) return;
		if (nearViewport()) onLoadMore();
	}

	// Load on scroll. We watch the actual scroll position rather than relying on an
	// IntersectionObserver, which fires only on enter/leave transitions: a scroll
	// that *ends* with the sentinel already in range (e.g. scrolling straight to the
	// bottom) produces no new observer callback, so nothing loads until the user
	// scrolls back up and down to force a fresh transition. Re-checking the sentinel
	// on every scroll is what reliably keeps the list growing.
	//
	// `capture: true` is required because scroll events don't bubble — capturing lets
	// a single window listener catch scrolls from any nested scroll container (here
	// the grid's <main>) as well as the document itself. rAF-throttled so it stays
	// cheap (one getBoundingClientRect per frame at most).
	$effect(() => {
		let scheduled = false;
		const onScroll = () => {
			if (scheduled) return;
			scheduled = true;
			requestAnimationFrame(() => {
				scheduled = false;
				maybeLoad();
			});
		};
		window.addEventListener('scroll', onScroll, { passive: true, capture: true });
		window.addEventListener('resize', onScroll, { passive: true });
		return () => {
			window.removeEventListener('scroll', onScroll, { capture: true });
			window.removeEventListener('resize', onScroll);
		};
	});

	// Re-check after mount and after each load settles (loading → false): if the
	// freshly added content still didn't push the sentinel past the viewport, load
	// again. This fills short pages and covers the sentinel already being in range on
	// first render, without waiting for a scroll.
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
