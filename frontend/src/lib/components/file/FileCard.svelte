<script lang="ts">
	import { get } from 'svelte/store';
	import { authStore } from '$lib/stores/auth';
	import type { File } from '$lib/api/types';

	const LONG_PRESS_MS = 400;
	const DRAG_THRESHOLD = 8; // px — cancel long-press if pointer moves more than this

	interface Props {
		file: File;
		index: number;
		selected?: boolean;
		selectionMode?: boolean;
		onTap?: (e: MouseEvent) => void;
		/** Called when long-press fires; receives the pointerType of the gesture. */
		onLongPress?: (pointerType: string) => void;
	}

	let {
		file,
		index,
		selected = false,
		selectionMode = false,
		onTap,
		onLongPress,
	}: Props = $props();

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

	// --- Long press + drag detection ---
	let pressTimer: ReturnType<typeof setTimeout> | null = null;
	let didLongPress = false;
	let pressStartX = 0;
	let pressStartY = 0;
	let currentPointerType = '';

	function onPointerDown(e: PointerEvent) {
		if (e.button !== 0 && e.pointerType === 'mouse') return;
		didLongPress = false;
		pressStartX = e.clientX;
		pressStartY = e.clientY;
		currentPointerType = e.pointerType;
		pressTimer = setTimeout(() => {
			didLongPress = true;
			onLongPress?.(currentPointerType);
		}, LONG_PRESS_MS);
	}

	function onPointerMoveInternal(e: PointerEvent) {
		// Cancel long-press if pointer has moved significantly (user is scrolling)
		if (pressTimer !== null) {
			const dx = e.clientX - pressStartX;
			const dy = e.clientY - pressStartY;
			if (Math.hypot(dx, dy) > DRAG_THRESHOLD) {
				clearTimeout(pressTimer);
				pressTimer = null;
			}
		}
	}

	function cancelPress() {
		if (pressTimer !== null) {
			clearTimeout(pressTimer);
			pressTimer = null;
		}
	}

	function onClick(e: MouseEvent) {
		if (didLongPress) {
			didLongPress = false;
			return;
		}
		cancelPress();
		onTap?.(e);
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div
	class="card"
	class:loaded={!!imgSrc}
	class:selected
	data-file-index={index}
	onpointerdown={onPointerDown}
	onpointermove={onPointerMoveInternal}
	onpointerup={() => { cancelPress(); didLongPress = false; }}
	onpointerleave={cancelPress}
	oncontextmenu={(e) => e.preventDefault()}
	onclick={onClick}
	title={file.original_name ?? undefined}
>
	{#if imgSrc}
		<img src={imgSrc} alt={file.original_name ?? ''} class="thumb" draggable="false" />
	{:else if failed}
		<div class="placeholder failed" aria-label="Failed to load"></div>
	{:else}
		<div class="placeholder loading" aria-label="Loading"></div>
	{/if}
	<div class="overlay"></div>
	{#if selected}
		<div class="check" aria-hidden="true">
			<svg width="18" height="18" viewBox="0 0 18 18" fill="none">
				<circle cx="9" cy="9" r="8.5" fill="rgba(0,0,0,0.55)" stroke="white" stroke-width="1"/>
				<path d="M5 9l3 3 5-5" stroke="white" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
			</svg>
		</div>
	{:else if selectionMode}
		<div class="check" aria-hidden="true">
			<svg width="18" height="18" viewBox="0 0 18 18" fill="none">
				<circle cx="9" cy="9" r="8.5" fill="rgba(0,0,0,0.35)" stroke="rgba(255,255,255,0.5)" stroke-width="1"/>
			</svg>
		</div>
	{/if}
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
		user-select: none;
		-webkit-user-select: none;
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

	.card.selected .overlay {
		background-color: color-mix(in srgb, var(--color-accent) 35%, transparent);
	}

	.check {
		position: absolute;
		top: 6px;
		right: 6px;
		pointer-events: none;
	}

	@keyframes shimmer {
		0% { background-position: 200% 0; }
		100% { background-position: -200% 0; }
	}
</style>