<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';

	let { children } = $props();

	const tabs = [
		{ href: '/admin/users', label: 'Users' },
		{ href: '/admin/audit', label: 'Audit log' }
	];
</script>

<div class="admin-shell">
	<nav class="admin-nav">
		<button class="back-btn" onclick={() => goto('/files')} aria-label="Back to files">
			<svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
				<path
					d="M10 3L5 8L10 13"
					stroke="currentColor"
					stroke-width="1.8"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</button>
		<span class="admin-title">Admin</span>
		<div class="tabs">
			{#each tabs as tab}
				<a href={tab.href} class="tab" class:active={$page.url.pathname.startsWith(tab.href)}
					>{tab.label}</a
				>
			{/each}
		</div>
	</nav>
	<div class="admin-content">
		{@render children()}
	</div>
</div>

<style>
	.admin-shell {
		flex: 1;
		display: flex;
		flex-direction: column;
		min-height: 0;
	}

	.admin-nav {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 8px 14px;
		background-color: var(--color-bg-elevated);
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 20%, transparent);
		flex-shrink: 0;
	}

	.back-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		background: none;
		color: var(--color-text-muted);
		cursor: pointer;
		flex-shrink: 0;
	}

	.back-btn:hover {
		color: var(--color-text-primary);
		border-color: var(--color-accent);
	}

	.admin-title {
		font-size: 0.8rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: var(--color-text-muted);
		flex-shrink: 0;
	}

	.tabs {
		display: flex;
		gap: 2px;
		margin-left: 8px;
	}

	.tab {
		height: 28px;
		padding: 0 12px;
		border-radius: 6px;
		font-size: 0.85rem;
		font-weight: 500;
		color: var(--color-text-muted);
		text-decoration: none;
		display: flex;
		align-items: center;
	}

	.tab:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
		color: var(--color-text-primary);
	}

	.tab.active {
		background-color: color-mix(in srgb, var(--color-accent) 20%, transparent);
		color: var(--color-accent);
	}

	.admin-content {
		flex: 1;
		min-height: 0;
		overflow-y: auto;
	}
</style>
