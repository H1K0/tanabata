<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { getDuplicates, dismissDuplicate } from '$lib/api/duplicates';
	import Thumb from '$lib/components/file/Thumb.svelte';
	import DuplicateMergeDialog from '$lib/components/file/DuplicateMergeDialog.svelte';
	import PreviewLightbox from '$lib/components/file/PreviewLightbox.svelte';
	import type { File } from '$lib/api/types';

	const LIMIT = 20;

	// A cluster carries a stable local key so resolving one pair (delete / dismiss /
	// merge) can edit it in place — no full reload, no scroll jump, no lost "keep".
	interface Cluster {
		key: number;
		files: File[];
	}
	let nextKey = 0;

	let clusters = $state<Cluster[]>([]);
	let total = $state(0);
	// Server group cursor; advances monotonically per page so local removals don't
	// shift the offset and make "Load more" repeat or skip clusters.
	let offset = $state(0);
	let loading = $state(false);
	let initialLoaded = $state(false);
	let error = $state('');
	let busyId = $state<number | null>(null); // cluster currently performing an action

	// Which file is the survivor for a given cluster (keyed by its stable key).
	let keepers = $state<Record<number, string>>({});

	// Merge dialog state — mergeId pins the cluster so onMerged edits the right one.
	let mergeId = $state<number | null>(null);
	let mergeKeep = $state<File | null>(null);
	let mergeDiscard = $state<File | null>(null);

	// Enlarged-preview lightbox: thumbnails are too small to tell near-duplicates
	// apart, so a zoom opens the full preview and pages across the cluster.
	let lightbox = $state<{ files: File[]; startId: string } | null>(null);

	$effect(() => {
		if (!initialLoaded && !loading) void load();
	});

	function keeperId(c: Cluster): string {
		return keepers[c.key] ?? c.files[0]?.id ?? '';
	}

	async function load() {
		if (loading) return;
		loading = true;
		error = '';
		try {
			const res = await getDuplicates(LIMIT, offset);
			const incoming = (res.items ?? []).map((c) => ({ key: nextKey++, files: c.files }));
			total = res.total ?? total;
			// The server paginates by group index and may drop groups that fell below
			// two live files, so advance by the page size (clamped), not items returned.
			offset = Math.min(offset + LIMIT, total);
			clusters = [...clusters, ...incoming];
		} catch {
			error = 'Failed to load duplicates';
		} finally {
			loading = false;
			initialLoaded = true;
		}
	}

	async function reload() {
		clusters = [];
		keepers = {};
		total = 0;
		offset = 0;
		initialLoaded = false;
		await load();
	}

	function setKeeper(c: Cluster, id: string) {
		keepers = { ...keepers, [c.key]: id };
	}

	// Drop one file from a cluster after it's resolved, in place. With fewer than
	// two files there's nothing left to compare, so the cluster — and its slot in
	// the total — goes away. The server's view is live, so a later page reconciles.
	function removeFile(key: number, fileId: string) {
		const target = clusters.find((c) => c.key === key);
		if (!target) return;
		const remaining = target.files.filter((f) => f.id !== fileId);
		const dropCluster = remaining.length < 2;

		if (dropCluster) {
			clusters = clusters.filter((c) => c.key !== key);
			total = Math.max(0, total - 1);
		} else {
			clusters = clusters.map((c) => (c.key === key ? { ...c, files: remaining } : c));
		}
		// Forget a stale survivor pick when its cluster is gone or the pick was removed.
		if (dropCluster || keepers[key] === fileId) {
			const next = { ...keepers };
			delete next[key];
			keepers = next;
		}
	}

	function openLightbox(c: Cluster, startId: string) {
		lightbox = { files: c.files, startId };
	}

	function openMerge(c: Cluster, other: File) {
		const keep = c.files.find((f) => f.id === keeperId(c));
		if (!keep) return;
		mergeId = c.key;
		mergeKeep = keep;
		mergeDiscard = other;
	}

	async function deleteFile(c: Cluster, id: string) {
		if (busyId !== null) return;
		busyId = c.key;
		try {
			await api.post('/files/bulk/delete', { file_ids: [id] });
			removeFile(c.key, id);
		} catch {
			error = 'Failed to delete file';
		} finally {
			busyId = null;
		}
	}

	async function notDuplicate(c: Cluster, other: File) {
		if (busyId !== null) return;
		busyId = c.key;
		try {
			await dismissDuplicate(keeperId(c), other.id);
			removeFile(c.key, other.id);
		} catch {
			error = 'Failed to dismiss pair';
		} finally {
			busyId = null;
		}
	}

	function onMerged() {
		const key = mergeId;
		const discardId = mergeDiscard?.id;
		mergeId = null;
		mergeKeep = null;
		mergeDiscard = null;
		if (key !== null && discardId) removeFile(key, discardId);
	}

	let hasMore = $derived(offset < total);
