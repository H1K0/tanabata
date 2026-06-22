<script lang="ts">
	import type { File } from '$lib/api/types';
	import {
		resolveDuplicate,
		type MergeFields,
		type MetadataChoice,
		type RelationChoice,
		type ScalarChoice
	} from '$lib/api/duplicates';
	import Thumb from '$lib/components/file/Thumb.svelte';

	interface Props {
		/** The two files to merge; `keep` is the default survivor (swappable here). */
		keep: File;
		discard: File;
		/** Called with the updated survivor after a successful merge. */
		onResolved: (survivor: File) => void;
		onClose: () => void;
	}

	let { keep, discard, onResolved, onClose }: Props = $props();

	// Which file survives is swappable; derive the two sides from a single flag so
	// the choice stays in sync with the props.
	let swapped = $state(false);
	let a = $derived<File>(swapped ? discard : keep);
	let b = $derived<File>(swapped ? keep : discard);

	// Per-field source, all defaulting to the survivor ("keep").
	let original_name = $state<ScalarChoice>('keep');
	let notes = $state<ScalarChoice>('keep');
	let content_datetime = $state<ScalarChoice>('keep');
	let is_public = $state<ScalarChoice>('keep');
	let metadata = $state<MetadataChoice>('keep');
	let tags = $state<RelationChoice>('keep');
	let pools = $state<RelationChoice>('keep');
	let deleteDiscarded = $state(true);

	let busy = $state(false);
	let error = $state('');

	function swap() {
		swapped = !swapped;
	}

	function fmtDate(s?: string | null): string {
		if (!s) return '—';
		const d = new Date(s);
		return isNaN(d.getTime()) ? s : d.toLocaleString();
	}
	function metaCount(m: unknown): number {
		return m && typeof m === 'object' ? Object.keys(m as object).length : 0;
	}
	function metaEntries(m: unknown): [string, unknown][] {
		return m && typeof m === 'object' ? Object.entries(m as Record<string, unknown>) : [];
	}
	function fmtMeta(v: unknown): string {
		if (v === null || v === undefined) return '—';
		if (typeof v === 'object') return JSON.stringify(v);
		return String(v);
	}

	async function submit() {
		if (busy) return;
		busy = true;
		error = '';
		const fields: MergeFields = {
			original_name,
			notes,
			content_datetime,
			is_public,
			metadata,
			tags,
			pools
		};
		try {
			const survivor = await resolveDuplicate({
				keep: a.id,
				discard: b.id,
				fields,
				delete_discarded: deleteDiscarded
			});
			onResolved(survivor);
			onClose();
		} catch {
			error = 'Failed to merge';
			busy = false;
		}
	}
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="backdrop" role="presentation" onclick={onClose}></div>
<div class="sheet" class:busy role="dialog" aria-label="Merge duplicates">
	<div class="head">
		<span class="title">Merge duplicates</span>
		<button class="x" onclick={onClose} aria-label="Close">
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

	<div class="body">
		<!-- Survivor / other headers -->
		<div class="files">
			<div class="file">
				<Thumb id={a.id} size={80} alt={a.original_name ?? ''} />
				<span class="badge keep">Keep</span>
				<span class="fname" title={a.original_name ?? ''}>{a.original_name ?? '—'}</span>
			</div>
			<button class="swap" onclick={swap} title="Swap which file is kept" aria-label="Swap">
				<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
					<path
						d="M5 4h8l-2.5-2.5M13 14H5l2.5 2.5"
						stroke="currentColor"
						stroke-width="1.6"
						stroke-linecap="round"
						stroke-linejoin="round"
					/>
				</svg>
			</button>
			<div class="file">
				<Thumb id={b.id} size={80} alt={b.original_name ?? ''} />
				<span class="badge other">Other</span>
				<span class="fname" title={b.original_name ?? ''}>{b.original_name ?? '—'}</span>
			</div>
		</div>

		<!-- Scalar fields: keep vs discard -->
		{#snippet scalarRow(
			label: string,
			value: ScalarChoice,
			set: (v: ScalarChoice) => void,
			keepVal: string,
			otherVal: string
		)}
			<div class="row">
				<span class="label">{label}</span>
				<div class="seg">
					<button class:on={value === 'keep'} onclick={() => set('keep')} title={keepVal}>
						{keepVal || '—'}
					</button>
					<button class:on={value === 'discard'} onclick={() => set('discard')} title={otherVal}>
						{otherVal || '—'}
					</button>
				</div>
			</div>
		{/snippet}

		{@render scalarRow(
			'Name',
			original_name,
			(v) => (original_name = v),
			a.original_name ?? '',
			b.original_name ?? ''
		)}
		{@render scalarRow('Notes', notes, (v) => (notes = v), a.notes ?? '', b.notes ?? '')}
		{@render scalarRow(
			'Date',
			content_datetime,
			(v) => (content_datetime = v),
			fmtDate(a.content_datetime),
			fmtDate(b.content_datetime)
		)}
		{@render scalarRow(
			'Visibility',
			is_public,
			(v) => (is_public = v),
			a.is_public ? 'Public' : 'Private',
			b.is_public ? 'Public' : 'Private'
		)}

		<!-- Metadata: keep / other / merge -->
		<div class="row">
			<span class="label">Metadata</span>
			<div class="seg">
				<button class:on={metadata === 'keep'} onclick={() => (metadata = 'keep')}>
					Keep ({metaCount(a.metadata)})
				</button>
				<button class:on={metadata === 'discard'} onclick={() => (metadata = 'discard')}>
					Other ({metaCount(b.metadata)})
				</button>
				<button class:on={metadata === 'merge'} onclick={() => (metadata = 'merge')}>Merge</button>
			</div>
		</div>

		<!-- Compact side-by-side view of each side's metadata -->
		{#if metaCount(a.metadata) > 0 || metaCount(b.metadata) > 0}
			<div class="meta-preview">
				{#each [{ side: 'Keep', m: a.metadata }, { side: 'Other', m: b.metadata }] as col (col.side)}
					<div class="meta-col">
						<div class="meta-col-head">{col.side}</div>
						{#if metaEntries(col.m).length > 0}
							<dl class="meta-list">
								{#each metaEntries(col.m) as [k, v]}
									<dt title={k}>{k}</dt>
									<dd title={fmtMeta(v)}>{fmtMeta(v)}</dd>
								{/each}
							</dl>
						{:else}
							<span class="meta-none">—</span>
						{/if}
					</div>
				{/each}
			</div>
		{/if}

		<!-- Relations: keep vs union both -->
		<div class="row">
			<span class="label">Tags</span>
			<div class="seg">
				<button class:on={tags === 'keep'} onclick={() => (tags = 'keep')}>
					Keep ({a.tags?.length ?? 0})
				</button>
				<button class:on={tags === 'both'} onclick={() => (tags = 'both')}>Union both</button>
			</div>
		</div>
		<div class="row">
			<span class="label">Pools</span>
			<div class="seg">
				<button class:on={pools === 'keep'} onclick={() => (pools = 'keep')}>Keep</button>
				<button class:on={pools === 'both'} onclick={() => (pools = 'both')}>Union both</button>
			</div>
		</div>

		<label class="del">
			<input type="checkbox" bind:checked={deleteDiscarded} />
			Move the “Other” file to trash after merging
		</label>

		{#if error}<p class="error">{error}</p>{/if}
	</div>

	<div class="foot">
		<button class="btn ghost" onclick={onClose}>Cancel</button>
		<button class="btn primary" onclick={submit} disabled={busy}>Merge</button>
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 120;
		background: rgba(0, 0, 0, 0.5);
	}
	.sheet {
		position: fixed;
		left: 0;
		right: 0;
		bottom: 0;
		z-index: 121;
		background-color: var(--color-bg-secondary);
		border-radius: 14px 14px 0 0;
		padding-bottom: env(safe-area-inset-bottom, 0px);
		max-height: 88dvh;
		display: flex;
		flex-direction: column;
		animation: slide-up 0.18s ease-out;
	}
	.sheet.busy {
		opacity: 0.6;
		pointer-events: none;
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
	.head {
		display: flex;
		align-items: center;
		padding: 14px 16px 10px;
		gap: 8px;
	}
	.title {
		flex: 1;
		font-size: 0.95rem;
		font-weight: 600;
	}
	.x {
		background: none;
		border: none;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 4px;
		display: flex;
	}
	.x:hover {
		color: var(--color-text-primary);
	}
	.body {
		overflow-y: auto;
		padding: 0 14px 8px;
	}
	.files {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
		margin-bottom: 12px;
	}
	.file {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 4px;
		min-width: 0;
		max-width: 40%;
	}
	.badge {
		font-size: 0.7rem;
		font-weight: 600;
		padding: 1px 7px;
		border-radius: 6px;
	}
	.badge.keep {
		background-color: color-mix(in srgb, var(--color-accent) 30%, transparent);
		color: var(--color-accent);
	}
	.badge.other {
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
	}
	.fname {
		font-size: 0.78rem;
		color: var(--color-text-muted);
		max-width: 100%;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.swap {
		background-color: var(--color-bg-elevated);
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		color: var(--color-text-muted);
		border-radius: 8px;
		width: 32px;
		height: 32px;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		flex-shrink: 0;
	}
	.swap:hover {
		color: var(--color-accent);
		border-color: var(--color-accent);
	}
	.row {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 7px 0;
		border-top: 1px solid color-mix(in srgb, var(--color-accent) 12%, transparent);
	}
	.label {
		font-size: 0.82rem;
		color: var(--color-text-muted);
		width: 74px;
		flex-shrink: 0;
	}
	.seg {
		display: flex;
		flex: 1;
		gap: 4px;
		min-width: 0;
	}
	.seg button {
		flex: 1;
		min-width: 0;
		padding: 6px 8px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-muted);
		font-size: 0.8rem;
		font-family: inherit;
		cursor: pointer;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.seg button.on {
		background-color: color-mix(in srgb, var(--color-accent) 22%, var(--color-bg-elevated));
		color: var(--color-accent);
		border-color: var(--color-accent);
	}
	.meta-preview {
		display: flex;
		gap: 8px;
		padding: 2px 0 4px;
	}
	.meta-col {
		flex: 1;
		min-width: 0;
		background-color: var(--color-bg-elevated);
		border-radius: 7px;
		padding: 6px 8px;
	}
	.meta-col-head {
		font-size: 0.66rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--color-text-muted);
		margin-bottom: 4px;
	}
	.meta-list {
		display: grid;
		grid-template-columns: minmax(0, auto) minmax(0, 1fr);
		gap: 2px 8px;
		margin: 0;
		font-size: 0.72rem;
	}
	.meta-list dt {
		color: var(--color-text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.meta-list dd {
		margin: 0;
		color: var(--color-text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.meta-none {
		font-size: 0.72rem;
		color: var(--color-text-muted);
		opacity: 0.6;
	}
	.del {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 0.82rem;
		color: var(--color-text-muted);
		padding: 12px 0 4px;
	}
	.error {
		color: var(--color-danger);
		font-size: 0.85rem;
		text-align: center;
	}
	.foot {
		display: flex;
		gap: 8px;
		padding: 10px 14px calc(10px + env(safe-area-inset-bottom, 0px));
		border-top: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
	}
	.btn {
		flex: 1;
		height: 38px;
		border-radius: 8px;
		border: 1px solid transparent;
		font-size: 0.9rem;
		font-family: inherit;
		cursor: pointer;
	}
	.btn.ghost {
		background-color: var(--color-bg-elevated);
		border-color: color-mix(in srgb, var(--color-accent) 25%, transparent);
		color: var(--color-text-muted);
	}
	.btn.primary {
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-weight: 600;
	}
	.btn.primary:disabled {
		opacity: 0.6;
		cursor: default;
	}
</style>
