// In-memory, per-section view cache. When you leave a list (Files, Tags, …) for
// another section and come back, the page restores its loaded items, pagination
// cursors and scroll position from here instead of refetching from scratch.
//
// Kept deliberately simple: a plain module-level Map that lives for the session.
// No TTL — a snapshot is taken from the page's current state on the way out, so
// it already reflects local mutations (deletes, uploads, tag edits). It is
// dropped on a full reload, and each page validates the snapshot's `resetKey`
// (sort/filter/search) before trusting it, so a stale query never restores.

export type SectionKey = 'files' | 'tags' | 'categories' | 'pools';

/** Snapshot shape shared by the offset-paginated lists (tags/categories/pools). */
export interface OffsetListSnapshot<T> {
	/** sort|order|search at capture — guards against restoring a different query. */
	resetKey: string;
	search: string;
	items: T[];
	total: number;
	offset: number;
}

interface Snapshot<T> {
	/** Scroll offset of the list's scroller at capture time. */
	scrollTop: number;
	/** Page-specific state blob; opaque to this module. */
	data: T;
	savedAt: number;
}

const cache = new Map<SectionKey, Snapshot<unknown>>();

export function saveSection<T>(key: SectionKey, scrollTop: number, data: T): void {
	cache.set(key, { scrollTop, data, savedAt: Date.now() });
}

/** Read and remove a section's snapshot (restore consumes it). */
export function takeSection<T>(key: SectionKey): { scrollTop: number; data: T } | null {
	const snap = cache.get(key) as Snapshot<T> | undefined;
	if (!snap) return null;
	cache.delete(key);
	return { scrollTop: snap.scrollTop, data: snap.data };
}

export function clearSection(key: SectionKey): void {
	cache.delete(key);
}
