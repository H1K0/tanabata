<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { ApiError } from '$lib/api/client';
	import FileCard from '$lib/components/file/FileCard.svelte';
	import FileUpload from '$lib/components/file/FileUpload.svelte';
	import FilterBar from '$lib/components/file/FilterBar.svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import SelectionBar from '$lib/components/layout/SelectionBar.svelte';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import { fileSorting, type FileSortField } from '$lib/stores/sorting';
	import { selectionStore, selectionActive } from '$lib/stores/selection';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import BulkTagEditor from '$lib/components/file/BulkTagEditor.svelte';
	import { parseDslFilter } from '$lib/utils/dsl';
	import type { File, FileCursorPage, Pool, PoolOffsetPage } from '$lib/api/types';
	import { appSettings } from '$lib/stores/appSettings';

	let uploader = $state<{ open: () => void } | undefined>();
	let confirmDeleteFiles = $state(false);

	// ---- Bulk tag editor ----
	let tagEditorOpen = $state(false);

	// ---- Add to pool picker ----
	let poolPickerOpen = $state(false);
	let pools = $state<Pool[]>([]);
	let poolsLoading = $state(false);
	let poolPickerSearch = $state('');
	let poolPickerError = $state('');

	async function openPoolPicker() {
		poolPickerOpen = true;
		poolPickerError = '';
		poolsLoading = true;
		poolPickerSearch = '';
		try {
			const res = await api.get<PoolOffsetPage>('/pools?limit=200&sort=name&order=asc');
			pools = res.items ?? [];
		} catch {
			poolPickerError = 'Failed to load pools';
		} finally {
			poolsLoading = false;
		}
	}

	async function addToPool(poolId: string) {
		const ids = [...$selectionStore.ids];
		poolPickerOpen = false;
		selectionStore.exit();
		try {
			await api.post(`/pools/${poolId}/files`, { file_ids: ids });
		} catch {
			// silently ignore
		}
	}

	let filteredPools = $derived(
		poolPickerSearch.trim()
			? pools.filter((p) => p.name?.toLowerCase().includes(poolPickerSearch.toLowerCase()))
			: pools
	);

	function handleUploaded(file: File) {
		files = [file, ...files];
	}

	let LIMIT = $derived($appSettings.fileLoadLimit);

	const FILE_SORT_OPTIONS = [
		{ value: 'created', label: 'Created' },
		{ value: 'content_datetime', label: 'Date taken' },
		{ value: 'original_name', label: 'Name' },
		{ value: 'mime', label: 'Type' },
	];

	let files = $state<File[]>([]);
	let nextCursor = $state<string | null>(null);
	let loading = $state(false);
	let hasMore = $state(true);
	let error = $state('');
	let filterOpen = $state(false);

	let filterParam = $derived(page.url.searchParams.get('filter'));
	let activeTokens = $derived(parseDslFilter(filterParam));
	let sortState = $derived($fileSorting);

	let resetKey = $derived(`${sortState.sort}|${sortState.order}|${filterParam ?? ''}`);
	let prevKey = $state('');

	$effect(() => {
		if (resetKey !== prevKey) {
			prevKey = resetKey;
			files = [];
			nextCursor = null;
			hasMore = true;
			error = '';
		}
	});

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({
				limit: String(LIMIT),
				sort: sortState.sort,
				order: sortState.order,
			});
			if (nextCursor) params.set('cursor', nextCursor);
			if (filterParam) params.set('filter', filterParam);
			const res = await api.get<FileCursorPage>(`/files?${params}`);
			files = [...files, ...(res.items ?? [])];
			nextCursor = res.next_cursor ?? null;
			hasMore = !!res.next_cursor;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load files';
			hasMore = false;
		} finally {
			loading = false;
		}
	}

	function applyFilter(filter: string | null) {
		const url = new URL(page.url);
		if (filter) {
			url.searchParams.set('filter', filter);
		} else {
			url.searchParams.delete('filter');
		}
		goto(url.toString(), { replaceState: true });
		filterOpen = false;
	}

	function openFile(file: File) {
		if (file.id) goto(`/files/${file.id}`);
	}

	// ---- Selection logic ----

	let lastSelectedIdx = $state<number | null>(null);

	function handleTap(file: File, idx: number, e: MouseEvent) {
		if (!$selectionActive) {
			openFile(file);
			return;
		}
		if (e.shiftKey && lastSelectedIdx !== null) {
			// Range-select between lastSelectedIdx and idx (desktop)
			const from = Math.min(lastSelectedIdx, idx);
			const to = Math.max(lastSelectedIdx, idx);
			for (let i = from; i <= to; i++) {
				if (files[i]?.id) selectionStore.select(files[i].id!);
			}
			lastSelectedIdx = idx;
		} else {
			if (file.id) selectionStore.toggle(file.id);
			lastSelectedIdx = idx;
		}
	}

	function handleLongPress(file: File, idx: number, pointerType: string) {
		// Determine drag mode from whether this card is already selected
		const alreadySelected = $selectionStore.ids.has(file.id!);
		if (alreadySelected) {
			selectionStore.deselect(file.id!);
			dragMode = 'deselect';
		} else {
			selectionStore.select(file.id!);
			dragMode = 'select';
		}
		lastSelectedIdx = idx;
		// Only enter drag-select for touch — shift+click covers desktop range selection
		if (pointerType === 'touch') dragSelecting = true;
	}

	// ---- Drag-to-select / deselect (touch only) ----
	// Entered only after a long-press (400ms stillness), so by the time we
	// add the touchmove listener the scroll gesture hasn't started yet.
	// A non-passive touchmove listener lets us call preventDefault() to block
	// scroll while the user slides their finger across cards.

	let dragSelecting = $state(false);
	let dragMode = $state<'select' | 'deselect'>('select');

	$effect(() => {
		if (!dragSelecting) return;

		function onTouchMove(e: TouchEvent) {
			e.preventDefault(); // block scroll while drag-selecting
			const touch = e.touches[0];
			const el = document.elementFromPoint(touch.clientX, touch.clientY);
			const card = el?.closest<HTMLElement>('[data-file-index]');
			if (!card) return;
			const idx = parseInt(card.dataset.fileIndex ?? '');
			if (isNaN(idx) || !files[idx]?.id) return;
			if (dragMode === 'select') {
				selectionStore.select(files[idx].id!);
			} else {
				selectionStore.deselect(files[idx].id!);
			}
			lastSelectedIdx = idx;
		}

		function onTouchEnd() {
			dragSelecting = false;
		}

		document.addEventListener('touchmove', onTouchMove, { passive: false });
		document.addEventListener('touchend', onTouchEnd);
		document.addEventListener('touchcancel', onTouchEnd);
		return () => {
			document.removeEventListener('touchmove', onTouchMove);
			document.removeEventListener('touchend', onTouchEnd);
			document.removeEventListener('touchcancel', onTouchEnd);
		};
	});
