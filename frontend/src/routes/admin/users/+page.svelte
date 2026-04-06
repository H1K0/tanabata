<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError } from '$lib/api/client';
	import ConfirmDialog from '$lib/components/common/ConfirmDialog.svelte';
	import type { User, UserOffsetPage } from '$lib/api/types';

	const LIMIT = 100;

	let users = $state<User[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state('');

	// Create form
	let showCreate = $state(false);
	let newName = $state('');
	let newPassword = $state('');
	let newCanCreate = $state(false);
	let newIsAdmin = $state(false);
	let creating = $state(false);
	let createError = $state('');

	// Delete confirm
	let confirmDeleteUser = $state<User | null>(null);

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await api.get<UserOffsetPage>(`/users?limit=${LIMIT}&offset=0`);
			users = res.items ?? [];
			total = res.total ?? users.length;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load users';
		} finally {
			loading = false;
		}
	}

	async function createUser() {
		if (!newName.trim() || !newPassword.trim()) return;
		creating = true;
		createError = '';
		try {
			const u = await api.post<User>('/users', {
				name: newName.trim(),
				password: newPassword.trim(),
				can_create: newCanCreate,
				is_admin: newIsAdmin,
			});
			users = [u, ...users];
			total++;
			showCreate = false;
			newName = '';
			newPassword = '';
			newCanCreate = false;
			newIsAdmin = false;
		} catch (e) {
			createError = e instanceof ApiError ? e.message : 'Failed to create user';
		} finally {
			creating = false;
		}
	}

	async function deleteUser(u: User) {
		confirmDeleteUser = null;
		try {
			await api.delete(`/users/${u.id}`);
			users = users.filter((x) => x.id !== u.id);
			total--;
		} catch {
			// silently ignore
		}
	}

	$effect(() => { void load(); });
</script>

<svelte:head><title>Users — Admin | Tanabata</title></svelte:head>

