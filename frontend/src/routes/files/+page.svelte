<script lang="ts">
	import { page } from '$app/state';
	import { afterNavigate, beforeNavigate, goto, pushState, replaceState } from '$app/navigation';
	import { saveSection, takeSection } from '$lib/stores/sectionCache';
	import { api } from '$lib/api/client';
	import { ApiError } from '$lib/api/client';
	import FileCard from '$lib/components/file/FileCard.svelte';
	import FileViewer from '$lib/components/file/FileViewer.svelte';
	import FileUpload from '$lib/components/file/FileUpload.svelte';
	import FilterBar from '$lib/components/file/FilterBar.svelte';
	import Header from '$lib/components/layout/Header.svelte';
	import SelectionBar from '$lib/components/layout/SelectionBar.svelte';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import { fileSorting, type FileSortField } from '$lib/stores/sorting';
	import { selectionStore, selectionActive } from '$lib/stores/selection';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import BulkTagEditor from '$lib/components/file/BulkTagEditor.svelte';
	import PoolPicker from '$lib/components/file/PoolPicker.svelte';
	import { tick, flushSync } from 'svelte';
	import { parseDslFilter } from '$lib/utils/dsl';
	import type { File, FileCursorPage } from '$lib/api/types';
	import { appSettings } from '$lib/stores/appSettings';

	// What the section cache stores for the Files grid. `resetKey` guards against
	// restoring under a different sort/filter than was captured.
	interface FilesSnapshot {
		resetKey: string;
		files: File[];
		nextCursor: string | null;
		hasMore: boolean;
		prevCursor: string | null;
		hasPrev: boolean;
	}

	let scrollContainer = $state<HTMLElement | undefined>();

	let uploader = $state<{ open: () => void } | undefined>();
	let confirmDeleteFiles = $state(false);

	// ---- Bulk tag editor ----
	let tagEditorOpen = $state(false);

	// ---- Keyboard roving focus ----
	// The id of the grid's keyboard-focused file, plus a flag that gates the focus
	// ring so it only shows once the user actually starts navigating by keyboard.
	let focusedId = $state<string | null>(null);
	let kbActive = $state(false);

	function isFormTarget(t: EventTarget | null): boolean {
		return (
			t instanceof HTMLElement &&
			(t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON', 'A'].includes(t.tagName))
		);
	}

	function gridCols(): number {
		const w = scrollContainer?.clientWidth ?? 0;
		return Math.max(1, Math.floor((w || 360) / CARD_PITCH));
	}

	function focusedFile(): File | undefined {
		return focusedId ? files.find((f) => f.id === focusedId) : undefined;
	}

	// Move the roving focus by `delta` positions, clamped to the loaded grid, and
	// scroll the new card into view. Pulls the next page when nearing the end.
	function moveFocus(delta: number) {
		if (files.length === 0) return;
		kbActive = true;
		const cur = focusedId ? files.findIndex((f) => f.id === focusedId) : -1;
		const next = Math.max(0, Math.min(files.length - 1, cur < 0 ? 0 : cur + delta));
		focusedId = files[next]?.id ?? null;
		if (next >= files.length - gridCols() * 2 && hasMore && !loading) void loadMore();
		const id = focusedId;
		// scrollIntoView scrolls whichever element actually scrolls (the window
		// here, not <main>), so the grid follows the focus. The card's
		// scroll-margin-bottom leaves room for the fixed navbar so it doesn't slide
		// underneath.
		requestAnimationFrame(() => {
			const idx = files.findIndex((f) => f.id === id);
			scrollContainer
				?.querySelector<HTMLElement>(`[data-file-index="${idx}"]`)
				?.scrollIntoView({ block: 'nearest' });
		});
	}

	// Action keys operate on the selection; with nothing selected they fall back to
	// the focused card (selecting it first so the bulk sheets have a target).
	function ensureSelectedFocused() {
		const f = focusedFile();
		if (f?.id && !$selectionStore.ids.has(f.id)) selectionStore.select(f.id);
	}

	// Select via the keyboard: a plain press toggles the focused card and drops the
	// range anchor there; a Shift press selects everything from the anchor to the
	// focused card — the same model as Shift+click on the grid.
	// Select an inclusive index range in gesture direction (anchor → target) so the
	// selection's insertion order follows how the user swept, not grid order. The
	// Set preserves insertion order, so this is what later carries through to e.g.
	// the order files land in a pool.
	function selectRange(anchorIdx: number, targetIdx: number) {
		const step = targetIdx >= anchorIdx ? 1 : -1;
		for (let i = anchorIdx; i !== targetIdx + step; i += step) {
			if (files[i]?.id) selectionStore.select(files[i].id!);
		}
	}

	function selectFocused(range: boolean) {
		const idx = focusedId ? files.findIndex((f) => f.id === focusedId) : -1;
		if (idx < 0) return;
		if (range && lastSelectedIdx !== null) {
			selectRange(lastSelectedIdx, idx);
		} else if (files[idx]?.id) {
			selectionStore.toggle(files[idx].id!);
		}
		lastSelectedIdx = idx;
	}

	function openTagEditor() {
		tagEditorOpen = true;
		void tick().then(() => document.querySelector<HTMLInputElement>('.tag-sheet input')?.focus());
	}

	// Mark the current selection as review-done (tagging finished). Best-effort
	// optimistic update of the local list so the "needs review" badges clear.
	async function markSelectionReviewed() {
		const ids = [...$selectionStore.ids];
		if (ids.length === 0) return;
		selectionStore.exit();
		try {
			await api.post('/files/bulk/review', { file_ids: ids, needs_review: false });
			files = files.map((f) =>
				ids.includes(f.id ?? '') ? { ...f, needs_review: false } : f
			);
		} catch {
			// ignore — list already reflects the intended state optimistically
		}
	}

	function openFilterAndFocus() {
		filterOpen = true;
		void tick().then(() => document.querySelector<HTMLInputElement>('.bar .search')?.focus());
	}

	// Single window handler for the grid: Escape peels one layer at a time (overlay
	// → selection; the viewer owns its own Escape), and the rest drives roving
	// focus + bulk actions while the bare list is in front.
	function handleKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (tagEditorOpen) tagEditorOpen = false;
			else if (poolPickerOpen) return; // PoolPicker owns Escape (clear search, then close)
			else if (confirmDeleteFiles) confirmDeleteFiles = false;
			else if (activeFileId) return;
			else if ($selectionActive) selectionStore.exit();
			return;
		}

		if (activeFileId || tagEditorOpen || poolPickerOpen || confirmDeleteFiles) return;
		if (isFormTarget(e.target) || e.metaKey || e.ctrlKey || e.altKey) return;

		// Navigation / named keys — same on every layout.
		switch (e.key) {
			case 'ArrowRight':
				e.preventDefault();
				moveFocus(1);
				return;
			case 'ArrowLeft':
				e.preventDefault();
				moveFocus(-1);
				return;
			case 'ArrowDown':
				e.preventDefault();
				moveFocus(gridCols());
				return;
			case 'ArrowUp':
				e.preventDefault();
				moveFocus(-gridCols());
				return;
			case 'Enter': {
				const f = focusedFile();
				if (f) {
					e.preventDefault();
					openFile(f);
				}
				return;
			}
			case ' ':
				e.preventDefault();
				selectFocused(e.shiftKey);
				return;
			case 'Delete':
				if ($selectionActive || focusedFile()) {
					e.preventDefault();
					ensureSelectedFocused();
					confirmDeleteFiles = true;
				}
				return;
		}

		// Select by position (x), Shift = range — handled before the unshifted-only
		// guard below because Shift+x is a valid range-select.
		if (e.code === 'KeyX') {
			e.preventDefault();
			selectFocused(e.shiftKey);
			return;
		}

		// The remaining letter / symbol commands are unshifted-only, matched by
		// physical position so they fire the same on a non-Latin layout.
		if (e.shiftKey) return;
		switch (e.code) {
			case 'KeyE':
				if ($selectionActive || focusedFile()) {
					e.preventDefault();
					ensureSelectedFocused();
					openTagEditor();
				}
				break;
			case 'KeyP':
				if ($selectionActive || focusedFile()) {
					e.preventDefault();
					ensureSelectedFocused();
					void openPoolPicker();
				}
				break;
			case 'Slash':
				e.preventDefault();
				openFilterAndFocus();
				break;
		}
	}

	// ---- Add to pool picker ----
	// The picker itself (load, search, add) lives in PoolPicker; here we just
	// gate it open and clear the selection once files land in a pool.
	let poolPickerOpen = $state(false);

	function openPoolPicker() {
		poolPickerOpen = true;
	}

	function handleUploaded(file: File) {
		files = [file, ...files];
	}

	let LIMIT = $derived($appSettings.fileLoadLimit);

	const FILE_SORT_OPTIONS = [
		{ value: 'created', label: 'Created' },
		{ value: 'content_datetime', label: 'Date taken' },
		{ value: 'original_name', label: 'Name' },
		{ value: 'mime', label: 'Type' }
	];

	let files = $state<File[]>([]);
	let nextCursor = $state<string | null>(null);
	// Start busy when arriving with an ?anchor so the InfiniteScroll sentinels
	// can't fire a stray page-1 loadMore before loadAroundAnchor takes over (their
	// effects run before this component's reset effect on mount).
	let loading = $state(Boolean(page.url.searchParams.get('anchor')));
	let hasMore = $state(true);
	// Backward pagination — only active after an anchored return, where the grid
	// starts in the middle of the list and can grow upward as well as downward.
	let prevCursor = $state<string | null>(null);
	let hasPrev = $state(false);
	let error = $state('');
	let filterOpen = $state(false);

	let filterParam = $derived(page.url.searchParams.get('filter'));
	let anchorParam = $derived(page.url.searchParams.get('anchor'));
	let activeTokens = $derived(parseDslFilter(filterParam));
	let sortState = $derived($fileSorting);

	let resetKey = $derived(`${sortState.sort}|${sortState.order}|${filterParam ?? ''}`);
	let prevKey = $state('');

	// Scroll offset to reapply once the restored grid has painted (set when a
	// cached snapshot is rehydrated; consumed in afterNavigate so it wins over
	// SvelteKit's own scroll-to-top).
	let pendingScroll: number | null = null;

	// Reset + reload when the query (sort/order/filter) changes or on first mount.
	// The viewer opens as an overlay now (the list is never unmounted), so there's
	// no snapshot to restore — except a deep-link return carrying an anchor.
	$effect(() => {
		const key = resetKey;
		if (key === prevKey) return;
		const firstRun = prevKey === '';
		prevKey = key;

		// Returning to this section: rehydrate the loaded grid + cursors + scroll
		// from the section cache instead of refetching, as long as the snapshot was
		// taken under the same sort/filter. Skip when arriving on an anchor, which
		// has its own (deep-link) restore path below.
		if (firstRun && !anchorParam) {
			const snap = takeSection<FilesSnapshot>('files');
			if (snap && snap.data.resetKey === key && snap.data.files.length > 0) {
				files = snap.data.files;
				nextCursor = snap.data.nextCursor;
				hasMore = snap.data.hasMore;
				prevCursor = snap.data.prevCursor;
				hasPrev = snap.data.hasPrev;
				// Hold the load guards shut until the scroll is reapplied, so the
				// InfiniteScroll sentinels can't fire a stray page load at the top.
				loading = true;
				pendingScroll = snap.scrollTop;
				return;
			}
		}

		files = [];
		nextCursor = null;
		hasMore = true;
		// A plain list starts at the top, so there is nothing before it.
		prevCursor = null;
		hasPrev = false;
		error = '';
		// Deep-link return carrying a position anchor but no loaded grid: load a
		// window centred on the anchor instead of page 1, so we can scroll to it
		// and grow the grid in both directions. Otherwise (first mount, or a sort/
		// filter change) load page 1 right here — the list isn't remounted on a
		// query change, so InfiniteScroll won't re-trigger on its own.
		if (firstRun && anchorParam) {
			void loadAroundAnchor(anchorParam);
		} else {
			void loadMore();
		}
	});

	// Scroll to an ?anchor= file on a deep-link return. Runs in afterNavigate
	// because it fires AFTER SvelteKit's own scroll handling, so our position wins
	// instead of being reset to the top.
	afterNavigate(() => {
		const anchor = page.url.searchParams.get('anchor');
		if (anchor) {
			scrollToFile(anchor);
			// Pre-focus the anchor file so keyboard navigation resumes from it.
			focusedId = anchor;
			kbActive = true;
			consumeAnchor();
			return;
		}
		// Reapply a cached scroll position after a section-cache rehydrate.
		if (pendingScroll != null) {
			restoreScrollTop(pendingScroll);
			pendingScroll = null;
		}
	});

	// Snapshot the loaded grid, cursors and scroll position on the way out, so
	// returning to this section restores them instead of refetching. Skipped for
	// the shallow-routed viewer (pushState doesn't trigger a navigation) — only
	// real departures to another route reach here.
	beforeNavigate((nav) => {
		// Staying on the list (a sort/filter query change via goto) isn't a
		// departure — nothing to snapshot.
		if (nav.to?.url.pathname === '/files') return;
		if (files.length === 0) return;
		const scroller = getScroller();
		saveSection<FilesSnapshot>('files', scroller.scrollTop, {
			resetKey,
			files,
			nextCursor,
			hasMore,
			prevCursor,
			hasPrev
		});
	});

	// Reapply a restored scroll offset, retrying across frames because the grid
	// may not be laid out yet right after rehydrate. Releases the load guard once
	// applied so InfiniteScroll can resume.
	function restoreScrollTop(top: number) {
		let tries = 10;
		const apply = () => {
			const scroller = getScroller();
			if (scroller.scrollHeight > top + scroller.clientHeight || tries-- <= 0) {
				scroller.scrollTop = top;
				loading = false;
				return;
			}
			requestAnimationFrame(apply);
		};
		requestAnimationFrame(apply);
	}

	// Scroll the grid so the given file is centred. Uses scrollIntoView (works
	// whether the actual scroller is <main> or the window) and retries across
	// frames because the cards may not be laid out yet right after a restore.
	function scrollToFile(anchorId: string | null) {
		if (!anchorId) return;
		const tryScroll = () => {
			const idx = files.findIndex((f) => f.id === anchorId);
			const card =
				idx >= 0 && scrollContainer
					? scrollContainer.querySelector<HTMLElement>(`[data-file-index="${idx}"]`)
					: null;
			if (card) {
				card.scrollIntoView({ block: 'center' });
				return true;
			}
			return false;
		};
		// Centre immediately if the card is already laid out (it is, right after the
		// anchored load's tick) so it's pinned before any scroll sentinel fires.
		if (tryScroll()) return;
		let tries = 10;
		const loop = () => {
			if (tryScroll() || tries-- <= 0) return;
			requestAnimationFrame(loop);
		};
		requestAnimationFrame(loop);
	}

	// Drop the ?anchor= param once consumed so it doesn't linger in the URL or
	// re-fire on later interactions. Shallow update — no navigation, no scroll.
	function consumeAnchor() {
		const url = new URL(page.url);
		if (!url.searchParams.has('anchor')) return;
		url.searchParams.delete('anchor');
		replaceState(`${url.pathname}${url.search}`, page.state);
	}

	// How many pages to pre-load on each side of the anchor so the viewport is
	// covered and the scroll sentinels start out of range (no mount-time storm).
	const ANCHOR_PREFILL_PAGES = 3;

	function baseListParams(): URLSearchParams {
		const p = new URLSearchParams({
			limit: String(LIMIT),
			sort: sortState.sort,
			order: sortState.order
		});
		if (filterParam) p.set('filter', filterParam);
		return p;
	}

	// Deep link / hard reload with an anchor but no loaded grid: fetch a window
	// centred on that file and pre-fill a few pages each way, all sequentially, so
	// the grid is filled around the anchor before we centre on it. The prev/next
	// cursors then let it keep growing in both directions as the user scrolls.
	async function loadAroundAnchor(anchor: string) {
		loading = true;
		error = '';
		try {
			const a = baseListParams();
			a.set('anchor', anchor);
			const res = await api.get<FileCursorPage>(`/files?${a}`);
			files = res.items ?? [];
			nextCursor = res.next_cursor ?? null;
			hasMore = !!res.next_cursor;
			prevCursor = res.prev_cursor ?? null;
			hasPrev = !!res.prev_cursor;

			for (let i = 0; i < ANCHOR_PREFILL_PAGES && hasMore && nextCursor; i++) {
				const p = baseListParams();
				p.set('cursor', nextCursor);
				const r = await api.get<FileCursorPage>(`/files?${p}`);
				files = [...files, ...(r.items ?? [])];
				nextCursor = r.next_cursor ?? null;
				hasMore = !!r.next_cursor;
			}
			for (let i = 0; i < ANCHOR_PREFILL_PAGES && hasPrev && prevCursor; i++) {
				const p = baseListParams();
				p.set('cursor', prevCursor);
				p.set('direction', 'backward');
				const r = await api.get<FileCursorPage>(`/files?${p}`);
				const items = r.items ?? [];
				if (items.length === 0) {
					hasPrev = false;
					break;
				}
				files = [...items, ...files];
				prevCursor = r.prev_cursor ?? null;
				hasPrev = !!r.prev_cursor;
			}

			await tick();
			scrollToFile(anchor);
			consumeAnchor();
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load files';
		} finally {
			loading = false;
		}
	}

	// Load the previous page (scrolling up) and prepend it. Content inserted above
	// the viewport would push everything down, so we shift the scroll down by
	// exactly the added height — applied synchronously (flushSync, no paint in
	// between) so there's no visible jump. Shares the `loading` guard with loadMore
	// so the two never mutate files concurrently.
	async function loadPrev() {
		if (loading || !hasPrev || !prevCursor) return;
		loading = true;
		try {
			const params = baseListParams();
			params.set('cursor', prevCursor);
			params.set('direction', 'backward');
			const res = await api.get<FileCursorPage>(`/files?${params}`);
			const items = res.items ?? [];
			if (items.length === 0) {
				hasPrev = false;
				return;
			}

			// Capture scroll state just before mutating (after the request, so the
			// user's scrolling during it doesn't skew the offset).
			const scroller = getScroller();
			const beforeTop = scroller.scrollTop;
			const beforeHeight = scroller.scrollHeight;

			files = [...items, ...files];
			prevCursor = res.prev_cursor ?? null;
			hasPrev = !!prevCursor;

			flushSync(); // apply the prepend now, before the browser paints
			scroller.scrollTop = beforeTop + (scroller.scrollHeight - beforeHeight);
		} catch {
			hasPrev = false;
		} finally {
			loading = false;
		}
	}

	// The element that actually scrolls the grid: the nearest scrollable ancestor,
	// or the document scroller (the grid's <main> doesn't scroll on its own here).
	function getScroller(): HTMLElement {
		let el: HTMLElement | null = scrollContainer ?? null;
		while (el) {
			const oy = getComputedStyle(el).overflowY;
			if ((oy === 'auto' || oy === 'scroll') && el.scrollHeight > el.clientHeight) {
				return el;
			}
			el = el.parentElement;
		}
		return (document.scrollingElement as HTMLElement | null) ?? document.documentElement;
	}

	// ---- Loaded grid -----------------------------------------------------------
	// The grid accumulates every loaded row for the lifetime of the visit. It is
	// only cleared on a sort/filter change or when leaving the page (the reset
	// effect above; the section cache restores it on return). Rows are never
	// trimmed while scrolling, so a long drag- or shift-select can span the whole
	// loaded list without earlier cards disappearing underneath it.

	const CARD_PITCH = 162; // 160px thumbnail + 2px grid gap (used for grid math)

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		error = '';
		try {
			const params = baseListParams();
			if (nextCursor) params.set('cursor', nextCursor);
			const res = await api.get<FileCursorPage>(`/files?${params}`);
			files = [...files, ...(res.items ?? [])];
			nextCursor = res.next_cursor ?? null;
			hasMore = !!nextCursor;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load files';
			hasMore = false;
		} finally {
			loading = false;
		}
		// Viewport filling is handled by InfiniteScroll, which re-checks after each
		// load — no manual recursion (which over-fetched here because <main> isn't
		// the scroller, so its scrollHeight never exceeds its clientHeight).
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
		if (!file.id) return;
		// Open the viewer as an overlay on top of the still-mounted grid via
		// shallow routing: the URL becomes /files/<id> and the browser back button
		// closes it, but the list is never torn down or reloaded.
		pushState(`/files/${file.id}`, { fileId: file.id });
	}

	// ---- Viewer overlay (shallow routing) ----
	let activeFileId = $derived(page.state.fileId);
	let activeIdx = $derived(activeFileId ? files.findIndex((f) => f.id === activeFileId) : -1);
	let viewerPrevId = $derived(activeIdx > 0 ? (files[activeIdx - 1]?.id ?? null) : null);
	let viewerNextId = $derived(
		activeIdx >= 0 && activeIdx < files.length - 1 ? (files[activeIdx + 1]?.id ?? null) : null
	);

	// Paging near the end of the loaded grid: pull the next page by cursor so the
	// viewer keeps advancing past what was loaded.
	$effect(() => {
		if (activeIdx >= 0 && activeIdx >= files.length - 3 && hasMore && !loading) {
			void loadMore();
		}
	});

	// When the overlay closes (back / Escape / close button), bring the grid to
	// the last-viewed file. The list was never unmounted, so this is instant.
	let lastOverlayId: string | null = null;
	$effect(() => {
		const id = activeFileId;
		if (id) {
			lastOverlayId = id;
		} else if (lastOverlayId) {
			const target = lastOverlayId;
			lastOverlayId = null;
			scrollToFile(target);
			// Land the keyboard roving-focus on the file we came back from, so arrow
			// navigation continues from there (and the ring marks where you were).
			focusedId = target;
			kbActive = true;
		}
	});

	function pageTo(id: string) {
		// Replace (not push) so a single back press returns to the grid rather than
		// stepping back through every file paged.
		replaceState(`/files/${id}`, { fileId: id });
	}

	function closeViewer() {
		history.back();
	}

	// ---- Selection logic ----

	let lastSelectedIdx = $state<number | null>(null);

	function handleTap(file: File, idx: number, e: MouseEvent) {
		if (!$selectionActive) {
			openFile(file);
			return;
		}
		if (e.shiftKey && lastSelectedIdx !== null) {
			// Range-select from the anchor toward idx (desktop), in gesture order.
			selectRange(lastSelectedIdx, idx);
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

<svelte:window onkeydown={handleKey} />

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
		onTrash={() => goto('/files/trash')}
		onDuplicates={() => goto('/files/duplicates')}
	/>

	{#if filterOpen}
		<FilterBar value={filterParam} onApply={applyFilter} onClose={() => (filterOpen = false)} />
	{/if}

	<FileUpload bind:this={uploader} onUploaded={handleUploaded}>
		<main bind:this={scrollContainer}>
			{#if error}
				<p class="error" role="alert">{error}</p>
			{/if}

			{#if hasPrev}
				<InfiniteScroll {loading} hasMore={hasPrev} onLoadMore={loadPrev} edge="top" />
			{/if}

			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div class="grid" onpointerdowncapture={() => (kbActive = false)}>
				{#each files as file, i (file.id)}
					<FileCard
						{file}
						index={i}
						selected={$selectionStore.ids.has(file.id ?? '')}
						selectionMode={$selectionActive}
						focused={kbActive && file.id === focusedId}
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

<!-- File viewer overlay (shallow routing): renders on top of the still-mounted
     grid, so closing it reveals the list untouched. -->
{#if activeFileId}
	<div class="viewer-overlay">
		<FileViewer
			fileId={activeFileId}
			prevId={viewerPrevId}
			nextId={viewerNextId}
			onNavigate={pageTo}
			onClose={closeViewer}
			onReviewChange={(id, nr) =>
				(files = files.map((f) => (f.id === id ? { ...f, needs_review: nr } : f)))}
		/>
	</div>
{/if}

{#if $selectionActive}
	<SelectionBar
		onEditTags={openTagEditor}
		onAddToPool={openPoolPicker}
		onMarkReviewed={markSelectionReviewed}
		onDelete={() => (confirmDeleteFiles = true)}
	/>
{/if}

{#if tagEditorOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<div class="picker-backdrop" role="presentation" onclick={() => (tagEditorOpen = false)}></div>
	<div class="picker-sheet tag-sheet" role="dialog" aria-label="Edit tags">
		<div class="picker-header">
			<span class="picker-title"
				>Edit tags — {$selectionStore.ids.size} file{$selectionStore.ids.size !== 1
					? 's'
					: ''}</span
			>
			<button class="picker-close" onclick={() => (tagEditorOpen = false)} aria-label="Close">
				<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
					<path
						d="M3 3l10 10M13 3L3 13"
						stroke="currentColor"
						stroke-width="1.8"
						stroke-linecap="round"
					/>
				</svg>
			</button>
		</div>
		<div class="tag-sheet-body">
			<BulkTagEditor fileIds={[...$selectionStore.ids]} onDone={() => (tagEditorOpen = false)} />
		</div>
	</div>
{/if}

{#if poolPickerOpen}
	<PoolPicker
		fileIds={[...$selectionStore.ids]}
		onAdded={() => selectionStore.exit()}
		onClose={() => (poolPickerOpen = false)}
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

	/* Full-screen overlay covering the grid and the bottom navbar (z 100). */
	.viewer-overlay {
		position: fixed;
		inset: 0;
		z-index: 200;
		background-color: var(--color-bg-primary);
		overflow-y: auto;
		overscroll-behavior: contain;
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

	/* ---- Bottom-sheet shell (shared by the tag editor sheet) ---- */
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
		from {
			transform: translateY(20px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
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
</style>
