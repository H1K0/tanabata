<script lang="ts">
	import { goto } from '$app/navigation';
	import { login } from '$lib/api/auth';
	import { api } from '$lib/api/client';
	import { ApiError } from '$lib/api/client';
	import { authStore } from '$lib/stores/auth';
	import type { User } from '$lib/api/types';

	let name = $state('');
	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		loading = true;
		try {
			await login(name, password);
			const me = await api.get<User>('/users/me');
			authStore.update((s) => ({
				...s,
				user: {
					id: me.id!,
					name: me.name!,
					isAdmin: me.is_admin ?? false,
				},
			}));
			await goto('/files');
		} catch (err) {
			if (err instanceof ApiError && err.status === 401) {
				error = 'Invalid username or password.';
			} else if (err instanceof Error) {
				error = err.message;
			} else {
				error = 'An unexpected error occurred.';
			}
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Welcome to Tanabata File Manager!</title>
</svelte:head>

<div class="login-root">
	<img src="/images/tanabata-left.png" alt="" class="decoration left" aria-hidden="true" />
	<img src="/images/tanabata-right.png" alt="" class="decoration right" aria-hidden="true" />

	<form onsubmit={handleSubmit} novalidate>
		<h1>Welcome to<br />Tanabata File Manager!</h1>

		{#if error}
			<p class="error" role="alert">{error}</p>
		{/if}

		<div class="field">
			<input
				type="text"
				name="username"
				placeholder="Username..."
				autocomplete="username"
				required
				disabled={loading}
				bind:value={name}
			/>
		</div>
		<div class="field">
			<input
				type="password"
				name="password"
				placeholder="Password..."
				autocomplete="current-password"
				required
				disabled={loading}
				bind:value={password}
			/>
		</div>
		<div class="field">
			<button type="submit" disabled={loading || !name || !password}>
				{loading ? 'Logging in…' : 'Log in'}
			</button>
		</div>
	</form>
</div>

<style>
	.login-root {
		position: fixed;
		inset: 0;
		background-color: var(--color-bg-primary);
		display: flex;
		align-items: center;
		justify-content: center;
		font-family: var(--font-sans);
		overflow: hidden;
	}

	.decoration {
		position: absolute;
		top: 0;
		width: 20vw;
		pointer-events: none;
		user-select: none;
	}

	.decoration.left  { left: 0; }
	.decoration.right { right: 0; }

	form {
		position: relative;
		z-index: 1;
		width: min(380px, calc(100vw - 48px));
		display: flex;
		flex-direction: column;
		gap: 0;
	}

	h1 {
		color: var(--color-text-primary);
		font-size: 1.75rem;
		font-weight: 700;
		line-height: 1.25;
		margin: 0 0 28px;
		text-align: center;
	}

	.error {
		background-color: color-mix(in srgb, var(--color-danger) 20%, transparent);
		border: 1px solid var(--color-danger);
		border-radius: 10px;
		color: var(--color-danger);
		font-size: 0.875rem;
		margin-bottom: 12px;
		padding: 10px 14px;
		text-align: center;
	}

	.field {
		margin-top: 14px;
	}

	input {
		background-color: var(--color-bg-elevated);
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		border-radius: 14px;
		color: var(--color-text-primary);
		font-family: inherit;
		font-size: 1rem;
		height: 52px;
		outline: none;
		padding: 0 16px;
		transition: border-color 0.15s;
		width: 100%;
		box-sizing: border-box;
	}

	input::placeholder { color: var(--color-text-muted); }

	input:focus {
		border-color: var(--color-accent);
	}

	input:disabled {
		opacity: 0.5;
	}

	button {
		background-color: var(--color-accent);
		border: 1px solid #454261;
		border-radius: 14px;
		color: var(--color-text-primary);
		cursor: pointer;
		font-family: inherit;
		font-size: 1.25rem;
		font-weight: 500;
		height: 50px;
		margin-top: 20px;
		transition: background-color 0.15s;
		width: 100%;
	}

	button:hover:not(:disabled) {
		background-color: var(--color-accent-hover);
	}

	button:disabled {
		cursor: not-allowed;
		opacity: 0.5;
	}
</style>