<div class="page">
	<div class="toolbar">
		<span class="count">{total} user{total !== 1 ? 's' : ''}</span>
		<button class="btn primary" onclick={() => (showCreate = !showCreate)}>
			{showCreate ? 'Cancel' : '+ New user'}
		</button>
	</div>

	{#if showCreate}
		<div class="create-form">
			{#if createError}<p class="form-error" role="alert">{createError}</p>{/if}
			<div class="form-row">
				<input class="input" type="text" placeholder="Username" bind:value={newName} autocomplete="off" />
				<input class="input" type="password" placeholder="Password" bind:value={newPassword} autocomplete="new-password" />
			</div>
			<div class="form-row checks">
				<label class="check-label">
					<input type="checkbox" bind:checked={newCanCreate} />
					Can create
				</label>
				<label class="check-label">
					<input type="checkbox" bind:checked={newIsAdmin} />
					Admin
				</label>
				<button
					class="btn primary"
					onclick={createUser}
					disabled={creating || !newName.trim() || !newPassword.trim()}
				>
					{creating ? 'Creating…' : 'Create'}
				</button>
			</div>
		</div>
	{/if}

	{#if error}
		<p class="error" role="alert">{error}</p>
	{:else if loading}
		<div class="loading"><span class="spinner" role="status" aria-label="Loading"></span></div>
	{:else if users.length === 0}
		<p class="empty">No users found.</p>
	{:else}
		<table class="table">
			<thead>
				<tr>
					<th>ID</th>
					<th>Name</th>
					<th>Role</th>
					<th>Status</th>
					<th></th>
				</tr>
			</thead>
			<tbody>
				{#each users as u (u.id)}
					<tr class="user-row" class:blocked={u.is_blocked}>
						<td class="id-cell">{u.id}</td>
						<td class="name-cell">
							<button class="name-btn" onclick={() => goto(`/admin/users/${u.id}`)}>
								{u.name}
							</button>
						</td>
						<td>
							<span class="badge" class:admin={u.is_admin} class:creator={!u.is_admin && u.can_create}>
								{u.is_admin ? 'Admin' : u.can_create ? 'Creator' : 'Viewer'}
							</span>
						</td>
						<td>
							{#if u.is_blocked}
								<span class="badge blocked">Blocked</span>
							{:else}
								<span class="badge active">Active</span>
							{/if}
						</td>
						<td class="actions-cell">
							<button class="icon-btn" onclick={() => goto(`/admin/users/${u.id}`)} title="Edit">
								<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
									<path d="M9.5 2.5l2 2-7 7H2.5v-2l7-7z" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/>
								</svg>
							</button>
							<button class="icon-btn danger" onclick={() => (confirmDeleteUser = u)} title="Delete">
								<svg width="14" height="14" viewBox="0 0 14 14" fill="none" aria-hidden="true">
									<path d="M2 4h10M5 4V2.5h4V4M5.5 6.5v4M8.5 6.5v4M3 4l.8 7.5h6.4L11 4" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
								</svg>
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>

{#if confirmDeleteUser}
	<ConfirmDialog
		message="Delete user &ldquo;{confirmDeleteUser.name}&rdquo;? This cannot be undone."
		confirmLabel="Delete"
		danger
		onConfirm={() => deleteUser(confirmDeleteUser!)}
		onCancel={() => (confirmDeleteUser = null)}
	/>
{/if}

<style>
	.page {
		padding: 16px;
		max-width: 760px;
		margin: 0 auto;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.count {
		font-size: 0.85rem;
		color: var(--color-text-muted);
	}

	.create-form {
		background-color: var(--color-bg-elevated);
		border-radius: 10px;
		padding: 14px;
		display: flex;
		flex-direction: column;
		gap: 10px;
	}

	.form-row {
		display: flex;
		gap: 8px;
		flex-wrap: wrap;
	}

	.form-row.checks {
		align-items: center;
	}

	.input {
		flex: 1;
		min-width: 140px;
		height: 34px;
		padding: 0 10px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.875rem;
		font-family: inherit;
		outline: none;
	}

	.input:focus {
		border-color: var(--color-accent);
	}

	.check-label {
		display: flex;
		align-items: center;
		gap: 5px;
		font-size: 0.85rem;
		cursor: pointer;
		user-select: none;
	}

	.form-error {
		font-size: 0.82rem;
		color: var(--color-danger);
		margin: 0;
	}

	.btn {
		height: 32px;
		padding: 0 14px;
		border-radius: 7px;
		border: none;
		font-size: 0.85rem;
		font-family: inherit;
		font-weight: 600;
		cursor: pointer;
		white-space: nowrap;
	}

	.btn:disabled { opacity: 0.5; cursor: default; }

	.btn.primary {
		background-color: var(--color-accent);
		color: #fff;
	}

	.btn.primary:hover:not(:disabled) {
		background-color: var(--color-accent-hover);
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.875rem;
	}

	.table th {
		text-align: left;
		padding: 6px 10px;
		font-size: 0.75rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
	}

	.table td {
		padding: 9px 10px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 10%, transparent);
		vertical-align: middle;
	}

	.user-row.blocked td {
		opacity: 0.55;
	}

	.id-cell {
		color: var(--color-text-muted);
		font-size: 0.8rem;
		width: 40px;
	}

	.name-btn {
		background: none;
		border: none;
		color: var(--color-text-primary);
		font-size: inherit;
		font-family: inherit;
		cursor: pointer;
		padding: 0;
		font-weight: 500;
	}

	.name-btn:hover {
		color: var(--color-accent);
		text-decoration: underline;
	}

	.badge {
		display: inline-block;
		font-size: 0.72rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		padding: 2px 7px;
		border-radius: 4px;
		background-color: color-mix(in srgb, var(--color-accent) 15%, transparent);
		color: var(--color-text-muted);
	}

	.badge.admin {
		background-color: color-mix(in srgb, var(--color-warning) 20%, transparent);
		color: var(--color-warning);
	}

	.badge.creator {
		background-color: color-mix(in srgb, var(--color-info) 15%, transparent);
		color: var(--color-info);
	}

	.badge.active {
		background-color: color-mix(in srgb, #7ECBA1 15%, transparent);
		color: #7ECBA1;
	}

	.badge.blocked {
		background-color: color-mix(in srgb, var(--color-danger) 15%, transparent);
		color: var(--color-danger);
	}

	.actions-cell {
		display: flex;
		gap: 4px;
		justify-content: flex-end;
	}

	.icon-btn {
		width: 28px;
		height: 28px;
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background: none;
		color: var(--color-text-muted);
		cursor: pointer;
	}

	.icon-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-accent);
	}

	.icon-btn.danger:hover {
		color: var(--color-danger);
		border-color: var(--color-danger);
	}

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

	.error, .empty {
		font-size: 0.875rem;
		color: var(--color-text-muted);
		text-align: center;
		padding: 40px 0;
	}

	.error { color: var(--color-danger); }
</style>