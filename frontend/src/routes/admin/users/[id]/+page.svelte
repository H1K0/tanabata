<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import type { User } from '$lib/api/types';

	let userId = $derived(page.params.id);

	let user = $state<User | null>(null);
	let loading = $state(true);
	let error = $state('');
	let saving = $state(false);
	let saveError = $state('');
	let saveSuccess = $state(false);
	let confirmDelete = $state(false);
	let deleting = $state(false);

	// editable fields
	let isAdmin = $state(false);
	let canCreate = $state(false);
	let isBlocked = $state(false);

	$effect(() => {
		const id = userId;
		loading = true;
		error = '';
		void api.get<User>(`/users/${id}`).then((u) => {
			user = u;
			isAdmin = u.is_admin ?? false;
			canCreate = u.can_create ?? false;
			isBlocked = u.is_blocked ?? false;
		}).catch((e) => {
			error = e instanceof ApiError ? e.message : 'Failed to load user';
		}).finally(() => {
			loading = false;
		});
	});

	async function save() {
		if (saving || !user) return;
		saving = true;
		saveError = '';
		saveSuccess = false;
		try {
			const updated = await api.patch<User>(`/users/${user.id}`, {
				is_admin: isAdmin,
				can_create: canCreate,
				is_blocked: isBlocked,
			});
			user = updated;
			saveSuccess = true;
			setTimeout(() => (saveSuccess = false), 2500);
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to save';
		} finally {
			saving = false;
		}
	}

	async function doDelete() {
		confirmDelete = false;
		deleting = true;
		try {
			await api.delete(`/users/${user!.id}`);
			goto('/admin/users');
		} catch (e) {
			saveError = e instanceof ApiError ? e.message : 'Failed to delete';
			deleting = false;
		}
	}
</script>

<svelte:head><title>{user?.name ?? 'User'} — Admin | Tanabata</title></svelte:head>

<div class="page">
	<button class="back-link" onclick={() => goto('/admin/users')}>
		← All users
	</button>

	{#if error}
		<p class="msg error" role="alert">{error}</p>
	{:else if loading}
		<div class="loading"><span class="spinner" role="status" aria-label="Loading"></span></div>
	{:else if user}
		<div class="card">
			<div class="user-header">
				<span class="user-name">{user.name}</span>
				<span class="user-id">#{user.id}</span>
			</div>

			{#if saveError}<p class="msg error" role="alert">{saveError}</p>{/if}
			{#if saveSuccess}<p class="msg success" role="status">Saved.</p>{/if}

			<div class="section-label">Role & permissions</div>

			<div class="toggle-group">
				<div class="toggle-row">
					<div>
						<span class="toggle-label">Admin</span>
						<p class="toggle-hint">Full access to all data and admin panel.</p>
					</div>
					<button
						class="toggle" class:on={isAdmin}
						role="switch" aria-checked={isAdmin}
						onclick={() => (isAdmin = !isAdmin)}
					><span class="thumb"></span></button>
				</div>

				<div class="toggle-row">
					<div>
						<span class="toggle-label">Can create</span>
						<p class="toggle-hint">Can upload files and create tags, pools, categories.</p>
					</div>
					<button
						class="toggle" class:on={canCreate}
						role="switch" aria-checked={canCreate}
						onclick={() => (canCreate = !canCreate)}
					><span class="thumb"></span></button>
				</div>
			</div>

			<div class="section-label">Account status</div>

			<div class="toggle-group">
				<div class="toggle-row">
					<div>
						<span class="toggle-label" class:danger-label={isBlocked}>Blocked</span>
						<p class="toggle-hint">Blocked users cannot log in.</p>
					</div>
					<button
						class="toggle" class:on={isBlocked} class:danger={isBlocked}
						role="switch" aria-checked={isBlocked}
						onclick={() => (isBlocked = !isBlocked)}
					><span class="thumb"></span></button>
				</div>
			</div>

			<div class="action-row">
				<button class="btn primary" onclick={save} disabled={saving}>
					{saving ? 'Saving…' : 'Save changes'}
				</button>
				<button class="btn danger-outline" onclick={() => (confirmDelete = true)} disabled={deleting}>
					{deleting ? 'Deleting…' : 'Delete user'}
				</button>
			</div>
		</div>
	{/if}
</div>

{#if confirmDelete && user}
	<ConfirmDialog
		message="Delete user &ldquo;{user.name}&rdquo;? This cannot be undone."
		confirmLabel="Delete"
		danger
		onConfirm={doDelete}
		onCancel={() => (confirmDelete = false)}
	/>
{/if}

<style>
	.page {
		padding: 16px;
		max-width: 520px;
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.back-link {
		background: none;
		border: none;
		color: var(--color-text-muted);
		font-size: 0.85rem;
		cursor: pointer;
		padding: 0;
		text-align: left;
		font-family: inherit;
	}

	.back-link:hover { color: var(--color-accent); }

	.card {
		background-color: var(--color-bg-elevated);
		border-radius: 12px;
		padding: 18px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.user-header {
		display: flex;
		align-items: baseline;
		gap: 8px;
	}

	.user-name {
		font-size: 1.15rem;
		font-weight: 700;
	}

	.user-id {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.section-label {
		font-size: 0.72rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.07em;
		color: var(--color-text-muted);
		margin-bottom: -6px;
	}

	.toggle-group {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.toggle-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.toggle-label {
		font-size: 0.9rem;
		font-weight: 500;
	}

	.toggle-label.danger-label {
		color: var(--color-danger);
	}

	.toggle-hint {
		font-size: 0.78rem;
		color: var(--color-text-muted);
		margin: 2px 0 0;
	}

	/* toggle switch */
	.toggle {
		flex-shrink: 0;
		position: relative;
		width: 40px;
		height: 22px;
		border-radius: 11px;
		border: none;
		background-color: color-mix(in srgb, var(--color-accent) 22%, var(--color-bg-primary));
		cursor: pointer;
		padding: 0;
		transition: background-color 0.15s;
	}

	.toggle.on { background-color: var(--color-accent); }
	.toggle.on.danger { background-color: var(--color-danger); }

	.toggle .thumb {
		position: absolute;
		top: 3px;
		left: 3px;
		width: 16px;
		height: 16px;
		border-radius: 50%;
		background-color: #fff;
		transition: transform 0.15s;
	}

	.toggle.on .thumb { transform: translateX(18px); }

	.action-row {
		display: flex;
		gap: 8px;
		margin-top: 4px;
	}

	.btn {
		height: 34px;
		padding: 0 16px;
		border-radius: 7px;
		font-size: 0.875rem;
		font-family: inherit;
		font-weight: 600;
		cursor: pointer;
		border: none;
	}

	.btn:disabled { opacity: 0.5; cursor: default; }

	.btn.primary {
		background-color: var(--color-accent);
		color: #fff;
	}

	.btn.primary:hover:not(:disabled) {
		background-color: var(--color-accent-hover);
	}

	.btn.danger-outline {
		background: none;
		border: 1px solid var(--color-danger);
		color: var(--color-danger);
	}

	.btn.danger-outline:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--color-danger) 12%, transparent);
	}

	.msg {
		font-size: 0.85rem;
		margin: 0;
	}

	.msg.error { color: var(--color-danger); }
	.msg.success { color: #7ECBA1; }

	.loading {
		display: flex;
		justify-content: center;
		padding: 40px;
	}

	.spinner {
		display: block;
		width: 28px;
		height: 28px;
		border: 3px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-top-color: var(--color-accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin { to { transform: rotate(360deg); } }
</style>