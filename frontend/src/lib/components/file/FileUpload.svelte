<script lang="ts">
	import { uploadWithProgress, ApiError } from '$lib/api/client';
	import type { File as ApiFile } from '$lib/api/types';
	import type { Snippet } from 'svelte';

	interface Props {
		onUploaded: (file: ApiFile) => void;
		children: Snippet;
	}

	let { onUploaded, children }: Props = $props();

	// ---- Upload queue ----
	type UploadStatus = 'uploading' | 'done' | 'error';

	interface QueueItem {
		id: string;
		name: string;
		progress: number;
		status: UploadStatus;
		error?: string;
	}

	let queue = $state<QueueItem[]>([]);
	let fileInput = $state<HTMLInputElement | undefined>();

	let allSettled = $derived(queue.length > 0 && queue.every((i) => i.status !== 'uploading'));

	// ---- File input ----
	export function open() {
		fileInput?.click();
	}

	function onInputChange(e: Event) {
		const files = (e.currentTarget as HTMLInputElement).files;
		if (files?.length) {
			void enqueue(Array.from(files));
			// Reset so the same file can be re-selected
			(e.currentTarget as HTMLInputElement).value = '';
		}
	}

	// ---- Upload logic ----
	function uid() {
		return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
	}

	async function enqueue(files: globalThis.File[]) {
		const items: QueueItem[] = files.map((f) => ({
			id: uid(),
			name: f.name,
			progress: 0,
			status: 'uploading'
		}));
		queue = [...queue, ...items];

		await Promise.all(files.map((file, i) => uploadOne(file, items[i].id)));
	}

	async function uploadOne(file: globalThis.File, itemId: string) {
		const fd = new FormData();
		fd.append('file', file);

		try {
			const result = await uploadWithProgress<ApiFile>('/files', fd, (pct) =>
				updateItem(itemId, { progress: pct })
			);
			updateItem(itemId, { status: 'done', progress: 100 });
			onUploaded(result);
		} catch (e) {
			const msg =
				e instanceof ApiError
					? e.status === 415
						? `Unsupported file type`
						: e.message
					: 'Upload failed';
			updateItem(itemId, { status: 'error', error: msg });
		}
	}

	function updateItem(id: string, patch: Partial<QueueItem>) {
		queue = queue.map((item) => (item.id === id ? { ...item, ...patch } : item));
	}

	function clearQueue() {
		queue = [];
	}

	// ---- Drag and drop ----
	let dragCounter = $state(0);
	let dragOver = $derived(dragCounter > 0);

	function onDragEnter(e: DragEvent) {
		if (!e.dataTransfer?.types.includes('Files')) return;
		e.preventDefault();
		dragCounter++;
	}

	function onDragLeave() {
		dragCounter = Math.max(0, dragCounter - 1);
	}

	function onDragOver(e: DragEvent) {
		if (!e.dataTransfer?.types.includes('Files')) return;
		e.preventDefault();
		e.dataTransfer.dropEffect = 'copy';
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragCounter = 0;
		const files = Array.from(e.dataTransfer?.files ?? []);
		if (files.length) void enqueue(files);
	}
</script>

<!-- Hidden file input -->
<input
	bind:this={fileInput}
	type="file"
	multiple
	accept="image/*,video/*"
	style="display:none"
	onchange={onInputChange}
/>

<!-- Drop zone wrapper -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	class="drop-zone"
	class:drag-over={dragOver}
	ondragenter={onDragEnter}
	ondragleave={onDragLeave}
	ondragover={onDragOver}
	ondrop={onDrop}
