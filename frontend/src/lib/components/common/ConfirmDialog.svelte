<script lang="ts">
	interface Props {
		message: string;
		confirmLabel?: string;
		danger?: boolean;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let { message, confirmLabel = 'Confirm', danger = false, onConfirm, onCancel }: Props = $props();

	let dialog = $state<HTMLDialogElement | undefined>();

	$effect(() => {
		dialog?.showModal();
		return () => dialog?.close();
	});

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onCancel();
	}

	function handleBackdropClick(e: MouseEvent) {
		if (e.target === dialog) onCancel();
	}
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<dialog
	bind:this={dialog}
	onkeydown={handleKeydown}
	onclick={handleBackdropClick}
	aria-modal="true"
>
	<div class="body">
		<p class="message">{message}</p>
		<div class="actions">
			<button class="btn cancel" onclick={onCancel}>Cancel</button>
			<button class="btn confirm" class:danger onclick={onConfirm}>{confirmLabel}</button>
		</div>
	</div>
</dialog>

<style>
	dialog {
		padding: 0;
		border: none;
		border-radius: 12px;
		background-color: var(--color-bg-secondary);
		color: var(--color-text-primary);
		box-shadow: 0 8px 32px rgba(0, 0, 0, 0.6);
		max-width: min(340px, calc(100vw - 32px));
		width: 100%;
		position: fixed;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		margin: 0;
	}

	dialog::backdrop {
		background-color: rgba(0, 0, 0, 0.55);
		backdrop-filter: blur(2px);
	}

	.body {
		padding: 20px 20px 16px;
		display: flex;
		flex-direction: column;
		gap: 18px;
	}

	.message {
		margin: 0;
		font-size: 0.9rem;
		line-height: 1.5;
		color: var(--color-text-primary);
	}

	.actions {
		display: flex;
		gap: 8px;
		justify-content: flex-end;
	}

	.btn {
		height: 36px;
		padding: 0 16px;
		border-radius: 8px;
		font-size: 0.875rem;
		font-weight: 600;
		font-family: inherit;
		cursor: pointer;
	}

	.btn.cancel {
		border: 1px solid color-mix(in srgb, var(--color-accent) 35%, transparent);
		background: none;
		color: var(--color-text-primary);
	}

	.btn.cancel:hover {
		background-color: color-mix(in srgb, var(--color-accent) 12%, transparent);
	}

	.btn.confirm {
		border: none;
		background-color: var(--color-accent);
		color: var(--color-bg-primary);
	}

	.btn.confirm:hover {
		background-color: var(--color-accent-hover);
	}

	.btn.confirm.danger {
		background-color: var(--color-danger);
	}

	.btn.confirm.danger:hover {
		background-color: color-mix(in srgb, var(--color-danger) 80%, #fff);
	}
</style>