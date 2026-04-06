<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import { tick } from 'svelte';
	import FileCard from '$lib/components/file/FileCard.svelte';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import { selectionStore, selectionActive, selectionCount } from '$lib/stores/selection';
	import { appSettings } from '$lib/stores/appSettings';
	import type { File, FileCursorPage } from '$lib/api/types';

	let scrollContainer = $state<HTMLElement | undefined>();

	let LIMIT = $derived($appSettings.fileLoadLimit);

	let files = $state<File[]>([]);
	let nextCursor = $state<string | null>(null);
	let loading = $state(false);
	let hasMore = $state(true);
	let error = $state('');
	let initialLoaded = $state(false);

	// confirmation dialogs
	let confirmRestore = $state(false);
	let confirmPermDelete = $state(false);
	let actionBusy = $state(false);

	$effect(() => {
		if (!initialLoaded && !loading) void loadMore();
	});

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ limit: String(LIMIT), trash: 'true' });
			if (nextCursor) params.set('cursor', nextCursor);
			const res = await api.get<FileCursorPage>(`/files?${params}`);
			files = [...files, ...(res.items ?? [])];
			nextCursor = res.next_cursor ?? null;
			hasMore = !!res.next_cursor;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load trash';
			hasMore = false;
		} finally {
			loading = false;
			initialLoaded = true;
		}
		await tick();
		if (hasMore && scrollContainer && scrollContainer.scrollHeight <= scrollContainer.clientHeight) {
			void loadMore();
		}
	}

	// ---- Selection ----
	let lastSelectedIdx = $state<number | null>(null);
	let dragSelecting = $state(false);
	let dragMode = $state<'select' | 'deselect'>('select');

	function handleTap(file: File, idx: number, e: MouseEvent) {
		// In trash, tap always selects (no detail page)
		if (e.shiftKey && lastSelectedIdx !== null) {
			const from = Math.min(lastSelectedIdx, idx);
			const to   = Math.max(lastSelectedIdx, idx);
			for (let i = from; i <= to; i++) {
				if (files[i]?.id) selectionStore.select(files[i].id!);
			}
		} else {
			if (!$selectionActive) selectionStore.enter();
			if (file.id) selectionStore.toggle(file.id);
		}
		lastSelectedIdx = idx;
	}

	function handleLongPress(file: File, idx: number, pointerType: string) {
		const alreadySelected = $selectionStore.ids.has(file.id!);
		if (alreadySelected) {
			selectionStore.deselect(file.id!);
			dragMode = 'deselect';
		} else {
			selectionStore.select(file.id!);
			dragMode = 'select';
		}
		lastSelectedIdx = idx;
		if (pointerType === 'touch') dragSelecting = true;
	}

	$effect(() => {
		if (!dragSelecting) return;
		function onTouchMove(e: TouchEvent) {
			e.preventDefault();
			const touch = e.touches[0];
			const el = document.elementFromPoint(touch.clientX, touch.clientY);
			const card = el?.closest<HTMLElement>('[data-file-index]');
			if (!card) return;
			const idx = parseInt(card.dataset.fileIndex ?? '');
			if (isNaN(idx) || !files[idx]?.id) return;
			if (dragMode === 'select') selectionStore.select(files[idx].id!);
			else selectionStore.deselect(files[idx].id!);
			lastSelectedIdx = idx;
		}
		function onTouchEnd() { dragSelecting = false; }
		document.addEventListener('touchmove', onTouchMove, { passive: false });
		document.addEventListener('touchend', onTouchEnd);
		document.addEventListener('touchcancel', onTouchEnd);
		return () => {
			document.removeEventListener('touchmove', onTouchMove);
			document.removeEventListener('touchend', onTouchEnd);
			document.removeEventListener('touchcancel', onTouchEnd);
		};
	});

	// ---- Actions ----
	async function restoreSelected() {
		const ids = [...$selectionStore.ids];
		confirmRestore = false;
		actionBusy = true;
		selectionStore.exit();
		try {
			await Promise.all(ids.map((id) => api.post(`/files/${id}/restore`, {})));
			files = files.filter((f) => !ids.includes(f.id ?? ''));
		} catch {
			// partial failure: reload
		} finally {
			actionBusy = false;
		}
	}

	async function permDeleteSelected() {
		const ids = [...$selectionStore.ids];
		confirmPermDelete = false;
		actionBusy = true;
		selectionStore.exit();
		try {
			await Promise.all(ids.map((id) => api.delete(`/files/${id}/permanent`)));
			files = files.filter((f) => !ids.includes(f.id ?? ''));
		} catch {
			// partial failure: reload
		} finally {
			actionBusy = false;
		}
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') selectionStore.exit();
	}
</script>

<svelte:head><title>Trash | Tanabata</title></svelte:head>
<svelte:window onkeydown={handleKeydown} />

