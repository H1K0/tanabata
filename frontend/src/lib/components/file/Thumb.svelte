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

	// Thumbnails are auth-gated, so fetch with the bearer token and render the blob
	// (mirrors FileCard's loader). Re-runs whenever the id changes.
	$effect(() => {
		const token = get(authStore).accessToken;
		let objectUrl: string | null = null;
		let cancelled = false;
		imgSrc = null;
		failed = false;

		fetch(`/api/v1/files/${id}/thumbnail`, {
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

<div class="thumb" style="width:{size}px;height:{size}px">
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