</script>

<svelte:head>
	<title>Files | Tanabata</title>
</svelte:head>

<div class="page">
	<Header
		sortOptions={FILE_SORT_OPTIONS}
		sort={sortState.sort}
		order={sortState.order}
		filterActive={activeTokens.length > 0 || filterOpen}
		onSortChange={(s) => fileSorting.setSort(s as FileSortField)}
		onOrderToggle={() => fileSorting.toggleOrder()}
		onFilterToggle={() => (filterOpen = !filterOpen)}
		onUpload={() => uploader?.open()}
	/>

	{#if filterOpen}
		<FilterBar
			value={filterParam}
			onApply={applyFilter}
			onClose={() => (filterOpen = false)}
		/>
	{/if}

	<FileUpload bind:this={uploader} onUploaded={handleUploaded}>
		<main>
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
				<div class="empty">No files yet.</div>
			{/if}
		</main>
	</FileUpload>
</div>

{#if $selectionActive}
	<SelectionBar
		onEditTags={() => (tagEditorOpen = true)}
		onAddToPool={openPoolPicker}
		onDelete={() => (confirmDeleteFiles = true)}
	/>
{/if}

{#if tagEditorOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="picker-backdrop" role="presentation" onclick={() => (tagEditorOpen = false)}></div>
	<div class="picker-sheet tag-sheet" role="dialog" aria-label="Edit tags">
		<div class="picker-header">
			<span class="picker-title">Edit tags — {$selectionStore.ids.size} file{$selectionStore.ids.size !== 1 ? 's' : ''}</span>
			<button class="picker-close" onclick={() => (tagEditorOpen = false)} aria-label="Close">
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path d="M3 3l10 10M13 3L3 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
				</svg>
			</button>
		</div>
		<div class="tag-sheet-body">
			<BulkTagEditor fileIds={[...$selectionStore.ids]} onDone={() => (tagEditorOpen = false)} />
		</div>
	</div>
{/if}

{#if poolPickerOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="picker-backdrop" role="presentation" onclick={() => (poolPickerOpen = false)}></div>
	<div class="picker-sheet" role="dialog" aria-label="Add to pool">
		<div class="picker-header">
			<span class="picker-title">Add {$selectionStore.ids.size} file{$selectionStore.ids.size !== 1 ? 's' : ''} to pool</span>
			<button class="picker-close" onclick={() => (poolPickerOpen = false)} aria-label="Close">
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path d="M3 3l10 10M13 3L3 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/>
				</svg>
			</button>
		</div>
		<div class="picker-search-wrap">
			<input
				class="picker-search"
				type="search"
				placeholder="Search pools…"
				bind:value={poolPickerSearch}
				autocomplete="off"
			/>
		</div>
		{#if poolPickerError}
			<p class="picker-error">{poolPickerError}</p>
		{:else if poolsLoading}
			<p class="picker-empty">Loading…</p>
		{:else if filteredPools.length === 0}
			<p class="picker-empty">No pools found.</p>
		{:else}
			<ul class="picker-list">
				{#each filteredPools as pool (pool.id)}
					<li>
						<button class="picker-item" onclick={() => pool.id && addToPool(pool.id)}>
							<span class="picker-item-name">{pool.name}</span>
							<span class="picker-item-count">{pool.file_count ?? 0} files</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
{/if}

{#if confirmDeleteFiles}
	<ConfirmDialog
		message={`Move ${$selectionStore.ids.size} file(s) to trash?`}
		confirmLabel="Move to trash"
		danger
		onConfirm={async () => {
			const ids = [...$selectionStore.ids];
			confirmDeleteFiles = false;
			selectionStore.exit();
			try {
				await api.post('/files/bulk/delete', { file_ids: ids });
				files = files.filter((f) => !ids.includes(f.id ?? ''));
			} catch {
				// silently ignore — file list already updated optimistically
			}
		}}
		onCancel={() => (confirmDeleteFiles = false)}
	/>
{/if}

<style>
	.page {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}

	main {
		flex: 1;
		overflow-y: auto;
		padding: 10px 10px calc(60px + 10px); /* clear fixed navbar */
	}

	.grid {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-content: flex-start;
		align-items: flex-start;
		gap: 2px;
	}

	/* phantom last item so justify-content doesn't stretch final row */
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

	/* ---- Tag editor sheet ---- */
	.tag-sheet {
		max-height: 80dvh;
	}

	.tag-sheet-body {
		padding: 0 14px 16px;
		overflow-y: auto;
		flex: 1;
	}

	/* ---- Pool picker ---- */
	.picker-backdrop {
		position: fixed;
		inset: 0;
		z-index: 110;
		background: rgba(0, 0, 0, 0.5);
	}

	.picker-sheet {
		position: fixed;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 111;
		background-color: var(--color-bg-secondary);
		border-radius: 14px 14px 0 0;
		padding-bottom: env(safe-area-inset-bottom, 0px);
		max-height: 70dvh;
		display: flex;
		flex-direction: column;
		animation: slide-up 0.18s ease-out;
	}

	@keyframes slide-up {
		from { transform: translateY(20px); opacity: 0; }
		to   { transform: translateY(0);    opacity: 1; }
	}

	.picker-header {
		display: flex;
		align-items: center;
		padding: 14px 16px 10px;
		gap: 8px;
	}

	.picker-title {
		flex: 1;
		font-size: 0.95rem;
		font-weight: 600;
	}

	.picker-close {
		background: none;
		border: none;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 4px;
		display: flex;
		align-items: center;
	}

	.picker-close:hover {
		color: var(--color-text-primary);
	}

	.picker-search-wrap {
		padding: 0 14px 10px;
	}

	.picker-search {
		width: 100%;
		box-sizing: border-box;
		height: 34px;
		padding: 0 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.9rem;
		font-family: inherit;
		outline: none;
	}

	.picker-search:focus {
		border-color: var(--color-accent);
	}

	.picker-list {
		list-style: none;
		margin: 0;
		padding: 0 8px 12px;
		overflow-y: auto;
		flex: 1;
	}

	.picker-item {
		display: flex;
		align-items: center;
		width: 100%;
		text-align: left;
		padding: 11px 10px;
		border-radius: 8px;
		background: none;
		border: none;
		cursor: pointer;
		font-family: inherit;
		gap: 8px;
	}

	.picker-item:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
	}

	.picker-item-name {
		flex: 1;
		font-size: 0.95rem;
		color: var(--color-text-primary);
	}

	.picker-item-count {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.picker-empty,
	.picker-error {
		text-align: center;
		padding: 20px;
		font-size: 0.9rem;
		color: var(--color-text-muted);
	}

	.picker-error {
		color: var(--color-danger);
	}
</style>
