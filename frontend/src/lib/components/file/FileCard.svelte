<script lang="ts">
	import { get } from 'svelte/store';
	import { authStore } from '$lib/stores/auth';
	import type { File } from '$lib/api/types';

	interface Props {
		file: File;
		onclick?: (file: File) => void;
	}

	let { file, onclick }: Props = $props();

	let imgSrc = $state<string | null>(null);
	let failed = $state(false);

	$effect(() => {
		const token = get(authStore).accessToken;
		let objectUrl: string | null = null;
		let cancelled = false;

		fetch(`/api/v1/files/${file.id}/thumbnail`, {
			headers: token ? { Authorization: `Bearer ${token}` } : {},
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

	function handleClick() {
		onclick?.(file);
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	class="card"
	class:loaded={!!imgSrc}
	onclick={handleClick}
	title={file.original_name ?? undefined}
>
	{#if imgSrc}
		<img src={imgSrc} alt={file.original_name ?? ''} class="thumb" />
	{:else if failed}
		<div class="placeholder failed" aria-label="Failed to load"></div>
	{:else}
		<div class="placeholder loading" aria-label="Loading"></div>
	{/if}
	<div class="overlay"></div>
</div>

<style>
	.card {
		position: relative;
		width: 160px;
		height: 160px;
		max-width: calc(33vw - 7px);
		max-height: calc(33vw - 7px);
		overflow: hidden;
		cursor: pointer;
		background-color: var(--color-bg-elevated);
		flex-shrink: 0;
	}

	.thumb {
		width: 100%;
		height: 100%;
		object-fit: contain;
		object-position: center;
		display: block;
	}

	.placeholder {
		width: 100%;
		height: 100%;
	}

	.placeholder.loading {
		background: linear-gradient(
			90deg,
			var(--color-bg-elevated) 25%,
			color-mix(in srgb, var(--color-accent) 12%, var(--color-bg-elevated)) 50%,
			var(--color-bg-elevated) 75%
		);
		background-size: 200% 100%;
		animation: shimmer 1.4s infinite;
	}

	.placeholder.failed {
		background-color: color-mix(in srgb, var(--color-danger) 15%, var(--color-bg-elevated));
	}

	.overlay {
		position: absolute;
		inset: 0;
		background-color: rgba(0, 0, 0, 0.1);
		transition: background-color 0.15s;
	}

	.card:hover .overlay {
		background-color: rgba(0, 0, 0, 0.3);
	}

	@keyframes shimmer {
		0% { background-position: 200% 0; }
		100% { background-position: -200% 0; }
	}
</style>