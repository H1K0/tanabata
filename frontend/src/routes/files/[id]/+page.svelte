<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { get } from 'svelte/store';
	import { untrack } from 'svelte';
	import { api, ApiError } from '$lib/api/client';
	import { authStore } from '$lib/stores/auth';
	import { fileSorting } from '$lib/stores/sorting';
	import { appSettings } from '$lib/stores/appSettings';
	import { peekFilesSnapshot, setLastOpened, loadMoreIntoSnapshot } from '$lib/stores/filesCache';
	import TagPicker from '$lib/components/file/TagPicker.svelte';
	import type { File, Tag, FileCursorPage } from '$lib/api/types';

	// ---- State ----
	let fileId = $derived(page.params.id);

	let file = $state<File | null>(null);
	let fileTags = $state<Tag[]>([]);
	let previewSrc = $state<string | null>(null);
	let prevFile = $state<File | null>(null);
	let nextFile = $state<File | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');

	// Editable fields (initialised on load)
	let notes = $state('');
	let contentDatetime = $state('');
	let isPublic = $state(false);
	let dirty = $state(false);

	let exifEntries = $derived(
		file?.exif ? Object.entries(file.exif as Record<string, unknown>) : [],
	);

	// ---- Load ----
	$effect(() => {
		if (!fileId) return;
		const id = fileId; // snapshot — don't re-run if other state changes
		// Revoke old blob URL without tracking previewSrc as a dependency
		untrack(() => {
			if (previewSrc) URL.revokeObjectURL(previewSrc);
			previewSrc = null;
		});
		void loadPage(id);
	});

	async function loadPage(id: string) {
		loading = true;
		error = '';
		try {
			const [fileData, tags] = await Promise.all([
				api.get<File>(`/files/${id}`),
				api.get<Tag[]>(`/files/${id}/tags`),
			]);
			file = fileData;
			fileTags = tags;
			notes = fileData.notes ?? '';
			contentDatetime = fileData.content_datetime
				? fileData.content_datetime.slice(0, 16) // YYYY-MM-DDTHH:mm
				: '';
			isPublic = fileData.is_public ?? false;
			dirty = false;

			void fetchPreview(id);
			resolveNeighbors(id);
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load file';
		} finally {
			loading = false;
		}
	}

	async function fetchPreview(id: string) {
		const token = get(authStore).accessToken;
		try {
			const res = await fetch(`/api/v1/files/${id}/preview`, {
				headers: token ? { Authorization: `Bearer ${token}` } : {},
			});
			if (res.ok) {
				const blob = await res.blob();
				previewSrc = URL.createObjectURL(blob);
			}
		} catch {
			// non-critical — thumbnail stays as fallback
		}
	}

	// Derive prev/next from the shared grid snapshot so paging is symmetric and
	// instant and matches the order the user was browsing. As we approach the end
	// of the cached list, prefetch the next page into the snapshot so forward
	// paging continues and the grid restores correctly on return.
	function resolveNeighbors(id: string) {
		const snap = peekFilesSnapshot();
		const idx = snap ? snap.files.findIndex((f) => f.id === id) : -1;
		if (snap && idx >= 0) {
			prevFile = idx > 0 ? snap.files[idx - 1] : null;
			nextFile = idx < snap.files.length - 1 ? snap.files[idx + 1] : null;
			if (idx >= snap.files.length - 3 && snap.hasMore) {
				void loadMoreIntoSnapshot(get(appSettings).fileLoadLimit).then(() => {
					if (page.params.id !== id) return; // user navigated on; ignore
					const s2 = peekFilesSnapshot();
					const i2 = s2 ? s2.files.findIndex((f) => f.id === id) : -1;
					if (s2 && i2 >= 0) nextFile = i2 < s2.files.length - 1 ? s2.files[i2 + 1] : null;
				});
			}
			return;
		}
		// No cached grid (e.g. a deep link straight to this file) — fall back to
		// an anchored window from the API.
		void loadNeighborsAnchor(id);
	}

	async function loadNeighborsAnchor(id: string) {
		const sort = get(fileSorting);
		const params = new URLSearchParams({
			anchor: id,
			limit: '3',
			sort: sort.sort,
			order: sort.order,
		});
		try {
			const result = await api.get<FileCursorPage>(`/files?${params}`);
			const items = result.items ?? [];
			const idx = items.findIndex((f) => f.id === id);
			prevFile = idx > 0 ? items[idx - 1] : null;
			nextFile = idx >= 0 && idx < items.length - 1 ? items[idx + 1] : null;
		} catch {
			// non-critical
		}
	}

	// ---- Save ----
	async function save() {
		if (!file || saving) return;
		saving = true;
		error = '';
		try {
			const updated = await api.patch<File>(`/files/${file.id}`, {
				notes: notes.trim() || null,
				content_datetime: contentDatetime
					? new Date(contentDatetime).toISOString()
					: undefined,
				is_public: isPublic,
			});
			file = updated;
			dirty = false;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	// ---- Tags ----
	async function addTag(tagId: string) {
		const updated = await api.put<Tag[]>(`/files/${fileId}/tags/${tagId}`);
		fileTags = updated;
	}

	async function removeTag(tagId: string) {
		await api.delete(`/files/${fileId}/tags/${tagId}`);
		fileTags = fileTags.filter((t) => t.id !== tagId);
	}

	// ---- Navigation ----
	function navigateTo(f: File | null) {
		if (!f?.id) return;
		// Remember where we paged to, so returning to the grid lands here.
		setLastOpened(f.id);
		goto(`/files/${f.id}`);
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
		if (e.key === 'ArrowLeft') navigateTo(prevFile);
		if (e.key === 'ArrowRight') navigateTo(nextFile);
		if (e.key === 'Escape') goto('/files');
	}

	// ---- Helpers ----
	function formatDatetime(iso: string | null | undefined): string {
		if (!iso) return '—';
		return new Date(iso).toLocaleString();
	}

	// EXIF values may be nested arrays/objects (e.g. rationals, GPS); render those
	// as JSON instead of the useless "[object Object]".
	function formatExifValue(val: unknown): string {
		if (val === null || val === undefined) return '—';
		if (typeof val === 'object') return JSON.stringify(val);
		return String(val);
	}
</script>

<svelte:head>
	<title>
		{file?.original_name ?? fileId} | Tanabata
	</title>
</svelte:head>

<svelte:window onkeydown={handleKeydown} />

<div class="viewer-page">
	<!-- Top bar -->
	<div class="top-bar">
		<button class="back-btn" onclick={() => goto('/files')} aria-label="Back to files">
			<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
				<path d="M12 4L6 10L12 16" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
			</svg>
		</button>
		<span class="filename">{file?.original_name ?? ''}</span>
	</div>

	<!-- Preview -->
	<div class="preview-wrap">
		{#if previewSrc}
			<img src={previewSrc} alt={file?.original_name ?? ''} class="preview-img" />
		{:else if loading}
			<div class="preview-placeholder shimmer"></div>
		{:else}
			<div class="preview-placeholder failed"></div>
		{/if}

		<!-- Prev / Next -->
		{#if prevFile}
			<button
				class="nav-btn nav-prev"
				onclick={() => navigateTo(prevFile)}
				aria-label="Previous file"
			>
				<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
					<path d="M11 3L5 9L11 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</button>
		{/if}
		{#if nextFile}
			<button
				class="nav-btn nav-next"
				onclick={() => navigateTo(nextFile)}
				aria-label="Next file"
			>
				<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
					<path d="M7 3L13 9L7 15" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
				</svg>
			</button>
		{/if}
	</div>

	<!-- Metadata panel -->
	<div class="meta-panel">
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		{#if file}
			<!-- File info -->
			<div class="info-row">
				<span class="mime">{file.mime_type}</span>
				<span class="sep">·</span>
				<span class="created">Added {formatDatetime(file.created_at)}</span>
			</div>

			<!-- Edit form -->
			<section class="section">
				<label class="field-label" for="notes">Notes</label>
				<textarea
					id="notes"
					class="textarea"
					rows="3"
					bind:value={notes}
					oninput={() => (dirty = true)}
					placeholder="Add notes…"
				></textarea>
			</section>

			<section class="section">
				<label class="field-label" for="datetime">Date taken</label>
				<input
					id="datetime"
					type="datetime-local"
					class="input"
					bind:value={contentDatetime}
					oninput={() => (dirty = true)}
				/>
			</section>

			<section class="section toggle-row">
				<span class="field-label">Public</span>
				<button
					class="toggle"
					class:on={isPublic}
					onclick={() => { isPublic = !isPublic; dirty = true; }}
					role="switch"
					aria-checked={isPublic}
					aria-label="Public"
				>
					<span class="thumb"></span>
				</button>
			</section>

			<button
				class="save-btn"
				onclick={save}
				disabled={!dirty || saving}
			>
				{saving ? 'Saving…' : 'Save changes'}
			</button>

			<!-- Tags -->
			<section class="section">
				<div class="field-label">Tags</div>
				<TagPicker {fileTags} onAdd={addTag} onRemove={removeTag} />
			</section>

			<!-- EXIF -->
			{#if exifEntries.length > 0}
				<section class="section">
					<div class="field-label">EXIF</div>
					<dl class="exif">
						{#each exifEntries as [key, val]}
							<dt>{key}</dt>
							<dd>{formatExifValue(val)}</dd>
						{/each}
					</dl>
				</section>
			{/if}
		{:else if !loading}
			<p class="empty">File not found.</p>
		{/if}
	</div>
</div>

<style>
	.viewer-page {
		display: flex;
		flex-direction: column;
		min-height: 0;
		padding-bottom: 70px; /* clear navbar */
	}

	/* ---- Top bar ---- */
	.top-bar {
		position: sticky;
		top: 0;
		z-index: 20;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 10px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		min-height: 44px;
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

	.filename {
		font-size: 0.9rem;
		color: var(--color-text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* ---- Preview ---- */
	.preview-wrap {
		position: relative;
		background-color: #000;
		display: flex;
		align-items: center;
		justify-content: center;
		/* Fill viewport below the top bar (44px) */
		height: calc(100dvh - 44px);
		flex-shrink: 0;
		overflow: hidden;
	}

	.preview-img {
		max-width: 100%;
		max-height: 100%;
		object-fit: contain;
		display: block;
	}

	.preview-placeholder {
		width: 100%;
		height: 100%;
	}

	.preview-placeholder.shimmer {
		background: linear-gradient(
			90deg,
			#111 25%,
			#222 50%,
			#111 75%
		);
		background-size: 200% 100%;
		animation: shimmer 1.4s infinite;
	}

	.preview-placeholder.failed {
		background-color: #1a1010;
	}

	/* ---- Nav buttons ---- */
	.nav-btn {
		position: absolute;
		top: 50%;
		transform: translateY(-50%);
		width: 40px;
		height: 40px;
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

	.nav-btn:hover {
		background-color: rgba(0, 0, 0, 0.8);
	}

	.nav-prev { left: 10px; }
	.nav-next { right: 10px; }

	/* ---- Metadata panel ---- */
	.meta-panel {
		padding: 14px 14px 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.info-row {
		display: flex;
		align-items: center;
		gap: 6px;
		font-size: 0.8rem;
		color: var(--color-text-muted);
		padding-bottom: 10px;
	}

	.sep { opacity: 0.4; }

	.section {
		padding: 10px 0;
		border-top: 1px solid color-mix(in srgb, var(--color-accent) 12%, transparent);
	}

	.field-label {
		font-size: 0.75rem;
		font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		margin-bottom: 6px;
	}

	.textarea {
		width: 100%;
		box-sizing: border-box;
		padding: 8px 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		resize: vertical;
		outline: none;
		min-height: 70px;
	}

	.textarea:focus {
		border-color: var(--color-accent);
	}

	.input {
		width: 100%;
		box-sizing: border-box;
		height: 36px;
		padding: 0 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		outline: none;
		color-scheme: dark;
	}

	.input:focus {
		border-color: var(--color-accent);
	}

	/* ---- Toggle ---- */
	.toggle-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding-top: 12px;
		padding-bottom: 12px;
	}

	.toggle-row .field-label {
		margin-bottom: 0;
	}

	.toggle {
		position: relative;
		width: 44px;
		height: 26px;
		border-radius: 13px;
		border: none;
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-elevated));
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

	/* ---- Save button ---- */
	.save-btn {
		width: 100%;
		height: 40px;
		border-radius: 8px;
		border: none;
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-size: 0.9rem;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
		margin-top: 4px;
		margin-bottom: 4px;
		transition: background-color 0.15s, opacity 0.15s;
	}

	.save-btn:hover:not(:disabled) {
		background-color: var(--color-accent-hover);
	}

	.save-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}

	/* ---- EXIF ---- */
	.exif {
		display: grid;
		grid-template-columns: auto 1fr;
		gap: 3px 12px;
		font-size: 0.78rem;
		margin: 0;
	}

	dt {
		color: var(--color-text-muted);
		font-weight: 500;
	}

	dd {
		margin: 0;
		color: var(--color-text-primary);
		word-break: break-word;
	}

	/* ---- Misc ---- */
	.error {
		color: var(--color-danger);
		font-size: 0.875rem;
		padding: 8px 0;
	}

	.empty {
		color: var(--color-text-muted);
		font-size: 0.95rem;
		text-align: center;
		padding: 40px 0;
	}

	@keyframes shimmer {
		0% { background-position: 200% 0; }
		100% { background-position: -200% 0; }
	}
</style>