import { api } from '$lib/api/client';
import type { File, FileCursorPage } from '$lib/api/types';

/** The sort/order/filter that identifies a particular files listing. */
export interface FilesQuery {
	sort: string;
	order: string;
	filter: string | null;
}

/**
 * A snapshot of the files grid, kept in memory so that opening a file and
 * returning restores the same list (and scroll position) instead of reloading
 * page 1 from the top. The file viewer also reads this to derive prev/next and
 * extends it as the user pages past the loaded set.
 */
export interface FilesSnapshot {
	query: FilesQuery;
	files: File[];
	nextCursor: string | null;
	hasMore: boolean;
	scrollTop: number;
	/** ID of the file the user opened — restore the grid centred on this. */
	lastOpenedId: string | null;
}

/** Stable string identity for a query, used to tell whether a snapshot still
 *  applies to the current sort/order/filter. */
export function queryKey(q: FilesQuery): string {
	return `${q.sort}|${q.order}|${q.filter ?? ''}`;
}

let snapshot: FilesSnapshot | null = null;
let loading = false;

/** Save (replace) the current grid snapshot. */
export function saveFilesSnapshot(s: FilesSnapshot): void {
	snapshot = s;
}

/** Read the snapshot without consuming it. */
export function peekFilesSnapshot(): FilesSnapshot | null {
	return snapshot;
}

/** Forget the snapshot (e.g. on logout). */
export function clearFilesSnapshot(): void {
	snapshot = null;
}

/** Record the file currently being viewed so back-navigation lands on it. */
export function setLastOpened(id: string): void {
	if (snapshot) snapshot = { ...snapshot, lastOpenedId: id };
}

/**
 * Append the next page to the snapshot using its own query/cursor. The file
 * viewer calls this to extend the cached list as the user pages forward, so
 * prev/next keep working past the originally loaded set and the grid restores
 * correctly on return. No-op when there is nothing cached, no further pages, or
 * a load is already in flight.
 */
export async function loadMoreIntoSnapshot(limit: number): Promise<void> {
	if (!snapshot || !snapshot.hasMore || loading) return;
	loading = true;
	try {
		const q = snapshot.query;
		const params = new URLSearchParams({ limit: String(limit), sort: q.sort, order: q.order });
		if (snapshot.nextCursor) params.set('cursor', snapshot.nextCursor);
		if (q.filter) params.set('filter', q.filter);
		const res = await api.get<FileCursorPage>(`/files?${params}`);
		// Re-read snapshot: it may have been replaced while the request was in flight.
		if (!snapshot) return;
		snapshot = {
			...snapshot,
			files: [...snapshot.files, ...(res.items ?? [])],
			nextCursor: res.next_cursor ?? null,
			hasMore: !!res.next_cursor,
		};
	} catch {
		// Non-critical: leave the snapshot unchanged.
	} finally {
		loading = false;
	}
}
