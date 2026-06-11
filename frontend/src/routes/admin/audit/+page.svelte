<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import type { AuditEntry, AuditOffsetPage, User, UserOffsetPage } from '$lib/api/types';

	const LIMIT = 50;
	const OBJECT_TYPES = ['file', 'tag', 'category', 'pool'];
	const ACTION_LABELS: Record<string, string> = {
		// Auth
		user_login: 'User logged in',
		user_logout: 'User logged out',
		// Files
		file_create: 'File uploaded',
		file_edit: 'File edited',
		file_delete: 'File deleted',
		file_restore: 'File restored',
		file_permanent_delete: 'File permanently deleted',
		file_replace: 'File replaced',
		// Tags
		tag_create: 'Tag created',
		tag_edit: 'Tag edited',
		tag_delete: 'Tag deleted',
		// Categories
		category_create: 'Category created',
		category_edit: 'Category edited',
		category_delete: 'Category deleted',
		// Pools
		pool_create: 'Pool created',
		pool_edit: 'Pool edited',
		pool_delete: 'Pool deleted',
		// Relations
		file_tag_add: 'Tag added to file',
		file_tag_remove: 'Tag removed from file',
		file_pool_add: 'File added to pool',
		file_pool_remove: 'File removed from pool',
		// ACL
		acl_change: 'ACL changed',
		// Admin
		user_create: 'User created',
		user_delete: 'User deleted',
		user_block: 'User blocked',
		user_unblock: 'User unblocked',
		user_role_change: 'User role changed',
		// Sessions
		session_terminate: 'Session terminated'
	};

	// ---- Filters ----
	let filterUserId = $state('');
	let filterAction = $state('');
	let filterObjectType = $state('');
	let filterObjectId = $state('');
	let filterFrom = $state('');
	let filterTo = $state('');

	// ---- Data ----
	let entries = $state<AuditEntry[]>([]);
	let total = $state(0);
	let page = $state(0); // 0-based
	let loading = $state(false);
	let error = $state('');
	let initialLoaded = $state(false);

	let totalPages = $derived(Math.max(1, Math.ceil(total / LIMIT)));

	// ---- Users for filter dropdown ----
	let allUsers = $state<User[]>([]);
	$effect(() => {
		api
			.get<UserOffsetPage>('/users?limit=200')
			.then((r) => {
				allUsers = r.items ?? [];
			})
			.catch(() => {});
	});

	// Unknown action types not in ACTION_LABELS (server may add new ones)
	let knownActions = $derived(
		[...new Set(entries.map((e) => e.action).filter(Boolean))].sort() as string[]
	);

	// ---- Reset on filter change ----
	let filterKey = $derived(
		`${filterUserId}|${filterAction}|${filterObjectType}|${filterObjectId}|${filterFrom}|${filterTo}`
	);
	let prevFilterKey = $state('');

	$effect(() => {
		if (filterKey !== prevFilterKey) {
			prevFilterKey = filterKey;
			page = 0;
			initialLoaded = false;
			error = '';
		}
	});

	$effect(() => {
		if (!initialLoaded && !loading) void load();
	});

	async function load() {
		if (loading) return;
		loading = true;
		error = '';
		try {
			const params = new URLSearchParams({ limit: String(LIMIT), offset: String(page * LIMIT) });
			if (filterUserId) params.set('user_id', filterUserId);
			if (filterAction) params.set('action', filterAction);
			if (filterObjectType) params.set('object_type', filterObjectType);
			if (filterObjectId.trim()) params.set('object_id', filterObjectId.trim());
			if (filterFrom) params.set('from', new Date(filterFrom).toISOString());
			if (filterTo) params.set('to', new Date(filterTo).toISOString());

			const res = await api.get<AuditOffsetPage>(`/audit?${params}`);
			entries = res.items ?? [];
			total = res.total ?? entries.length;
		} catch (e) {
			error = e instanceof ApiError ? e.message : 'Failed to load audit log';
		} finally {
			loading = false;
			initialLoaded = true;
		}
	}

	async function goToPage(p: number) {
		if (p < 0 || p >= totalPages || p === page) return;
		page = p;
		initialLoaded = false;
	}

	function formatTs(iso: string | undefined | null): string {
		if (!iso) return '—';
		const d = new Date(iso);
		return d.toLocaleString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit'
		});
	}

	function actionLabel(action: string | undefined | null): string {
		if (!action) return '—';
		return ACTION_LABELS[action] ?? action.replace(/_/g, ' ');
	}

	function shortId(id: string | undefined | null): string {
		if (!id) return '—';
		return id.slice(-8);
	}

	function clearFilters() {
		filterUserId = '';
		filterAction = '';
		filterObjectType = '';
		filterObjectId = '';
		filterFrom = '';
		filterTo = '';
	}

	let filtersActive = $derived(
		!!(filterUserId || filterAction || filterObjectType || filterObjectId || filterFrom || filterTo)
	);
