<script lang="ts">
	import { get } from 'svelte/store';
	import { authStore } from '$lib/stores/auth';

	interface Props {
		/** File id whose thumbnail to load. */
		id: string;
		alt?: string;
		/** Square edge length in px. */
		size?: number;
	}

	let { id, alt = '', size = 96 }: Props = $props();

	let imgSrc = $state<string | null>(null);
	let failed = $state(false);
	// Gate the fetch on visibility. A duplicate cluster can hold hundreds of files,
	// and firing every thumbnail request on mount buries the server in a request
	// storm (10k+ in-flight is easy). We only load once the tile nears the viewport.
	let visible = $state(false);

	// Svelte action: flips `visible` true the first time the tile nears the
	// viewport, then stops observing — the blob is kept once loaded.
	function lazyload(node: HTMLElement) {
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0]?.isIntersecting) {
					visible = true;
					observer.disconnect();
				}
			},
			{ rootMargin: '200px' }
		);
		observer.observe(node);
		return {
			destroy() {
				observer.disconnect();
			}
		};
	}

	// Thumbnails are auth-gated, so fetch with the bearer token and render the blob
	// (mirrors FileCard's loader). Runs once visible; re-runs whenever the id changes.
	$effect(() => {
		if (!visible) return;
		const token = get(authStore).accessToken;
		const currentId = id; // track id so a reused node refetches on change
		let objectUrl: string | null = null;
		let cancelled = false;
		imgSrc = null;
		failed = false;

		fetch(`/api/v1/files/${currentId}/thumbnail`, {
			headers: token ? { Authorization: `Bearer ${token}` } : {}
		})
			.then((res) => (res.ok ? res.blob() : null))
			.then((blob) => {
				if (cancelled || !blob) {
					if (!cancelled) failed = true;
					return;
				}
				objectUrl = URL.createObjectURL(blob);
				imgSrc = objectUrl;
			})
			.catch(() => {
				if (!cancelled) failed = true;
			});

		return () => {
			cancelled = true;
			if (objectUrl) URL.revokeObjectURL(objectUrl);
		};
	});
</script>

<div class="thumb" use:lazyload style="width:{size}px;height:{size}px">
	{#if imgSrc}
		<img src={imgSrc} {alt} draggable="false" />
	{:else if failed}
		<div class="ph failed" aria-label="Failed to load"></div>
	{:else}
		<div class="ph loading" aria-label="Loading"></div>
	{/if}
</div>

<style>
	.thumb {
		overflow: hidden;
		border-radius: 8px;
		background-color: var(--color-bg-elevated);
		flex-shrink: 0;
	}
	img {
		width: 100%;
		height: 100%;
		object-fit: contain;
		display: block;
	}
	.ph {
		width: 100%;
		height: 100%;
	}
	.ph.loading {
		background: linear-gradient(
			90deg,
			var(--color-bg-elevated) 25%,
			color-mix(in srgb, var(--color-accent) 12%, var(--color-bg-elevated)) 50%,
			var(--color-bg-elevated) 75%
		);
		background-size: 200% 100%;
		animation: shimmer 1.4s infinite;
	}
	.ph.failed {
		background-color: color-mix(in srgb, var(--color-danger) 15%, var(--color-bg-elevated));
	}
	@keyframes shimmer {
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}
</style>