<div class="page">
	<header>
		<button class="back-btn" onclick={() => { selectionStore.exit(); goto('/files'); }}>
			← Files
		</button>
		<span class="title">Trash</span>
		<button
			class="select-btn"
			class:active={$selectionActive}
			onclick={() => ($selectionActive ? selectionStore.exit() : selectionStore.enter())}
		>
			{$selectionActive ? 'Cancel' : 'Select'}
		</button>
	</header>

	<main bind:this={scrollContainer}>
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="grid">
			{#each files as file, i (file.id)}
				<FileCard
					{file}
					index={i}
					selected={$selectionStore.ids.has(file.id ?? '')}
					selectionMode={$selectionActive}
					onTap={(e) => handleTap(file, i, e)}
					onLongPress={(pt) => handleLongPress(file, i, pt)}
				/>
			{/each}
		</div>

		<InfiniteScroll {loading} {hasMore} onLoadMore={loadMore} />

		{#if !loading && !hasMore && files.length === 0}
			<div class="empty">Trash is empty.</div>
		{/if}
	</main>
</div>

{#if $selectionActive}
	<div class="sel-bar" role="toolbar" aria-label="Trash selection actions">
		<button class="sel-count" onclick={() => selectionStore.exit()} title="Clear selection">
			<span class="sel-num">{$selectionCount}</span>
			<span class="sel-label">selected</span>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
				<path d="M2 2l10 10M12 2L2 12" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
			</svg>
		</button>
		<div class="sel-spacer"></div>
		<button class="sel-action restore" onclick={() => (confirmRestore = true)} disabled={actionBusy}>
			Restore
		</button>
		<button class="sel-action perm-delete" onclick={() => (confirmPermDelete = true)} disabled={actionBusy}>
			Delete permanently
		</button>
	</div>
{/if}

{#if confirmRestore}
	<ConfirmDialog
		message={`Restore ${$selectionStore.ids.size} file(s)?`}
		confirmLabel="Restore"
		onConfirm={restoreSelected}
		onCancel={() => (confirmRestore = false)}
	/>
{/if}

{#if confirmPermDelete}
	<ConfirmDialog
		message={`Permanently delete ${$selectionStore.ids.size} file(s)? This cannot be undone.`}
		confirmLabel="Delete permanently"
		danger
		onConfirm={permDeleteSelected}
		onCancel={() => (confirmPermDelete = false)}
	/>
{/if}

<style>
	.page {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}

	header {
		display: flex;
		align-items: center;
		padding: 6px 10px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		gap: 8px;
		flex-shrink: 0;
		position: sticky;
		top: 0;
		z-index: 10;
	}

	.back-btn {
		background: none;
		border: none;
		color: var(--color-text-muted);
		font-size: 0.85rem;
		font-family: inherit;
		cursor: pointer;
		padding: 4px 8px;
		border-radius: 6px;
	}

	.back-btn:hover { color: var(--color-accent); }

	.title {
		font-size: 0.9rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.select-btn {
		margin-left: auto;
		height: 30px;
		padding: 0 12px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		font-size: 0.85rem;
		font-family: inherit;
		cursor: pointer;
	}

	.select-btn:hover { color: var(--color-text-primary); border-color: var(--color-accent); }

	.select-btn.active {
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-elevated));
		color: var(--color-accent);
		border-color: var(--color-accent);
	}

	main {
		flex: 1;
		overflow-y: auto;
		padding: 10px 10px calc(60px + 10px);
	}

	.grid {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-content: flex-start;
		align-items: flex-start;
		gap: 2px;
	}

	.grid::after {
		content: '';
		flex: auto;
	}

	.error {
		color: var(--color-danger);
		padding: 12px;
		font-size: 0.875rem;
	}

	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 60px 20px;
		font-size: 0.95rem;
	}

	/* ---- Trash selection bar ---- */
	.sel-bar {
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
		display: flex;
		align-items: center;
		gap: 4px;
		animation: slide-up 0.18s ease-out;
	}

	@keyframes slide-up {
		from { transform: translateY(12px); opacity: 0; }
		to   { transform: translateY(0);    opacity: 1; }
	}

	.sel-count {
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

	.sel-count:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
		color: var(--color-text-primary);
	}

	.sel-num {
		font-size: 1.1rem;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.sel-label { font-size: 0.85rem; }

	.sel-spacer { flex: 1; }

	.sel-action {
		background: none;
		border: none;
		cursor: pointer;
		padding: 6px 10px;
		border-radius: 6px;
		font-size: 0.85rem;
		font-family: inherit;
		font-weight: 600;
	}

	.sel-action:disabled { opacity: 0.5; cursor: default; }

	.sel-action.restore {
		color: #7ECBA1;
	}

	.sel-action.restore:hover:not(:disabled) {
		background-color: color-mix(in srgb, #7ECBA1 15%, transparent);
	}

	.sel-action.perm-delete {
		color: var(--color-danger);
	}

	.sel-action.perm-delete:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--color-danger) 15%, transparent);
	}
</style>