</script>

<svelte:head><title>Audit Log — Admin | Tanabata</title></svelte:head>

<div class="page">
	<!-- Filters -->
	<div class="filters">
		<div class="filters-row">
			<select class="filter-select" bind:value={filterUserId} title="Filter by user">
				<option value="">All users</option>
				{#each allUsers as u (u.id)}
					<option value={String(u.id)}>{u.name}</option>
				{/each}
			</select>

			<select class="filter-select" bind:value={filterAction} title="Filter by action">
				<option value="">All actions</option>
				{#each Object.keys(ACTION_LABELS) as a}
					<option value={a}>{ACTION_LABELS[a]}</option>
				{/each}
				{#each knownActions.filter((a) => !(a in ACTION_LABELS)) as a}
					<option value={a}>{a}</option>
				{/each}
			</select>

			<select class="filter-select" bind:value={filterObjectType} title="Filter by object type">
				<option value="">All objects</option>
				{#each OBJECT_TYPES as t}
					<option value={t}>{t}</option>
				{/each}
			</select>

			<input
				class="filter-input"
				type="text"
				placeholder="Object ID…"
				bind:value={filterObjectId}
				autocomplete="off"
			/>
		</div>

		<div class="filters-row">
			<label class="date-label">
				From
				<input class="filter-input date" type="datetime-local" bind:value={filterFrom} />
			</label>
			<label class="date-label">
				To
				<input class="filter-input date" type="datetime-local" bind:value={filterTo} />
			</label>
			{#if filtersActive}
				<button class="clear-btn" onclick={clearFilters}>Clear filters</button>
			{/if}
			<span class="total-hint">{total} entr{total !== 1 ? 'ies' : 'y'}</span>
		</div>
	</div>

	<!-- Table -->
	{#if error}
		<p class="msg error" role="alert">{error}</p>
	{:else}
		<div class="content-area">
			<div class="table-wrap">
				<table class="table">
					<thead>
						<tr>
							<th>Time</th>
							<th>User</th>
							<th>Action</th>
							<th>Object</th>
							<th>ID</th>
						</tr>
					</thead>
					<tbody>
						{#each entries as e (e.id)}
							<tr>
								<td class="ts-cell">{formatTs(e.performed_at)}</td>
								<td class="user-cell">{e.user_name ?? '—'}</td>
								<td class="action-cell">
									<span
										class="action-tag"
										class:file={e.object_type === 'file'}
										class:tag={e.object_type === 'tag'}
										class:pool={e.object_type === 'pool'}
										class:cat={e.object_type === 'category'}
									>
										{actionLabel(e.action)}
									</span>
								</td>
								<td class="obj-type-cell">{e.object_type ?? '—'}</td>
								<td class="obj-id-cell" title={e.object_id ?? ''}>{shortId(e.object_id)}</td>
							</tr>
						{/each}

						{#if loading}
							<tr class="loading-row">
								<td colspan="5">
									<span class="spinner" role="status" aria-label="Loading"></span>
								</td>
							</tr>
						{/if}

						{#if !loading && initialLoaded && entries.length === 0}
							<tr>
								<td colspan="5" class="empty-cell">No entries match the current filters.</td>
							</tr>
						{/if}
					</tbody>
				</table>
			</div>

			{#if totalPages > 1}
				<div class="pagination">
					<button
						class="page-btn"
						onclick={() => goToPage(page - 1)}
						disabled={page === 0 || loading}
					>
						← Prev
					</button>
					<span class="page-info">Page {page + 1} of {totalPages}</span>
					<button
						class="page-btn"
						onclick={() => goToPage(page + 1)}
						disabled={page >= totalPages - 1 || loading}
					>
						Next →
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>

<style>
	.page {
		padding: 14px 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
		height: 100%;
		box-sizing: border-box;
	}

	/* ---- Filters ---- */
	.filters {
		display: flex;
		flex-direction: column;
		gap: 8px;
		flex-shrink: 0;
	}

	.filters-row {
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
		align-items: center;
	}

	.filter-select,
	.filter-input {
		height: 32px;
		padding: 0 8px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-elevated);
		color: var(--color-text-primary);
		font-size: 0.82rem;
		font-family: inherit;
		outline: none;
	}

	.filter-select:focus,
	.filter-input:focus {
		border-color: var(--color-accent);
	}

	.filter-input {
		min-width: 140px;
	}

	.filter-input.date {
		min-width: 180px;
	}

	.date-label {
		display: flex;
		align-items: center;
		gap: 5px;
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.clear-btn {
		height: 30px;
		padding: 0 12px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-danger) 45%, transparent);
		background: none;
		color: var(--color-danger);
		font-size: 0.8rem;
		font-family: inherit;
		cursor: pointer;
	}

	.clear-btn:hover {
		background-color: color-mix(in srgb, var(--color-danger) 10%, transparent);
	}

	.total-hint {
		font-size: 0.78rem;
		color: var(--color-text-muted);
		margin-left: auto;
	}

	/* ---- Table ---- */
	.content-area {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.table-wrap {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
		border-radius: 10px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 15%, transparent);
	}

	.table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.82rem;
	}

	.table thead {
		position: sticky;
		top: 0;
		z-index: 1;
		background-color: var(--color-bg-elevated);
	}

	.table th {
		text-align: left;
		padding: 8px 10px;
		font-size: 0.72rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
		white-space: nowrap;
	}

	.table td {
		padding: 7px 10px;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 8%, transparent);
		vertical-align: middle;
	}

	.table tbody tr:last-child td {
		border-bottom: none;
	}

	.table tbody tr:hover td {
		background-color: color-mix(in srgb, var(--color-accent) 5%, transparent);
	}

	.ts-cell {
		white-space: nowrap;
		color: var(--color-text-muted);
		font-size: 0.78rem;
	}

	.user-cell {
		white-space: nowrap;
		font-weight: 500;
	}

	.action-tag {
		display: inline-block;
		padding: 2px 7px;
		border-radius: 4px;
		font-size: 0.75rem;
		font-weight: 600;
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
		color: var(--color-accent);
		white-space: nowrap;
	}

	.action-tag.file {
		background-color: color-mix(in srgb, var(--color-info) 12%, transparent);
		color: var(--color-info);
	}
	.action-tag.tag {
		background-color: color-mix(in srgb, #7ecba1 12%, transparent);
		color: #7ecba1;
	}
	.action-tag.pool {
		background-color: color-mix(in srgb, var(--color-warning) 12%, transparent);
		color: var(--color-warning);
	}
	.action-tag.cat {
		background-color: color-mix(in srgb, var(--color-danger) 12%, transparent);
		color: var(--color-danger);
	}

	.obj-type-cell {
		color: var(--color-text-muted);
		text-transform: capitalize;
		font-size: 0.78rem;
	}

	.obj-id-cell {
		color: var(--color-text-muted);
		font-family: monospace;
		font-size: 0.78rem;
	}

	.loading-row td {
		text-align: center;
		padding: 16px;
	}

	.empty-cell {
		text-align: center;
		color: var(--color-text-muted);
		padding: 40px 0;
	}

	.spinner {
		display: inline-block;
		width: 22px;
		height: 22px;
		border: 2px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-top-color: var(--color-accent);
		border-radius: 50%;
		animation: spin 0.7s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.pagination {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 12px;
		flex-shrink: 0;
	}

	.page-btn {
		height: 32px;
		padding: 0 14px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 35%, transparent);
		background: none;
		color: var(--color-text-muted);
		font-size: 0.82rem;
		font-family: inherit;
		cursor: pointer;
	}

	.page-btn:hover:not(:disabled) {
		border-color: var(--color-accent);
		color: var(--color-accent);
	}

	.page-btn:disabled {
		opacity: 0.35;
		cursor: default;
	}

	.page-info {
		font-size: 0.82rem;
		color: var(--color-text-muted);
		min-width: 100px;
		text-align: center;
	}

	.msg.error {
		font-size: 0.85rem;
		color: var(--color-danger);
		margin: 0;
	}
</style>
