<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, ApiError } from '$lib/api/client';
	import type { Category, CategoryOffsetPage, Tag, TagRule } from '$lib/api/types';
	import TagRuleEditor from '$lib/components/tag/TagRuleEditor.svelte';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';

	let tagId = $derived(page.params.id);

	let tag = $state<Tag | null>(null);
	let categories = $state<Category[]>([]);
	let rules = $state<TagRule[]>([]);

	let name = $state('');
	let notes = $state('');
	let color = $state('#444455');
	let categoryId = $state('');
	let isPublic = $state(false);

	let saving = $state(false);
	let deleting = $state(false);
	let loadError = $state('');
	let saveError = $state('');
	let confirmDelete = $state(false);

	let loaded = $state(false);

	$effect(() => {
		const id = tagId;
		loaded = false;
		loadError = '';
		void Promise.all([
			api.get<Tag>(`/tags/${id}`),
			api.get<CategoryOffsetPage>('/categories?limit=200&sort=name&order=asc'),
			api.get<TagRule[]>(`/tags/${id}/rules`).catch(() => [] as TagRule[]),
		]).then(([t, cats, r]) => {
			tag = t;
			categories = cats.items ?? [];
			rules = r;

			name = t.name ?? '';
			notes = t.notes ?? '';
			color = t.color ? `#${t.color}` : '#444455';
			categoryId = t.category_id ?? '';
			isPublic = t.is_public ?? false;
			loaded = true;
		}).catch((e) => {
			loadError = e instanceof ApiError ? e.message : 'Failed to load tag';
		});
	});

	async function save() {
		if (!name.trim() || saving) return;
		saving = true;
		saveError = '';
		try {
			await api.patch(`/tags/${tagId}`, {
				name: name.trim(),
				notes: notes.trim() || null,
				color: color.slice(1),
				category_id: categoryId || null,
				is_public: isPublic,
			});
			goto('/tags');
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to save tag';
		} finally {
			saving = false;
		}
	}

	async function doDeleteTag() {
		confirmDelete = false;
		deleting = true;
		try {
			await api.delete(`/tags/${tagId}`);
			goto('/tags');
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to delete tag';
			deleting = false;
		}
	}
</script>

<svelte:head>
	<title>{tag?.name ?? 'Tag'} | Tanabata</title>
</svelte:head>

<div class="page">
	<header class="top-bar">
		<button class="back-btn" onclick={() => goto('/tags')} aria-label="Back">
			<svg width="20" height="20" viewBox="0 0 20 20" fill="none" aria-hidden="true">
				<path d="M12 4L6 10L12 16" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
			</svg>
		</button>
		<h1 class="page-title">{tag?.name ?? 'Tag'}</h1>
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
							placeholder="Tag name"
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

				<div class="field">
					<label class="label" for="category">Category</label>
					<select id="category" class="input" bind:value={categoryId}>
						<option value="">— None —</option>
						{#each categories as cat (cat.id)}
							<option value={cat.id}>{cat.name}</option>
						{/each}
					</select>
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

			<!-- Tag rules -->
			<section class="section">
				<h2 class="section-title">Implied tags</h2>
				<TagRuleEditor {tagId} {rules} onRulesChange={(r) => (rules = r)} />
			</section>

		{/if}
	</main>
</div>

{#if confirmDelete}
	<ConfirmDialog
		message={`Delete tag "${name}"? This cannot be undone.`}
		confirmLabel="Delete"
		danger
		onConfirm={doDeleteTag}
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

	main { flex: 1; overflow-y: auto; padding: 16px 14px calc(60px + 16px); display: flex; flex-direction: column; gap: 24px; }

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

	select.input { cursor: pointer; color-scheme: dark; }

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
	}

	.error { color: var(--color-danger); font-size: 0.875rem; margin: 0; }
</style>