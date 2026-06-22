<script lang="ts">
	import { get } from 'svelte/store';
	import { untrack } from 'svelte';
	import { authStore } from '$lib/stores/auth';
	import type { File } from '$lib/api/types';

	interface Props {
		/** Files to page through (e.g. a duplicate cluster). */
		files: File[];
		/** Id to open first. */
		startId: string;
		onClose: () => void;
	}

	let { files, startId, onClose }: Props = $props();

	// Resolve the starting index once. The lightbox is remounted on each open, so
	// the props are effectively init-only — untrack acknowledges that read.
	let index = $state(untrack(() => Math.max(0, files.findIndex((f) => f.id === startId))));
	let src = $state<string | null>(null);
	let failed = $state(false);

	let current = $derived(files[index]);
	let hasPrev = $derived(index > 0);
	let hasNext = $derived(index < files.length - 1);

	// Load the full preview — the same image the single-file viewer shows, so the
	// user can actually tell duplicates apart instead of squinting at thumbnails.
	// Auth-gated, rendered from a blob; re-runs when the index changes.
	$effect(() => {
		const f = files[index];
		if (!f) return;
		const token = get(authStore).accessToken;
		let objectUrl: string | null = null;
		let cancelled = false;
		src = null;
		failed = false;

		fetch(`/api/v1/files/${f.id}/preview`, {
			headers: token ? { Authorization: `Bearer ${token}` } : {}
		})
			.then((res) => (res.ok ? res.blob() : null))
			.then((blob) => {
				if (cancelled || !blob) {
					if (!cancelled) failed = true;
					return;
				}
				objectUrl = URL.createObjectURL(blob);
				src = objectUrl;
			})
			.catch(() => {
				if (!cancelled) failed = true;
			});

		return () => {
			cancelled = true;
			if (objectUrl) URL.revokeObjectURL(objectUrl);
		};
	});

	function prev() {
		if (hasPrev) index -= 1;
	}
	function next() {
		if (hasNext) index += 1;
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onClose();
		else if (e.key === 'ArrowLeft') prev();
		else if (e.key === 'ArrowRight') next();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<!-- Close only when the backdrop itself is clicked (not the image or controls);
     Escape and the × button close from anywhere. -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<div
	class="backdrop"
	role="dialog"
	aria-modal="true"
	aria-label="Enlarged preview"
	tabindex="-1"
	onclick={(e) => {
		if (e.target === e.currentTarget) onClose();
	}}
>
	<button class="close" onclick={onClose} aria-label="Close">
		<svg width="22" height="22" viewBox="0 0 22 22" fill="none" aria-hidden="true">
			<path
				d="M6 6l10 10M16 6L6 16"
				stroke="currentColor"
				stroke-width="2"
				stroke-linecap="round"
			/>
		</svg>
	</button>

	<div class="stage">
		{#if src}
			<img class="img" {src} alt={current?.original_name ?? ''} />
		{:else if failed}
			<div class="ph failed">Failed to load preview</div>
		{:else}
			<div class="ph loading">Loading…</div>
		{/if}

		{#if hasPrev}
			<button class="nav prev" onclick={prev} aria-label="Previous">
				<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
					<path
						d="M12 4L6 10L12 16"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
		{/if}
		{#if hasNext}
			<button class="nav next" onclick={next} aria-label="Next">
				<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
					<path
						d="M8 4L14 10L8 16"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
		{/if}
	</div>

	<div class="caption">
		<span class="name" title={current?.original_name ?? ''}>{current?.original_name ?? '—'}</span>
		<span class="pos">{index + 1} / {files.length}</span>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 100;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding: 16px;
		background-color: rgba(0, 0, 0, 0.88);
	}

	.close {
		position: absolute;
		top: 12px;
		right: 12px;
		width: 38px;
		height: 38px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 50%;
		border: none;
		background-color: rgba(0, 0, 0, 0.5);
		color: #fff;
		cursor: pointer;
	}
	.close:hover {
		background-color: rgba(0, 0, 0, 0.8);
	}

	.stage {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		max-width: 100%;
		min-height: 0;
		flex: 1;
	}

	.img {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain;
		display: block;
		border-radius: 6px;
	}

	.ph {
		display: flex;
		align-items: center;
		justify-content: center;
		min-width: 220px;
		min-height: 220px;
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}
	.ph.failed {
		color: var(--color-danger);
	}

	.nav {
		position: absolute;
		top: 50%;
		transform: translateY(-50%);
		width: 44px;
		height: 44px;
		border-radius: 50%;
		border: none;
		background-color: rgba(0, 0, 0, 0.55);
		color: #fff;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition: background-color 0.15s;
	}
	.nav:hover {
		background-color: rgba(0, 0, 0, 0.85);
	}
	.nav.prev {
		left: 8px;
	}
	.nav.next {
		right: 8px;
	}

	.caption {
		display: flex;
		align-items: center;
		gap: 10px;
		max-width: 100%;
		color: var(--color-text-primary);
		font-size: 0.85rem;
		flex-shrink: 0;
	}
	.caption .name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.caption .pos {
		color: var(--color-text-muted);
		flex-shrink: 0;
	}
</style>