>
	{@render children()}

	{#if dragOver}
		<div class="drop-overlay" aria-hidden="true">
			<div class="drop-label">
				<svg width="36" height="36" viewBox="0 0 36 36" fill="none" aria-hidden="true">
					<path
						d="M18 4v20M10 14l8-10 8 10"
						stroke="currentColor"
						stroke-width="2.5"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
					<path d="M6 28h24" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
				</svg>
				Drop files to upload
			</div>
		</div>
	{/if}
</div>

<!-- Upload progress panel -->
{#if queue.length > 0}
	<div class="upload-panel" role="status">
		<div class="panel-header">
			<span class="panel-title">
				{#if allSettled}
					Uploads complete
				{:else}
					Uploading {queue.filter((i) => i.status === 'uploading').length} file(s)…
				{/if}
			</span>
			{#if allSettled}
				<button class="clear-btn" onclick={clearQueue}>Dismiss</button>
			{/if}
		</div>

		<ul class="upload-list">
			{#each queue as item (item.id)}
				<li
					class="upload-item"
					class:done={item.status === 'done'}
					class:error={item.status === 'error'}
				>
					<span class="item-name" title={item.name}>{item.name}</span>
					<div class="item-right">
						{#if item.status === 'uploading'}
							<div class="progress-track">
								<div class="progress-fill" style="width: {item.progress}%"></div>
							</div>
							<span class="pct">{item.progress}%</span>
						{:else if item.status === 'done'}
							<svg
								class="icon-ok"
								width="16"
								height="16"
								viewBox="0 0 16 16"
								fill="none"
								aria-label="Done"
							>
								<path
									d="M3 8l4 4 6-6"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
								/>
							</svg>
						{:else}
							<span class="err-msg" title={item.error}>{item.error}</span>
						{/if}
					</div>
				</li>
			{/each}
		</ul>
	</div>
{/if}

<style>
	/* ---- Drop zone ---- */
	.drop-zone {
		position: relative;
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.drop-overlay {
		position: absolute;
		inset: 0;
		z-index: 50;
		background-color: color-mix(in srgb, var(--color-accent) 18%, rgba(0, 0, 0, 0.7));
		display: flex;
		align-items: center;
		justify-content: center;
		border: 2px dashed var(--color-accent);
		border-radius: 4px;
		pointer-events: none;
	}

	.drop-label {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		color: #fff;
		font-size: 1.1rem;
		font-weight: 600;
	}

	/* ---- Upload panel ---- */
	.upload-panel {
		position: fixed;
		left: 10px;
		right: 10px;
		bottom: 65px;
		z-index: 110;
		background-color: var(--color-bg-secondary);
		border-radius: 10px;
		box-shadow: 0 0 16px rgba(0, 0, 0, 0.6);
		padding: 10px 12px;
		animation: slide-up 0.18s ease-out;
		max-height: 50vh;
		overflow-y: auto;
	}

	@keyframes slide-up {
		from {
			transform: translateY(10px);
			opacity: 0;
		}
		to {
			transform: translateY(0);
			opacity: 1;
		}
	}

	.panel-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin-bottom: 8px;
	}

	.panel-title {
		font-size: 0.85rem;
		font-weight: 600;
		color: var(--color-text-muted);
	}

	.clear-btn {
		background: none;
		border: none;
		color: var(--color-accent);
		font-size: 0.82rem;
		font-family: inherit;
		cursor: pointer;
		padding: 2px 6px;
	}

	.clear-btn:hover {
		text-decoration: underline;
	}

	.upload-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.upload-item {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 28px;
	}

	.item-name {
		flex: 1;
		font-size: 0.82rem;
		color: var(--color-text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.upload-item.done .item-name {
		color: var(--color-text-muted);
	}

	.upload-item.error .item-name {
		color: var(--color-text-muted);
	}

	.item-right {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-shrink: 0;
	}

	.progress-track {
		width: 80px;
		height: 4px;
		background-color: color-mix(in srgb, var(--color-accent) 20%, var(--color-bg-elevated));
		border-radius: 2px;
		overflow: hidden;
	}

	.progress-fill {
		height: 100%;
		background-color: var(--color-accent);
		border-radius: 2px;
		transition: width 0.1s linear;
	}

	.pct {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		min-width: 30px;
		text-align: right;
	}

	.icon-ok {
		color: var(--color-accent);
	}

	.err-msg {
		font-size: 0.75rem;
		color: var(--color-danger);
		max-width: 140px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
