<script lang="ts">
	interface Props {
		onClose: () => void;
	}

	let { onClose }: Props = $props();

	// Static cheat-sheet of the app's shortcuts, grouped by context. Kept in sync
	// by hand with the per-context handlers (global nav here, the rest on the
	// Files page / viewer / tag pickers).
	const groups: { title: string; rows: [string, string][] }[] = [
		{
			title: 'Anywhere',
			rows: [
				['g then c / t / f / p / s', 'Go to Categories / Tags / Files / Pools / Settings'],
				['1 – 5', 'Jump to a section'],
				['?', 'Toggle this help'],
				['/', 'Focus the filter / search']
			]
		},
		{
			title: 'File grid',
			rows: [
				['↑ ↓ ← →', 'Move focus between files'],
				['Enter', 'Open the focused file'],
				['Space / x', 'Select / deselect'],
				['Shift+Space / Shift+x', 'Select a range from the anchor'],
				['e', 'Edit tags (focus the tag filter)'],
				['p', 'Add to pool'],
				['Del', 'Move to trash'],
				['Esc', 'Clear selection']
			]
		},
		{
			title: 'Viewer',
			rows: [
				['← / → or j / k', 'Previous / next file'],
				['e', 'Jump to tags & focus the filter'],
				['Esc', 'Close']
			]
		},
		{
			title: 'Tag editor / filter',
			rows: [
				['↓ ↑', 'Highlight a suggestion'],
				['Enter', 'Add the highlighted tag'],
				['← →', 'Move across added tags / tokens (empty input)'],
				['Del', 'Remove the focused tag / token'],
				['& | ! ( )', 'Insert an operator (filter only)'],
				['Ctrl+Enter', 'Apply the filter'],
				['Ctrl+Backspace', 'Reset the filter'],
				['Esc', 'Leave the field / close']
			]
		}
	];
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
<div class="backdrop" role="presentation" onclick={onClose}></div>
<div class="sheet" role="dialog" aria-label="Keyboard shortcuts" aria-modal="true">
	<div class="head">
		<span class="title">Keyboard shortcuts</span>
		<button class="close" onclick={onClose} aria-label="Close">
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
		{#each groups as group}
			<section class="group">
				<h3 class="group-title">{group.title}</h3>
				{#each group.rows as [keys, desc]}
					<div class="row">
						<kbd class="keys">{keys}</kbd>
						<span class="desc">{desc}</span>
					</div>
				{/each}
			</section>
		{/each}
	</div>
</div>

<style>
	.backdrop {
		position: fixed;
		inset: 0;
		z-index: 300;
		background: rgba(0, 0, 0, 0.5);
	}

	.sheet {
		position: fixed;
		left: 50%;
		top: 50%;
		transform: translate(-50%, -50%);
		z-index: 301;
		width: min(560px, calc(100vw - 24px));
		max-height: min(80dvh, 640px);
		display: flex;
		flex-direction: column;
		background-color: var(--color-bg-secondary);
		border-radius: 14px;
		box-shadow: 0 8px 40px rgba(0, 0, 0, 0.5);
		animation: pop 0.16s ease-out;
	}

	@keyframes pop {
		from {
			transform: translate(-50%, -48%) scale(0.98);
			opacity: 0;
		}
		to {
			transform: translate(-50%, -50%) scale(1);
			opacity: 1;
		}
	}

	.head {
		display: flex;
		align-items: center;
		padding: 14px 16px 10px;
	}

	.title {
		flex: 1;
		font-size: 1rem;
		font-weight: 600;
	}

	.close {
		background: none;
		border: none;
		cursor: pointer;
		color: var(--color-text-muted);
		padding: 4px;
		display: flex;
	}

	.close:hover {
		color: var(--color-text-primary);
	}

	.body {
		padding: 0 16px 18px;
		overflow-y: auto;
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
		gap: 6px 20px;
	}

	.group {
		break-inside: avoid;
		padding-top: 8px;
	}

	.group-title {
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.06em;
		color: var(--color-accent);
		margin: 0 0 6px;
	}

	.row {
		display: flex;
		align-items: baseline;
		gap: 10px;
		padding: 3px 0;
	}

	.keys {
		flex-shrink: 0;
		font-family: var(--font-sans);
		font-size: 0.72rem;
		color: var(--color-text-primary);
		background-color: var(--color-bg-elevated);
		border: 1px solid color-mix(in srgb, var(--color-accent) 25%, transparent);
		border-radius: 5px;
		padding: 2px 6px;
		white-space: nowrap;
	}

	.desc {
		font-size: 0.82rem;
		color: var(--color-text-muted);
	}
</style>
