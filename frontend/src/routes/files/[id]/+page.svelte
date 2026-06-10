<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { get } from 'svelte/store';
	import { api } from '$lib/api/client';
	import { fileSorting } from '$lib/stores/sorting';
	import FileViewer from '$lib/components/file/FileViewer.svelte';
	import type { FileCursorPage } from '$lib/api/types';

	// This standalone route is the fallback for a deep link / hard reload to a
	// file. The normal path (opening from the grid) renders FileViewer as an
	// overlay on the still-mounted list via shallow routing — see files/+page.
	let fileId = $derived(page.params.id);

	let prevId = $state<string | null>(null);
	let nextId = $state<string | null>(null);

	$effect(() => {
		const id = fileId;
		if (id) void resolveNeighbors(id);
	});

	// No cached grid here, so derive neighbours from an anchored window. The
	// backend anchor window is forward-inclusive, so prev is only available once
	// we're past the first item of that window.
	async function resolveNeighbors(id: string) {
		const sort = get(fileSorting);
		const params = new URLSearchParams({
			anchor: id,
			limit: '3',
			sort: sort.sort,
			order: sort.order,
		});
		try {
			const result = await api.get<FileCursorPage>(`/files?${params}`);
			if (fileId !== id) return;
			const items = result.items ?? [];
			const idx = items.findIndex((f) => f.id === id);
			prevId = idx > 0 ? (items[idx - 1].id ?? null) : null;
			nextId = idx >= 0 && idx < items.length - 1 ? (items[idx + 1].id ?? null) : null;
		} catch {
			// non-critical
		}
	}

	function pageTo(id: string) {
		goto(`/files/${id}`);
	}

	function closeViewer() {
		// No list mounted underneath — go to the grid, carrying the file as an
		// anchor so it scrolls into view there.
		const id = fileId;
		goto('/files' + (id ? `?anchor=${id}` : ''), { noScroll: true });
	}
</script>

<svelte:head>
	<title>{fileId} | Tanabata</title>
</svelte:head>

{#if fileId}
	<FileViewer {fileId} {prevId} {nextId} onNavigate={pageTo} onClose={closeViewer} />
{/if}
