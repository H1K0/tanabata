<script lang="ts">
	import { get } from 'svelte/store';
	import { untrack, onDestroy } from 'svelte';
	import { api, ApiError } from '$lib/api/client';
	import { authStore } from '$lib/stores/auth';
	import TagPicker from '$lib/components/file/TagPicker.svelte';
	import PoolPicker from '$lib/components/file/PoolPicker.svelte';
	import type { File, Tag } from '$lib/api/types';

	interface Props {
		/** File currently shown. Changing it (paging) reloads in place. */
		fileId: string;
		/** Neighbour ids resolved by the parent; null hides the arrow. */
		prevId?: string | null;
		nextId?: string | null;
		/** Page to a neighbour. */
		onNavigate: (id: string) => void;
		/** Close the viewer. */
		onClose: () => void;
	}

	let { fileId, prevId = null, nextId = null, onNavigate, onClose }: Props = $props();

	let file = $state<File | null>(null);
	let fileTags = $state<Tag[]>([]);
	let previewSrc = $state<string | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let poolPickerOpen = $state(false);

	// Tags are loaded lazily — the Tags section sits below a full-viewport
	// preview, so fetching them on open just hammers the DB for data the user
	// usually never scrolls to. We fetch only once the section comes into view.
	let tagsVisible = $state(false);
	let tagsLoading = $state(false);
	let tagsLoadedFor = $state<string | null>(null);
	let tagsLoaded = $derived(tagsLoadedFor === fileId);

	// Editable fields (initialised on load)
	let notes = $state('');
	let contentDatetime = $state('');
	let isPublic = $state(false);
	let dirty = $state(false);

	let exifEntries = $derived(
		file?.exif ? Object.entries(file.exif as Record<string, unknown>) : []
	);

	// ---- Load (re-runs whenever the file changes, i.e. paging) ----
	$effect(() => {
		if (!fileId) return;
		const id = fileId; // snapshot — don't re-run if other state changes
		// Revoke old blob URL without tracking previewSrc as a dependency.
		untrack(() => {
			if (previewSrc) URL.revokeObjectURL(previewSrc);
			previewSrc = null;
		});
		void loadFile(id);
	});

	onDestroy(() => {
		if (previewSrc) URL.revokeObjectURL(previewSrc);
	});

	async function loadFile(id: string) {
		loading = true;
		error = '';
		// Drop the previous file's tags; they reload lazily when scrolled to.
		fileTags = [];
		try {
			const fileData = await api.get<File>(`/files/${id}`);
			if (fileId !== id) return; // paged on; ignore
			file = fileData;
			notes = fileData.notes ?? '';
			contentDatetime = fileData.content_datetime
				? fileData.content_datetime.slice(0, 16) // YYYY-MM-DDTHH:mm
				: '';
			isPublic = fileData.is_public ?? false;
			dirty = false;
			void fetchPreview(id);
			// Log the view (activity.file_views). Fire-and-forget — never block or
			// fail the viewer over view tracking.
			void api.post(`/files/${id}/views`).catch(() => {});
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
				headers: token ? { Authorization: `Bearer ${token}` } : {}
			});
			if (res.ok && fileId === id) {
				previewSrc = URL.createObjectURL(await res.blob());
			}
		} catch {
			// non-critical — thumbnail stays as fallback
		}
	}

	// Direct link to the full-resolution original, opened in a new tab. A
	// navigation can't send the auth header, so the token rides in the query —
	// the server accepts ?access_token= for GET media. Reactive on the token so a
	// silent refresh keeps the link valid.
	let originalUrl = $derived(
		fileId
			? `/api/v1/files/${fileId}/content?inline=1&access_token=${encodeURIComponent($authStore.accessToken ?? '')}`
			: '#'
	);

	// ---- Tags (lazy) ----
	// Fetch the current file's tags the first time the Tags section is visible.
	// Re-runs when fileId changes while the section is still on-screen.
	$effect(() => {
		const id = fileId;
		if (id && tagsVisible && tagsLoadedFor !== id && !tagsLoading) {
			void loadTags(id);
		}
	});

	async function loadTags(id: string) {
		tagsLoading = true;
		try {
			const tags = await api.get<Tag[]>(`/files/${id}/tags`);
			if (fileId !== id) return; // paged on; ignore
			fileTags = tags;
			tagsLoadedFor = id;
		} catch {
			// non-critical — a later scroll into view retries
		} finally {
			tagsLoading = false;
		}
	}

	// Svelte action: flips tagsVisible while the Tags section is in (or near) the
	// viewport. rootMargin pre-loads just before it scrolls fully into view.
	function tagsSentinel(node: HTMLElement) {
		const observer = new IntersectionObserver(
			(entries) => {
				tagsVisible = entries[0]?.isIntersecting ?? false;
			},
			{ rootMargin: '200px' }
		);
		observer.observe(node);
		return {
			destroy() {
				observer.disconnect();
			}
		};
	}

	async function addTag(tagId: string) {
		const updated = await api.put<Tag[]>(`/files/${fileId}/tags/${tagId}`);
		fileTags = updated;
		tagsLoadedFor = fileId;
	}

	async function removeTag(tagId: string) {
		await api.delete(`/files/${fileId}/tags/${tagId}`);
		fileTags = fileTags.filter((t) => t.id !== tagId);
	}

	// ---- Save ----
	async function save() {
		if (!file || saving) return;
		saving = true;
		error = '';
		try {
			const updated = await api.patch<File>(`/files/${file.id}`, {
				notes: notes.trim() || null,
				content_datetime: contentDatetime ? new Date(contentDatetime).toISOString() : undefined,
				is_public: isPublic
			});
			file = updated;
			dirty = false;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	// ---- Keyboard ----
	let viewerPage = $state<HTMLElement>();
	let tagsSection = $state<HTMLElement>();
	let pendingTagFocus = false;

	// Bring the preview back to the top of the scroll container (the overlay, or
	// the page in the standalone route). scrollIntoView resolves the right
	// scroller in either case. Called when Escape leaves the tag filter.
	function revealPreview() {
		viewerPage?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	function handleKeydown(e: KeyboardEvent) {
		// While the pool picker is open it owns the keyboard: Escape closes it
		// (even from its search field), and every other key is swallowed so the
		// viewer's shortcuts don't fire behind the modal. Typing still works —
		// non-Escape keys aren't prevented, only ignored here.
		if (poolPickerOpen) {
			if (e.key === 'Escape') {
				e.preventDefault();
				poolPickerOpen = false;
			}
			return;
		}
		if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return;
		if (e.ctrlKey || e.metaKey || e.altKey) return;
		// Letter keys are matched by physical position (e.code) so j/k/e work on any
		// keyboard layout; arrows and Escape are layout-independent already.
		if (e.key === 'ArrowLeft' || e.code === 'KeyK') {
			if (prevId) onNavigate(prevId);
		} else if (e.key === 'ArrowRight' || e.code === 'KeyJ') {
			if (nextId) onNavigate(nextId);
		} else if (e.code === 'KeyE') {
			e.preventDefault();
			jumpToTags();
		} else if (e.key === 'Escape') {
			onClose();
		}
	}

	// Scroll the (lazily loaded) Tags section into view and drop the cursor into
	// its filter. Forces the load so the focus lands even before the user reaches
	// the section by scrolling.
	function jumpToTags() {
		tagsVisible = true;
		tagsSection?.scrollIntoView({ behavior: 'smooth', block: 'start' });
		pendingTagFocus = true;
		focusTagInput();
	}

	function focusTagInput() {
		requestAnimationFrame(() => tagsSection?.querySelector<HTMLInputElement>('input')?.focus());
	}

	$effect(() => {
		if (tagsLoaded && pendingTagFocus) {
			pendingTagFocus = false;
			focusTagInput();
		}
	});

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

<svelte:window onkeydown={handleKeydown} />

<div class="viewer-page" bind:this={viewerPage}>
	<!-- Top bar -->
	<div class="top-bar">
		<button class="back-btn" onclick={onClose} aria-label="Back to files">
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
		<span class="filename">{file?.original_name ?? ''}</span>
		{#if file}
			<button
				class="pool-btn"
				onclick={() => (poolPickerOpen = true)}
				aria-label="Add to pool"
				title="Add to pool"
			>
				<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
					<rect
						x="3"
						y="5"
						width="14"
						height="11"
						rx="2"
						stroke="currentColor"
						stroke-width="1.6"
					/>
					<path
						d="M10 8.5v4M8 10.5h4"
						stroke="currentColor"
						stroke-width="1.6"
						stroke-linecap="round"
					/>
				</svg>
			</button>
		{/if}
	</div>

	<!-- Preview -->
	<div class="preview-wrap">
		{#if previewSrc}
			<a
				class="preview-link"
				href={originalUrl}
				target="_blank"
				rel="noopener"
				title="Open original in a new tab"
			>
				<img src={previewSrc} alt={file?.original_name ?? ''} class="preview-img" />
			</a>
		{:else if loading}
			<div class="preview-placeholder shimmer"></div>
		{:else}
			<div class="preview-placeholder failed"></div>
		{/if}

		<!-- Prev / Next -->
		{#if prevId}
			<button
				class="nav-btn nav-prev"
				onclick={() => prevId && onNavigate(prevId)}
				aria-label="Previous file"
			>
				<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
					<path
						d="M11 3L5 9L11 15"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
		{/if}
		{#if nextId}
			<button
				class="nav-btn nav-next"
				onclick={() => nextId && onNavigate(nextId)}
				aria-label="Next file"
			>
				<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
					<path
						d="M7 3L13 9L7 15"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
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
					onclick={() => {
						isPublic = !isPublic;
						dirty = true;
					}}
					role="switch"
					aria-checked={isPublic}
					aria-label="Public"
				>
					<span class="thumb"></span>
				</button>
			</section>

			<button class="save-btn" onclick={save} disabled={!dirty || saving}>
				{saving ? 'Saving…' : 'Save changes'}
			</button>

			<!-- Tags (loaded lazily on scroll) -->
			<section class="section" use:tagsSentinel bind:this={tagsSection}>
				<div class="field-label">Tags</div>
				{#if tagsLoaded}
					<TagPicker {fileTags} onAdd={addTag} onRemove={removeTag} onExit={revealPreview} />
				{:else}
					<p class="tags-loading">Loading tags…</p>
				{/if}
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

{#if poolPickerOpen && file}
	<PoolPicker fileIds={[file.id!]} onClose={() => (poolPickerOpen = false)} />
{/if}

<style>
	.viewer-page {
		display: flex;
		flex-direction: column;
		min-height: 0;
		padding-bottom: 70px; /* clear the bottom navbar in the standalone route */
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

	.pool-btn {
		margin-left: auto;
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

	.pool-btn:hover {
		background-color: color-mix(in srgb, var(--color-accent) 15%, transparent);
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

	/* Whole preview area is a link: click opens the original in a new tab. */
	.preview-link {
		width: 100%;
		height: 100%;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: zoom-in;
		text-decoration: none;
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
		background: linear-gradient(90deg, #111 25%, #222 50%, #111 75%);
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

	.nav-prev {
		left: 10px;
	}
	.nav-next {
		right: 10px;
	}

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

	.sep {
		opacity: 0.4;
	}

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
		transition:
			background-color 0.15s,
			opacity 0.15s;
	}

	.save-btn:hover:not(:disabled) {
		background-color: var(--color-accent-hover);
	}

	.save-btn:disabled {
		opacity: 0.4;
		cursor: default;
	}

	/* ---- Tags ---- */
	.tags-loading {
		margin: 0;
		font-size: 0.8rem;
		color: var(--color-text-muted);
		opacity: 0.7;
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
		0% {
			background-position: 200% 0;
		}
		100% {
			background-position: -200% 0;
		}
	}
</style>
