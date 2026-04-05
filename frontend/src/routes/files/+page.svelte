<script lang="ts">
	import { api } from '$lib/api/client';
	import { ApiError } from '$lib/api/client';
	import FileCard from '$lib/components/file/FileCard.svelte';
	import InfiniteScroll from '$lib/components/common/InfiniteScroll.svelte';
	import type { File, FileCursorPage } from '$lib/api/types';

	const LIMIT = 50;

	let files = $state<File[]>([]);
	let nextCursor = $state<string | null>(null);
	let loading = $state(false);
	let hasMore = $state(true);
	let error = $state('');

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		error = '';

		try {
			const params = new URLSearchParams({ limit: String(LIMIT) });
			if (nextCursor) params.set('cursor', nextCursor);

			const page = await api.get<FileCursorPage>(`/files?${params}`);
			files = [...files, ...(page.items ?? [])];
			nextCursor = page.next_cursor ?? null;
			hasMore = !!page.next_cursor;
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'Failed to load files';
			hasMore = false;
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Files | Tanabata</title>
</svelte:head>

<div class="page">
	<main>
		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="grid">
			{#each files as file (file.id)}
				<FileCard {file} />
			{/each}
		</div>

		<InfiniteScroll {loading} {hasMore} onLoadMore={loadMore} />

		{#if !loading && !hasMore && files.length === 0}
			<div class="empty">No files yet.</div>
		{/if}
	</main>
</div>

<style>
	.page {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}

	main {
		flex: 1;
		overflow-y: auto;
		padding: 10px 10px calc(60px + 10px); /* clear fixed navbar */
	}

	.grid {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-content: flex-start;
		align-items: flex-start;
		gap: 2px;
	}

	/* phantom last item so justify-content doesn't stretch final row */
	.grid::after {
		content: '';
		flex: auto;
	}

	.error {
		color: var(--color-danger);
		padding: 12px;
		font-size: 0.875rem;
	}

	.empty {
		text-align: center;
		color: var(--color-text-muted);
		padding: 60px 20px;
		font-size: 0.95rem;
	}
</style>