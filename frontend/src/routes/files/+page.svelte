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
	import { parseDslFilter } from '$lib/utils/dsl';
	import type { File, FileCursorPage } from '$lib/api/types';

	let uploader = $state<{ open: () => void } | undefined>();
	let confirmDeleteFiles = $state(false);

	function handleUploaded(file: File) {
		files = [file, ...files];
	}

	const LIMIT = 50;

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
		onEditTags={() => {/* TODO */}}
		onAddToPool={() => {/* TODO */}}
		onDelete={() => (confirmDeleteFiles = true)}
	/>
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
</style>
