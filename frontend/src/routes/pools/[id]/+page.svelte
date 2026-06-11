<script lang="ts">
	import { page } from '$app/state';
	import { goto, pushState, replaceState } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import { tick } from 'svelte';
	import FileCard from '$lib/components/file/FileCard.svelte';
	import FileViewer from '$lib/components/file/FileViewer.svelte';
	import FilterBar from '$lib/components/file/FilterBar.svelte';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import { parseDslFilter } from '$lib/utils/dsl';
	import type {
		Pool,
		PoolFile,
		PoolFileCursorPage,
		File as FileType,
		FileCursorPage
	} from '$lib/api/types';

	let poolId = $derived(page.params.id);

	// ---- Pool metadata ----
	let pool = $state<Pool | null>(null);
	let name = $state('');
	let notes = $state('');
	let isPublic = $state(false);
	let loadError = $state('');
	let loaded = $state(false);
	let saving = $state(false);
	let deleting = $state(false);
	let saveError = $state('');
	let editOpen = $state(false);
	let confirmDelete = $state(false);

	// ---- Pool files ----
	let files = $state<PoolFile[]>([]);
	let nextCursor = $state<string | null>(null);
	let hasMore = $state(true);
	let filesLoading = $state(false);
	let filesError = $state('');

	let filterParam = $derived(page.url.searchParams.get('filter'));
	let filterOpen = $state(false);
	let activeTokens = $derived(parseDslFilter(filterParam));

	// ---- Selection (for removal) ----
	let selectedIds = $state(new Set<string>());
	let selectionMode = $derived(selectedIds.size > 0);
	let lastSelectedIdx = $state<number | null>(null);
	let confirmRemove = $state(false);

	// ---- Add files mode ----
	let addMode = $state(false);
	let addFiles = $state<FileType[]>([]);
	let addNextCursor = $state<string | null>(null);
	let addHasMore = $state(true);
	let addLoading = $state(false);
	let addSearch = $state('');
	let addSelected = $state(new Set<string>());
	let addSearchPrev = $state('');

	// ---- Drag-to-reorder (disabled when filter active) ----
	let canReorder = $derived(!filterParam);
	let dragSrcIdx = $state<number | null>(null);
	let dragOverIdx = $state<number | null>(null);
	let reorderPending = $state(false);

	const LIMIT = 50;

	// ---- Load pool ----
	$effect(() => {
		const id = poolId;
		loaded = false;
		loadError = '';
		files = [];
		nextCursor = null;
		hasMore = true;
		filesError = '';
		selectedIds = new Set();
		editOpen = false;
		void api
			.get<Pool>(`/pools/${id}`)
			.then((p) => {
				pool = p;
				name = p.name ?? '';
				notes = p.notes ?? '';
				isPublic = p.is_public ?? false;
				loaded = true;
			})
			.catch((e) => {
				loadError = e instanceof ApiError ? e.message : 'Failed to load pool';
			});
	});

	// Reset files when filter changes
	let filterKey = $derived(`${poolId}|${filterParam ?? ''}`);
	let prevFilterKey = $state('');
	$effect(() => {
		if (filterKey !== prevFilterKey) {
			prevFilterKey = filterKey;
			files = [];
			nextCursor = null;
			hasMore = true;
			filesError = '';
		}
	});

	// ---- Load pool files ----
	async function loadMore() {
		if (filesLoading || !hasMore) return;
		filesLoading = true;
		filesError = '';
		try {
			const params = new URLSearchParams({ limit: String(LIMIT) });
			if (nextCursor) params.set('cursor', nextCursor);
			if (filterParam) params.set('filter', filterParam);
			const res = await api.get<PoolFileCursorPage>(`/pools/${poolId}/files?${params}`);
			files = [...files, ...(res.items ?? [])] as PoolFile[];
			nextCursor = res.next_cursor ?? null;
			hasMore = !!res.next_cursor;
		} catch (e) {
			filesError = e instanceof ApiError ? e.message : 'Failed to load files';
			hasMore = false;
		} finally {
			filesLoading = false;
		}
	}

	// ---- Save pool ----
	async function save() {
		if (!name.trim() || saving) return;
		saving = true;
		saveError = '';
		try {
			const updated = await api.patch<Pool>(`/pools/${poolId}`, {
				name: name.trim(),
				notes: notes.trim() || null,
				is_public: isPublic
			});
			pool = updated;
			editOpen = false;
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	// ---- Delete pool ----
	async function doDeletePool() {
		confirmDelete = false;
		deleting = true;
		try {
			await api.delete(`/pools/${poolId}`);
			goto('/pools');
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to delete pool';
			deleting = false;
		}
	}

	// ---- Filter ----
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

	// ---- Selection ----
	function handleTap(file: PoolFile, idx: number, e: MouseEvent) {
		if (!selectionMode) {
			openFile(file);
			return;
		}
		if (e.shiftKey && lastSelectedIdx !== null) {
			const from = Math.min(lastSelectedIdx, idx);
			const to = Math.max(lastSelectedIdx, idx);
			for (let i = from; i <= to; i++) {
				if (files[i]?.id) selectedIds.add(files[i].id!);
			}
			selectedIds = new Set(selectedIds);
		} else {
			if (file.id) {
				if (selectedIds.has(file.id)) {
					selectedIds.delete(file.id);
				} else {
					selectedIds.add(file.id);
				}
				selectedIds = new Set(selectedIds);
			}
		}
		lastSelectedIdx = idx;
	}

	function handleLongPress(file: PoolFile, idx: number) {
		if (file.id) {
			selectedIds.add(file.id);
			selectedIds = new Set(selectedIds);
		}
		lastSelectedIdx = idx;
	}

	async function removeSelected() {
		confirmRemove = false;
		const ids = [...selectedIds];
		selectedIds = new Set();
		try {
			await api.post(`/pools/${poolId}/files/remove`, { file_ids: ids });
			files = files.filter((f) => !ids.includes(f.id ?? ''));
			if (pool) pool = { ...pool, file_count: Math.max(0, (pool.file_count ?? 0) - ids.length) };
		} catch {
			// silently ignore
		}
	}

	// ---- File viewer overlay (shallow routing) ----
	// Open the viewer on top of the still-mounted pool grid so the back button (and
	// the viewer's own close) returns here — with the pool's list and scroll intact —
	// instead of navigating to the standalone /files/<id> route, whose close drops
	// the user on the global files list. Neighbours follow the pool's own order.
	let activeFileId = $derived(page.state.fileId);
	let activeIdx = $derived(activeFileId ? files.findIndex((f) => f.id === activeFileId) : -1);
	let viewerPrevId = $derived(activeIdx > 0 ? (files[activeIdx - 1]?.id ?? null) : null);
	let viewerNextId = $derived(
		activeIdx >= 0 && activeIdx < files.length - 1 ? (files[activeIdx + 1]?.id ?? null) : null
	);

	function openFile(file: PoolFile) {
		if (!file.id) return;
		// Keep the pool URL; the overlay is driven by page.state, so a back press (or
		// a reload, which clears page.state) reveals the pool untouched.
		pushState(`${page.url.pathname}${page.url.search}`, { fileId: file.id });
	}

	function pageTo(id: string) {
		// Replace (not push) so one back press returns to the grid rather than
		// stepping back through every file paged.
		replaceState(`${page.url.pathname}${page.url.search}`, { fileId: id });
	}

	function closeViewer() {
		history.back();
	}

	// Page in more pool files when the viewer nears the end of the loaded set.
	$effect(() => {
		if (activeIdx >= 0 && activeIdx >= files.length - 3 && hasMore && !filesLoading) {
			void loadMore();
		}
	});

	// On close, bring the grid back to the last-viewed file (the list never unmounted).
	let lastOverlayId: string | null = null;
	$effect(() => {
		const id = activeFileId;
		if (id) {
			lastOverlayId = id;
		} else if (lastOverlayId) {
			const target = lastOverlayId;
			lastOverlayId = null;
			const idx = files.findIndex((f) => f.id === target);
			if (idx >= 0) {
				document
					.querySelector<HTMLElement>(`[data-file-index="${idx}"]`)
					?.scrollIntoView({ block: 'center' });
			}
		}
	});

	// ---- Drag-to-reorder ----
	function onDragStart(idx: number, e: DragEvent) {
		dragSrcIdx = idx;
		e.dataTransfer!.effectAllowed = 'move';
		e.dataTransfer!.setData('text/plain', String(idx));
	}

	function onDragOver(idx: number, e: DragEvent) {
		e.preventDefault();
		e.dataTransfer!.dropEffect = 'move';
		dragOverIdx = idx;
	}

	function onDrop(idx: number, e: DragEvent) {
		e.preventDefault();
		if (dragSrcIdx === null || dragSrcIdx === idx) {
			dragSrcIdx = null;
			dragOverIdx = null;
			return;
		}
		const next = [...files];
		const [moved] = next.splice(dragSrcIdx, 1);
		const insertAt = idx > dragSrcIdx ? idx - 1 : idx;
		next.splice(insertAt, 0, moved);
		files = next;
		dragSrcIdx = null;
		dragOverIdx = null;
		void saveReorder();
	}

	function onDragEnd() {
		dragSrcIdx = null;
		dragOverIdx = null;
	}

	async function saveReorder() {
		if (reorderPending) return;
		reorderPending = true;
		try {
			await api.put(`/pools/${poolId}/files/reorder`, {
				file_ids: files.map((f) => f.id)
			});
		} catch {
			// non-critical — positions may be out of sync
		} finally {
			reorderPending = false;
		}
	}

	// ---- Add files mode ----
	async function openAddMode() {
		addMode = true;
		addFiles = [];
		addNextCursor = null;
		addHasMore = true;
		addSelected = new Set();
		addSearch = '';
		addSearchPrev = '';
		await tick();
		void loadAddFiles();
	}

	function closeAddMode() {
		addMode = false;
	}

	$effect(() => {
		const s = addSearch;
		if (!addMode) return;
		if (s === addSearchPrev) return;
		addSearchPrev = s;
		addFiles = [];
		addNextCursor = null;
		addHasMore = true;
		void loadAddFiles();
	});

	async function loadAddFiles() {
		if (addLoading || !addHasMore) return;
		addLoading = true;
		try {
			const params = new URLSearchParams({ limit: String(LIMIT), sort: 'created', order: 'desc' });
			if (addNextCursor) params.set('cursor', addNextCursor);
			if (addSearch.trim()) params.set('search', addSearch.trim());
			const res = await api.get<FileCursorPage>(`/files?${params}`);
			addFiles = [...addFiles, ...(res.items ?? [])];
			addNextCursor = res.next_cursor ?? null;
			addHasMore = !!res.next_cursor;
		} catch {
			addHasMore = false;
		} finally {
			addLoading = false;
		}
	}

	function toggleAddSelect(id: string) {
		if (addSelected.has(id)) {
			addSelected.delete(id);
		} else {
			addSelected.add(id);
		}
		addSelected = new Set(addSelected);
	}

	async function confirmAddFiles() {
		if (addSelected.size === 0) return;
		const ids = [...addSelected];
		try {
			await api.post(`/pools/${poolId}/files`, { file_ids: ids });
			if (pool) pool = { ...pool, file_count: (pool.file_count ?? 0) + ids.length };
			// Reload the pool files from scratch
			files = [];
			nextCursor = null;
			hasMore = true;
			closeAddMode();
		} catch {
			// ignore
		}
	}

	// Already-in-pool set for add mode
	let inPoolIds = $derived(new Set(files.map((f) => f.id ?? '')));
</script>

<svelte:head>
	<title>{pool?.name ?? 'Pool'} | Tanabata</title>
</svelte:head>

<div class="page">
	<!-- ====== ADD FILES OVERLAY ====== -->
	{#if addMode}
		<div class="add-overlay">
			<header class="top-bar">
				<button class="back-btn" onclick={closeAddMode} aria-label="Close">
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
				<span class="header-title">Add files to "{pool?.name ?? ''}"</span>
			</header>

			<div class="add-search-bar">
				<div class="search-wrap">
					<input
						class="search-input"
						type="search"
						placeholder="Search by name…"
						value={addSearch}
						oninput={(e) => (addSearch = (e.currentTarget as HTMLInputElement).value)}
						autocomplete="off"
					/>
					{#if addSearch}
						<button class="search-clear" onclick={() => (addSearch = '')} aria-label="Clear">
							<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
								<path
									d="M2 2l10 10M12 2L2 12"
									stroke="currentColor"
									stroke-width="1.8"
									stroke-linecap="round"
								/>
							</svg>
						</button>
					{/if}
				</div>
			</div>

			<main class="add-main">
				<div class="grid">
					{#each addFiles as file, i (file.id)}
						<div class="add-card-wrap" class:already-in={inPoolIds.has(file.id ?? '')}>
							<FileCard
								{file}
								index={i}
								selected={addSelected.has(file.id ?? '')}
								selectionMode
								onTap={() => file.id && toggleAddSelect(file.id)}
							/>
							{#if inPoolIds.has(file.id ?? '')}
								<div class="in-pool-badge" aria-label="Already in pool">✓</div>
							{/if}
						</div>
					{/each}
				</div>

				<InfiniteScroll loading={addLoading} hasMore={addHasMore} onLoadMore={loadAddFiles} />

				{#if !addLoading && addFiles.length === 0}
					<div class="empty">No files found.</div>
				{/if}
			</main>

			{#if addSelected.size > 0}
				<div class="add-action-bar">
					<button class="add-confirm-btn" onclick={confirmAddFiles}>
						Add {addSelected.size} file{addSelected.size !== 1 ? 's' : ''}
					</button>
				</div>
			{/if}
		</div>

		<!-- ====== NORMAL POOL VIEW ====== -->
	{:else}
		<header class="top-bar">
			<button class="back-btn" onclick={() => goto('/pools')} aria-label="Back">
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
			<span class="header-title">{pool?.name ?? 'Pool'}</span>
			<div class="header-actions">
				<button class="icon-text-btn" onclick={() => (editOpen = !editOpen)}>
					<svg width="15" height="15" viewBox="0 0 15 15" fill="none" aria-hidden="true">
						<path
							d="M10.5 2.5l2 2-8 8H2.5v-2l8-8z"
							stroke="currentColor"
							stroke-width="1.5"
							stroke-linejoin="round"
						/>
					</svg>
					Edit
				</button>
				<button class="icon-text-btn add-btn" onclick={openAddMode}>
					<svg width="15" height="15" viewBox="0 0 15 15" fill="none" aria-hidden="true">
						<path
							d="M7.5 2v11M2 7.5h11"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
						/>
					</svg>
					Add
				</button>
			</div>
		</header>

		{#if loadError}
			<p class="load-error" role="alert">{loadError}</p>
		{:else if !loaded}
			<div class="loading-row">
				<span class="spinner" role="status" aria-label="Loading"></span>
			</div>
		{:else}
			<!-- Edit form -->
			{#if editOpen}
				<div class="edit-section">
					{#if saveError}
						<p class="error" role="alert">{saveError}</p>
					{/if}
					<div class="field">
						<label class="label" for="name">Name <span class="required">*</span></label>
						<input
							id="name"
							class="input"
							type="text"
							bind:value={name}
							required
							placeholder="Pool name"
							autocomplete="off"
						/>
					</div>
					<div class="field">
						<label class="label" for="notes">Notes</label>
						<textarea
							id="notes"
							class="textarea"
							rows="2"
							bind:value={notes}
							placeholder="Optional notes…"
						></textarea>
					</div>
					<div class="toggle-row">
						<span class="label">Public</span>
						<button
							type="button"
							class="toggle"
							class:on={isPublic}
							onclick={() => (isPublic = !isPublic)}
							role="switch"
							aria-checked={isPublic}
							aria-label="Public"
						>
							<span class="thumb"></span>
						</button>
					</div>
					<div class="action-row">
						<button class="submit-btn" onclick={save} disabled={!name.trim() || saving}>
							{saving ? 'Saving…' : 'Save changes'}
						</button>
						<button class="delete-btn" onclick={() => (confirmDelete = true)} disabled={deleting}>
							{deleting ? 'Deleting…' : 'Delete'}
						</button>
					</div>
				</div>
			{/if}

			<!-- Files section header -->
			<div class="files-header">
				<span class="files-title">
					Files
					{#if pool?.file_count != null}<span class="count">({pool.file_count})</span>{/if}
				</span>
				<div class="files-header-actions">
					{#if canReorder && files.length > 1}
						<span class="reorder-hint" title="Drag thumbnails to reorder">
							<svg width="13" height="13" viewBox="0 0 13 13" fill="none" aria-hidden="true">
								<circle cx="4" cy="3" r="1" fill="currentColor" />
								<circle cx="9" cy="3" r="1" fill="currentColor" />
								<circle cx="4" cy="6.5" r="1" fill="currentColor" />
								<circle cx="9" cy="6.5" r="1" fill="currentColor" />
								<circle cx="4" cy="10" r="1" fill="currentColor" />
								<circle cx="9" cy="10" r="1" fill="currentColor" />
							</svg>
							reorder
						</span>
					{/if}
					<button
						class="filter-btn"
						class:active={activeTokens.length > 0 || filterOpen}
						onclick={() => (filterOpen = !filterOpen)}
						title="Filter"
					>
						<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
							<path
								d="M1 3h12M3 7h8M5 11h4"
								stroke="currentColor"
								stroke-width="1.8"
								stroke-linecap="round"
							/>
						</svg>
						Filter
					</button>
				</div>
			</div>

			{#if filterOpen}
				<FilterBar value={filterParam} onApply={applyFilter} onClose={() => (filterOpen = false)} />
			{/if}

			<!-- File grid -->
			<main>
				{#if filesError}
					<p class="error" role="alert">{filesError}</p>
				{/if}

				<div class="grid">
					{#each files as file, i (file.id)}
						<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
						<div
							class="card-wrap"
							role="listitem"
							class:drag-over={dragOverIdx === i && dragSrcIdx !== null && dragSrcIdx !== i}
							data-idx={i}
							draggable={canReorder}
							ondragstart={canReorder ? (e) => onDragStart(i, e) : undefined}
							ondragover={canReorder ? (e) => onDragOver(i, e) : undefined}
							ondrop={canReorder ? (e) => onDrop(i, e) : undefined}
							ondragend={canReorder ? onDragEnd : undefined}
						>
							<FileCard
								{file}
								index={i}
								selected={selectedIds.has(file.id ?? '')}
								{selectionMode}
								onTap={(e) => handleTap(file, i, e)}
								onLongPress={() => handleLongPress(file, i)}
							/>
						</div>
					{/each}
				</div>

				<InfiniteScroll loading={filesLoading} {hasMore} onLoadMore={loadMore} />

				{#if !filesLoading && !hasMore && files.length === 0}
					<div class="empty">
						{filterParam ? 'No files match the filter.' : 'No files in this pool yet.'}
					</div>
				{/if}
			</main>
		{/if}
	{/if}
</div>

<!-- File viewer overlay (shallow routing): renders on top of the still-mounted
     pool grid, so closing it reveals the pool untouched. -->
{#if activeFileId}
	<div class="viewer-overlay">
		<FileViewer
			fileId={activeFileId}
			prevId={viewerPrevId}
			nextId={viewerNextId}
			onNavigate={pageTo}
			onClose={closeViewer}
		/>
	</div>
{/if}

<!-- Selection bar (remove mode) -->
{#if selectionMode && !addMode}
	<div class="selection-bar" role="toolbar">
		<button
			class="sel-cancel"
			onclick={() => {
				selectedIds = new Set();
				lastSelectedIdx = null;
			}}
		>
			<span class="sel-num">{selectedIds.size}</span>
			<span class="sel-label">selected</span>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
				<path
					d="M2 2l10 10M12 2L2 12"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
				/>
			</svg>
		</button>
		<div class="sel-spacer"></div>
		<button class="sel-action remove-action" onclick={() => (confirmRemove = true)}>
			Remove from pool
		</button>
	</div>
{/if}

{#if confirmDelete}
	<ConfirmDialog
		message={`Delete pool "${name}"? The files themselves will not be deleted.`}
		confirmLabel="Delete pool"
		danger
		onConfirm={doDeletePool}
		onCancel={() => (confirmDelete = false)}
	/>
{/if}

{#if confirmRemove}
	<ConfirmDialog
		message={`Remove ${selectedIds.size} file${selectedIds.size !== 1 ? 's' : ''} from this pool?`}
		confirmLabel="Remove"
		danger
		onConfirm={removeSelected}
		onCancel={() => (confirmRemove = false)}
	/>
{/if}

<style>
	.page {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}

	/* Full-screen viewer overlay covering the grid and the bottom navbar. */
	.viewer-overlay {
		position: fixed;
		inset: 0;
		z-index: 200;
		background-color: var(--color-bg-primary);
		overflow-y: auto;
		overscroll-behavior: contain;
	}

	/* ---- Shared top bar ---- */
	.top-bar {
		position: sticky;
		top: 0;
		z-index: 20;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		min-height: 44px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		flex-shrink: 0;
	}

	.back-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border-radius: 8px;
		border: none;
		background: none;
		color: var(--color-text-primary);
		cursor: pointer;
		flex-shrink: 0;
	}
	.back-btn:hover {
		background-color: color-mix(in srgb, var(--color-accent) 15%, transparent);
	}

	.header-title {
		flex: 1;
		font-size: 0.95rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.header-actions {
		display: flex;
		gap: 4px;
		flex-shrink: 0;
	}

	.icon-text-btn {
		display: flex;
		align-items: center;
		gap: 4px;
		height: 28px;
		padding: 0 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		font-size: 0.8rem;
		font-family: inherit;
		cursor: pointer;
	}
	.icon-text-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-accent);
	}
	.icon-text-btn.add-btn {
		color: var(--color-accent);
		border-color: color-mix(in srgb, var(--color-accent) 50%, transparent);
	}
	.icon-text-btn.add-btn:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, var(--color-bg-elevated));
	}

	/* ---- Edit section ---- */
	.edit-section {
		padding: 14px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		display: flex;
		flex-direction: column;
		gap: 12px;
		background-color: var(--color-bg-elevated);
		flex-shrink: 0;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 5px;
	}

	.label {
		font-size: 0.72rem;
		font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}
	.required {
		color: var(--color-danger);
	}

	.input {
		width: 100%;
		box-sizing: border-box;
		height: 34px;
		padding: 0 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		outline: none;
	}
	.input:focus {
		border-color: var(--color-accent);
	}

	.textarea {
		width: 100%;
		box-sizing: border-box;
		padding: 7px 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		resize: vertical;
		outline: none;
		min-height: 60px;
	}
	.textarea:focus {
		border-color: var(--color-accent);
	}

	.toggle-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}
	.toggle-row .label {
		margin: 0;
	}

	.toggle {
		position: relative;
		width: 44px;
		height: 26px;
		border-radius: 13px;
		border: none;
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-primary));
		cursor: pointer;
		transition: background-color 0.2s;
		flex-shrink: 0;
	}
	.toggle.on {
		background-color: var(--color-accent);
	}
	.thumb {
		position: absolute;
		top: 3px;
		left: 3px;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		background-color: #fff;
		transition: transform 0.2s;
	}
	.toggle.on .thumb {
		transform: translateX(18px);
	}

	.action-row {
		display: flex;
		gap: 8px;
	}

	.submit-btn {
		flex: 1;
		height: 38px;
		border-radius: 8px;
		border: none;
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-size: 0.875rem;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
	}
	.submit-btn:hover:not(:disabled) {
		background-color: var(--color-accent-hover);
	}
	.submit-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}

	.delete-btn {
		height: 38px;
		padding: 0 14px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--color-danger) 50%, transparent);
		background: none;
		color: var(--color-danger);
		font-size: 0.875rem;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
	}
	.delete-btn:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--color-danger) 12%, transparent);
	}
	.delete-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}

	/* ---- Files header ---- */
	.files-header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 10%, transparent);
		flex-shrink: 0;
	}

	.files-title {
		font-size: 0.78rem;
		font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		flex: 1;
	}

	.count {
		font-weight: 400;
	}

	.files-header-actions {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.reorder-hint {
		display: flex;
		align-items: center;
		gap: 3px;
		font-size: 0.72rem;
		color: var(--color-text-muted);
		opacity: 0.6;
	}

	.filter-btn {
		display: flex;
		align-items: center;
		gap: 4px;
		height: 26px;
		padding: 0 8px;
		border-radius: 5px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background: none;
		color: var(--color-text-muted);
		font-size: 0.78rem;
		font-family: inherit;
		cursor: pointer;
	}
	.filter-btn:hover,
	.filter-btn.active {
		color: var(--color-accent);
		border-color: var(--color-accent);
	}

	/* ---- File grid ---- */
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

	.card-wrap {
		position: relative;
		cursor: grab;
		outline: 2px solid transparent;
		transition: outline-color 0.1s;
	}

	.card-wrap[draggable='false'] {
		cursor: default;
	}

	.card-wrap.drag-over {
		outline: 2px solid var(--color-accent);
		outline-offset: -2px;
	}

	/* ---- Loading / empty ---- */
	.loading-row {
		display: flex;
		justify-content: center;
		padding: 40px;
	}

	.spinner {
		display: block;
		width: 28px;
		height: 28px;
		border: 3px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-top-color: var(--color-accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}
	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.load-error,
	.error {
		color: var(--color-danger);
		font-size: 0.875rem;
		padding: 12px;
		margin: 0;
	}

	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 60px 20px;
		font-size: 0.9rem;
	}

	/* ---- Selection bar ---- */
	.selection-bar {
		position: fixed;
		left: 10px;
		right: 10px;
		bottom: 65px;
		background-color: var(--color-bg-secondary);
		border-radius: 10px;
		box-shadow: 0 0 12px rgba(0, 0, 0, 0.5);
		padding: 10px 14px;
		z-index: 100;
		display: flex;
		align-items: center;
		gap: 4px;
		animation: slide-up 0.18s ease-out;
	}

	@keyframes slide-up {
		from {
			transform: translateY(12px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}

	.sel-cancel {
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
	.sel-cancel:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
		color: var(--color-text-primary);
	}

	.sel-num {
		font-size: 1.1rem;
		font-weight: 700;
		color: var(--color-text-primary);
	}

	.sel-label {
		font-size: 0.85rem;
	}

	.sel-spacer {
		flex: 1;
	}

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

	.remove-action {
		color: var(--color-danger);
	}
	.remove-action:hover {
		background-color: color-mix(in srgb, var(--color-danger) 15%, transparent);
	}

	/* ---- Add files overlay ---- */
	.add-overlay {
		position: absolute;
		inset: 0;
		background-color: var(--color-bg-primary);
		display: flex;
		flex-direction: column;
		z-index: 50;
	}

	.add-search-bar {
		padding: 8px 12px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 10%, transparent);
		flex-shrink: 0;
	}

	.search-wrap {
		position: relative;
		display: flex;
		align-items: center;
	}

	.search-input {
		width: 100%;
		box-sizing: border-box;
		height: 34px;
		padding: 0 12px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		outline: none;
	}
	.search-input:focus {
		border-color: var(--color-accent);
	}

	.search-clear {
		position: absolute;
		right: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
		width: 20px;
		height: 20px;
		border-radius: 50%;
		border: none;
		background: none;
		color: var(--color-text-muted);
		cursor: pointer;
		padding: 0;
	}
	.search-clear:hover {
		color: var(--color-text-primary);
	}

	.add-main {
		flex: 1;
		overflow-y: auto;
		padding: 10px 10px calc(80px + 10px);
	}

	.add-card-wrap {
		position: relative;
		cursor: pointer;
	}

	.already-in {
		opacity: 0.45;
	}

	.in-pool-badge {
		position: absolute;
		bottom: 4px;
		left: 4px;
		background-color: rgba(0, 0, 0, 0.65);
		color: #fff;
		font-size: 0.7rem;
		font-weight: 700;
		padding: 1px 5px;
		border-radius: 4px;
		pointer-events: none;
	}

	.add-action-bar {
		position: fixed;
		bottom: 65px;
		left: 10px;
		right: 10px;
		z-index: 60;
		animation: slide-up 0.15s ease-out;
	}

	.add-confirm-btn {
		width: 100%;
		height: 46px;
		border-radius: 10px;
		border: none;
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-size: 1rem;
		font-weight: 700;
		font-family: inherit;
		cursor: pointer;
		box-shadow: 0 4px 16px rgba(0, 0, 0, 0.4);
	}
	.add-confirm-btn:hover {
		background-color: var(--color-accent-hover);
	}
</style>
