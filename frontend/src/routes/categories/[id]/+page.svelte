<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, ApiError } from '$lib/api/client';
	import type { Category, Tag, TagOffsetPage } from '$lib/api/types';
	import TagBadge from '$lib/components/tag/TagBadge.svelte';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';

	let categoryId = $derived(page.params.id);

	let category = $state<Category | null>(null);
	let tags = $state<Tag[]>([]);
	let tagsTotal = $state(0);
	let tagsOffset = $state(0);
	let tagsLoading = $state(false);

	let name = $state('');
	let notes = $state('');
	let color = $state('#9592B5');
	let isPublic = $state(false);

	let saving = $state(false);
	let deleting = $state(false);
	let loadError = $state('');
	let saveError = $state('');
	let loaded = $state(false);
	let confirmDelete = $state(false);

	const TAGS_LIMIT = 100;

	$effect(() => {
		const id = categoryId;
		loaded = false;
		loadError = '';
		tags = [];
		tagsOffset = 0;
		tagsTotal = 0;
		void api.get<Category>(`/categories/${id}`).then((cat) => {
			category = cat;
			name = cat.name ?? '';
			notes = cat.notes ?? '';
			color = cat.color ? `#${cat.color}` : '#9592B5';
			isPublic = cat.is_public ?? false;
			loaded = true;
		}).catch((e) => {
			loadError = e instanceof ApiError ? e.message : 'Failed to load category';
		});
		void loadTags(id, 0);
	});

	async function loadTags(id: string | undefined, startOffset: number) {
		if (!id) return;
		tagsLoading = true;
		try {
			const params = new URLSearchParams({
				limit: String(TAGS_LIMIT),
				offset: String(startOffset),
				sort: 'name',
				order: 'asc',
			});
			const p = await api.get<TagOffsetPage>(`/categories/${id}/tags?${params}`);
			tags = startOffset === 0 ? (p.items ?? []) : [...tags, ...(p.items ?? [])];
			tagsTotal = p.total ?? 0;
			tagsOffset = tags.length;
		} catch {
			// non-fatal — tags section just stays empty
		} finally {
			tagsLoading = false;
		}
	}

	let tagsHasMore = $derived(tags.length < tagsTotal);

	async function save() {
		if (!name.trim() || saving) return;
		saving = true;
		saveError = '';
		try {
			await api.patch(`/categories/${categoryId}`, {
				name: name.trim(),
				notes: notes.trim() || null,
				color: color.slice(1),
				is_public: isPublic,
			});
			goto('/categories');
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to save category';
		} finally {
			saving = false;
		}
	}

	async function doDeleteCategory() {
		confirmDelete = false;
		deleting = true;
		try {
			await api.delete(`/categories/${categoryId}`);
			goto('/categories');
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to delete category';
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>{category?.name ?? 'Category'} | Tanabata</title>
</svelte:head>

<div class="page">
	<header class="top-bar">
		<button class="back-btn" onclick={() => goto('/categories')} aria-label="Back">
			<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
				<path d="M12 4L6 10L12 16" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
			</svg>
		</button>
		<h1 class="page-title">{category?.name ?? 'Category'}</h1>
	</header>

	<main>
		{#if loadError}
			<p class="error" role="alert">{loadError}</p>
		{:else if !loaded}
			<div class="loading-row">
				<span class="spinner" role="status" aria-label="Loading"></span>
			</div>
		{:else}
			{#if saveError}
				<p class="error" role="alert">{saveError}</p>
			{/if}

			<form class="form" onsubmit={(e) => { e.preventDefault(); void save(); }}>
				<div class="row-fields">
					<div class="field" style="flex: 1">
						<label class="label" for="name">Name <span class="required">*</span></label>
						<input
							id="name"
							class="input"
							type="text"
							bind:value={name}
							required
							placeholder="Category name"
							autocomplete="off"
						/>
					</div>
					<div class="field color-field">
						<label class="label" for="color">Color</label>
						<input id="color" class="color-input" type="color" bind:value={color} />
					</div>
				</div>

				<div class="field">
					<label class="label" for="notes">Notes</label>
					<textarea id="notes" class="textarea" rows="3" bind:value={notes} placeholder="Optional notes…"></textarea>
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
					<button type="submit" class="submit-btn" disabled={!name.trim() || saving}>
						{saving ? 'Saving…' : 'Save changes'}
					</button>
					<button type="button" class="delete-btn" onclick={() => (confirmDelete = true)} disabled={deleting}>
						{deleting ? 'Deleting…' : 'Delete'}
					</button>
				</div>
			</form>

			<!-- Tags in this category -->
			<section class="section">
				<h2 class="section-title">
					Tags
					{#if tagsTotal > 0}<span class="count">({tagsTotal})</span>{/if}
				</h2>

				{#if tagsLoading && tags.length === 0}
					<div class="loading-row">
						<span class="spinner" role="status" aria-label="Loading"></span>
					</div>
				{:else if tags.length === 0}
					<p class="empty-tags">No tags in this category.</p>
				{:else}
					<div class="tag-grid">
						{#each tags as tag (tag.id)}
							<TagBadge {tag} onclick={() => goto(`/tags/${tag.id}`)} size="sm" />
						{/each}
					</div>

					{#if tagsHasMore}
						<button
							class="load-more"
							onclick={() => loadTags(categoryId, tagsOffset)}
							disabled={tagsLoading}
						>
							{tagsLoading ? 'Loading…' : 'Load more'}
						</button>
					{/if}
				{/if}
			</section>
		{/if}
	</main>
</div>

{#if confirmDelete}
	<ConfirmDialog
		message={`Delete category "${name}"? Tags in this category will be unassigned.`}
		confirmLabel="Delete"
		danger
		onConfirm={doDeleteCategory}
		onCancel={() => (confirmDelete = false)}
	/>
{/if}

<style>
	.page { flex: 1; min-height: 0; display: flex; flex-direction: column; }

	.top-bar {
		position: sticky; top: 0; z-index: 10;
		display: flex; align-items: center; gap: 8px;
		padding: 6px 10px; min-height: 44px;
		background-color: var(--color-bg-primary);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		flex-shrink: 0;
	}

	.back-btn {
		display: flex; align-items: center; justify-content: center;
		width: 36px; height: 36px; border-radius: 8px;
		border: none; background: none;
		color: var(--color-text-primary); cursor: pointer;
	}
	.back-btn:hover { background-color: color-mix(in srgb, var(--color-accent) 15%, transparent); }

	.page-title { font-size: 1rem; font-weight: 600; margin: 0; }

	main {
		flex: 1; overflow-y: auto;
		padding: 16px 14px calc(60px + 16px);
		display: flex; flex-direction: column; gap: 24px;
	}

	.loading-row { display: flex; justify-content: center; padding: 40px; }

	.spinner {
		display: block; width: 28px; height: 28px;
		border: 3px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-top-color: var(--color-accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }

	.form { display: flex; flex-direction: column; gap: 14px; }

	.row-fields { display: flex; gap: 10px; align-items: flex-end; }

	.field { display: flex; flex-direction: column; gap: 5px; }

	.color-field { flex-shrink: 0; }

	.label {
		font-size: 0.75rem; font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase; letter-spacing: 0.05em;
	}

	.required { color: var(--color-danger); }

	.input {
		width: 100%; box-sizing: border-box;
		height: 36px; padding: 0 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.875rem; font-family: inherit; outline: none;
	}
	.input:focus { border-color: var(--color-accent); }

	.color-input {
		width: 50px; height: 36px; padding: 2px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		cursor: pointer;
	}

	.textarea {
		width: 100%; box-sizing: border-box; padding: 8px 10px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.875rem; font-family: inherit;
		resize: vertical; outline: none; min-height: 70px;
	}
	.textarea:focus { border-color: var(--color-accent); }

	.toggle-row {
		display: flex; align-items: center;
		justify-content: space-between;
		padding: 4px 0;
	}
	.toggle-row .label { margin: 0; }

	.toggle {
		position: relative; width: 44px; height: 26px;
		border-radius: 13px; border: none;
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-elevated));
		cursor: pointer; transition: background-color 0.2s; flex-shrink: 0;
	}
	.toggle.on { background-color: var(--color-accent); }
	.thumb {
		position: absolute; top: 3px; left: 3px;
		width: 20px; height: 20px; border-radius: 50%;
		background-color: #fff; transition: transform 0.2s;
	}
	.toggle.on .thumb { transform: translateX(18px); }

	.action-row { display: flex; gap: 8px; }

	.submit-btn {
		flex: 1; height: 42px; border-radius: 8px; border: none;
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
		font-size: 0.9rem; font-weight: 600; font-family: inherit; cursor: pointer;
	}
	.submit-btn:hover:not(:disabled) { background-color: var(--color-accent-hover); }
	.submit-btn:disabled { opacity: 0.4; cursor: default; }

	.delete-btn {
		height: 42px; padding: 0 18px; border-radius: 8px;
		border: 1px solid color-mix(in srgb, var(--color-danger) 50%, transparent);
		background: none; color: var(--color-danger);
		font-size: 0.9rem; font-weight: 600; font-family: inherit; cursor: pointer;
	}
	.delete-btn:hover:not(:disabled) { background-color: color-mix(in srgb, var(--color-danger) 12%, transparent); }
	.delete-btn:disabled { opacity: 0.4; cursor: default; }

	.section { display: flex; flex-direction: column; gap: 10px; }

	.section-title {
		font-size: 0.75rem; font-weight: 600;
		color: var(--color-text-muted);
		text-transform: uppercase; letter-spacing: 0.05em;
		margin: 0;
		padding-bottom: 6px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
		display: flex; gap: 6px; align-items: baseline;
	}

	.count {
		font-weight: 400;
		color: var(--color-text-muted);
	}

	.tag-grid {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}

	.empty-tags {
		font-size: 0.85rem;
		color: var(--color-text-muted);
		margin: 0;
	}

	.load-more {
		display: block;
		margin-top: 8px;
		padding: 6px 20px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 40%, transparent);
		background: none;
		color: var(--color-accent);
		font-family: inherit;
		font-size: 0.82rem;
		cursor: pointer;
	}

	.load-more:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--color-accent) 10%, transparent);
	}

	.load-more:disabled { opacity: 0.5; cursor: default; }

	.error { color: var(--color-danger); font-size: 0.875rem; margin: 0; }
</style>