</script>

<svelte:head>
	<title>Duplicates | Tanabata</title>
</svelte:head>

<div class="page">
	<header>
		<button class="back" onclick={() => goto('/files')} aria-label="Back to files">
			<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
				<path
					d="M11 4l-5 5 5 5"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</button>
		<span class="htitle">Duplicates{total ? ` (${total})` : ''}</span>
		<button class="refresh" onclick={reload} title="Refresh" aria-label="Refresh">
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<path
					d="M13 8a5 5 0 1 1-1.5-3.5M13 2v3h-3"
					stroke="currentColor"
					stroke-width="1.6"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</button>
	</header>

	<main>
		{#if error}<p class="error" role="alert">{error}</p>{/if}

		{#if initialLoaded && clusters.length === 0 && !error}
			<div class="empty">
				<p>No duplicates found.</p>
				<p class="hint">
					The list reflects the last <code>dedup</code> run. New uploads appear after the next rescan.
				</p>
			</div>
		{/if}

		{#each clusters as c (c.key)}
			{@const keep = keeperId(c)}
			<section class="cluster" class:busy={busyId === c.key}>
				<div class="files">
					{#each c.files as f (f.id)}
						<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
						<div
							class="file"
							class:keep={f.id === keep}
							role="button"
							tabindex="0"
							onclick={() => setKeeper(c, f.id)}
							title="Click to keep this one"
						>
							<div class="thumbwrap">
								<Thumb id={f.id} size={96} alt={f.original_name ?? ''} />
								<button
									class="zoom"
									onclick={(e) => {
										e.stopPropagation();
										openLightbox(c, f.id);
									}}
									aria-label="Enlarge preview"
									title="Enlarge preview"
								>
									<svg width="15" height="15" viewBox="0 0 15 15" fill="none" aria-hidden="true">
										<circle cx="6.5" cy="6.5" r="4.5" stroke="currentColor" stroke-width="1.5" />
										<path
											d="M10 10l3.5 3.5"
											stroke="currentColor"
											stroke-width="1.5"
											stroke-linecap="round"
										/>
										<path
											d="M6.5 4.5v4M4.5 6.5h4"
											stroke="currentColor"
											stroke-width="1.3"
											stroke-linecap="round"
										/>
									</svg>
								</button>
							</div>
							{#if f.id === keep}<span class="kbadge">Keep</span>{/if}
							<span class="fname" title={f.original_name ?? ''}>{f.original_name ?? '—'}</span>
							<span class="fmeta">{f.mime_type} · {f.tags?.length ?? 0} tags</span>
						</div>
					{/each}
				</div>

				<div class="actions">
					{#each c.files.filter((f) => f.id !== keep) as other (other.id)}
						<div class="actrow">
							<span class="aname" title={other.original_name ?? ''}>{other.original_name ?? '—'}</span>
							<button class="abtn" onclick={() => openMerge(c, other)}>Merge</button>
							<button class="abtn" onclick={() => deleteFile(c, other.id)}>Delete</button>
							<button class="abtn ghost" onclick={() => notDuplicate(c, other)}>Not a dup</button>
						</div>
					{/each}
				</div>
			</section>
		{/each}

		{#if hasMore}
			<button class="more" onclick={load} disabled={loading}>
				{loading ? 'Loading…' : 'Load more'}
			</button>
		{:else if loading && !initialLoaded}
			<p class="loadingp">Loading…</p>
		{/if}
	</main>
</div>

{#if lightbox}
	<PreviewLightbox
		files={lightbox.files}
		startId={lightbox.startId}
		onClose={() => (lightbox = null)}
	/>
{/if}

{#if mergeKeep && mergeDiscard}
	<DuplicateMergeDialog
		keep={mergeKeep}
		discard={mergeDiscard}
		onResolved={onMerged}
		onClose={() => {
			mergeKeep = null;
			mergeDiscard = null;
		}}
	/>
{/if}

<style>
	.page {
		display: flex;
		flex-direction: column;
		height: 100%;
	}
	header {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		position: sticky;
		top: 0;
		z-index: 10;
		flex-shrink: 0;
	}
	.back,
	.refresh {
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		cursor: pointer;
	}
	.back:hover,
	.refresh:hover {
		color: var(--color-text-primary);
		border-color: var(--color-accent);
	}
	.htitle {
		flex: 1;
		font-size: 0.95rem;
		font-weight: 600;
	}
	main {
		flex: 1;
		overflow-y: auto;
		padding: 10px 12px calc(72px + env(safe-area-inset-bottom, 0px));
	}
	.error {
		color: var(--color-danger);
		font-size: 0.9rem;
		text-align: center;
	}
	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 40px 16px;
	}
	.empty .hint {
		font-size: 0.82rem;
		opacity: 0.8;
	}
	code {
		font-family: monospace;
		background-color: var(--color-bg-elevated);
		padding: 0 4px;
		border-radius: 4px;
	}
	.cluster {
		background-color: var(--color-bg-secondary);
		border-radius: 12px;
		padding: 12px;
		margin-bottom: 12px;
	}
	.cluster.busy {
		opacity: 0.55;
		pointer-events: none;
	}
	.files {
		display: flex;
		flex-wrap: wrap;
		gap: 10px;
		margin-bottom: 10px;
	}
	.file {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 3px;
		width: 96px;
		cursor: pointer;
		border-radius: 10px;
		padding: 4px;
		border: 2px solid transparent;
	}
	.file.keep {
		border-color: var(--color-accent);
		background-color: color-mix(in srgb, var(--color-accent) 10%, transparent);
	}
	.thumbwrap {
		position: relative;
		line-height: 0;
	}
	.zoom {
		position: absolute;
		top: 4px;
		right: 4px;
		width: 24px;
		height: 24px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 6px;
		border: none;
		background-color: rgba(0, 0, 0, 0.55);
		color: #fff;
		cursor: pointer;
		opacity: 0;
		transition: opacity 0.12s;
	}
	/* Always show the zoom on touch (no hover); reveal on hover for pointers. */
	.thumbwrap:hover .zoom,
	.zoom:focus-visible {
		opacity: 1;
	}
	@media (hover: none) {
		.zoom {
			opacity: 1;
		}
	}
	.zoom:hover {
		background-color: rgba(0, 0, 0, 0.85);
	}
	.kbadge {
		font-size: 0.68rem;
		font-weight: 600;
		color: var(--color-accent);
	}
	.fname {
		font-size: 0.74rem;
		max-width: 96px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		color: var(--color-text-primary);
	}
	.fmeta {
		font-size: 0.68rem;
		color: var(--color-text-muted);
		max-width: 96px;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.actions {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.actrow {
		display: flex;
		align-items: center;
		gap: 6px;
	}
	.aname {
		flex: 1;
		min-width: 0;
		font-size: 0.78rem;
		color: var(--color-text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.abtn {
		padding: 5px 10px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.78rem;
		font-family: inherit;
		cursor: pointer;
		flex-shrink: 0;
	}
	.abtn:hover {
		border-color: var(--color-accent);
	}
	.abtn.ghost {
		color: var(--color-text-muted);
	}
	.more {
		display: block;
		width: 100%;
		padding: 10px;
		border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-family: inherit;
		font-size: 0.85rem;
		cursor: pointer;
	}
	.loadingp {
		text-align: center;
		color: var(--color-text-muted);
		font-size: 0.9rem;
	}
</style>
