<script lang="ts">
	import { api, ApiError } from '$lib/api/client';
	import { authStore } from '$lib/stores/auth';
	import { themeStore, toggleTheme } from '$lib/stores/theme';
	import { appSettings } from '$lib/stores/appSettings';
	import { resetPwa as doPwaReset } from '$lib/utils/pwa';
	import type { User, Session, SessionList } from '$lib/api/types';

	// ---- Profile ----
	let userName = $state($authStore.user?.name ?? '');
	let password = $state('');
	let passwordConfirm = $state('');
	let profileSaving = $state(false);
	let profileSuccess = $state(false);
	let profileError = $state('');

	async function saveProfile() {
		profileError = '';
		profileSuccess = false;
		if (!userName.trim()) {
			profileError = 'Name is required';
			return;
		}
		if (password && password !== passwordConfirm) {
			profileError = 'Passwords do not match';
			return;
		}
		profileSaving = true;
		try {
			const body: Record<string, string> = { name: userName.trim() };
			if (password) body.password = password;
			const updated = await api.patch<User>('/users/me', body);
			authStore.update((s) => ({
				...s,
				user: s.user ? { ...s.user, name: updated.name ?? s.user.name } : s.user,
			}));
			password = '';
			passwordConfirm = '';
			profileSuccess = true;
			setTimeout(() => (profileSuccess = false), 3000);
		} catch (e) {
			profileError = e instanceof ApiError ? e.message : 'Failed to save';
		} finally {
			profileSaving = false;
		}
	}

	// ---- Sessions ----
	let sessions = $state<Session[]>([]);
	let sessionsTotal = $state(0);
	let sessionsLoading = $state(true);
	let sessionsError = $state('');
	let terminatingIds = $state(new Set<number>());

	async function loadSessions() {
		sessionsLoading = true;
		sessionsError = '';
		try {
			const res = await api.get<SessionList>('/auth/sessions');
			sessions = res.items ?? [];
			sessionsTotal = res.total ?? sessions.length;
		} catch (e) {
			sessionsError = e instanceof ApiError ? e.message : 'Failed to load sessions';
		} finally {
			sessionsLoading = false;
		}
	}

	async function terminateSession(id: number) {
		terminatingIds = new Set([...terminatingIds, id]);
		try {
			await api.delete(`/auth/sessions/${id}`);
			sessions = sessions.filter((s) => s.id !== id);
			sessionsTotal = Math.max(0, sessionsTotal - 1);
		} catch {
			// silently ignore
		} finally {
			terminatingIds.delete(id);
			terminatingIds = new Set(terminatingIds);
		}
	}

	$effect(() => {
		void loadSessions();
	});

	// ---- PWA reset ----
	let pwaResetting = $state(false);
	let pwaSuccess = $state(false);

	async function resetPwa() {
		pwaResetting = true;
		pwaSuccess = false;
		try {
			await doPwaReset();
			pwaSuccess = true;
			setTimeout(() => (pwaSuccess = false), 3000);
		} finally {
			pwaResetting = false;
		}
	}

	// ---- Helpers ----
	function formatDate(iso: string | null | undefined): string {
		if (!iso) return '—';
		const d = new Date(iso);
		return d.toLocaleString(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
		});
	}

	function shortUserAgent(ua: string | null | undefined): string {
		if (!ua) return 'Unknown';
		// Extract browser + OS from UA string
		const browser =
			ua.match(/\b(Chrome|Firefox|Safari|Edge|Opera|Brave)\/[\d.]+/)?.[0] ??
			ua.match(/\b(MSIE|Trident)\b/)?.[0] ??
			ua.slice(0, 40);
		const os =
			ua.match(/\((Windows[^;)]*|Mac OS X [^;)]*|Linux[^;)]*|Android [^;)]*|iOS [^;)]*)/)?.[1] ?? '';
		return os ? `${browser} · ${os}` : browser;
	}
</script>

<svelte:head>
	<title>Settings | Tanabata</title>
</svelte:head>

<div class="page">
	<!-- ====== Profile ====== -->
	<section class="card">
		<h2 class="section-title">Profile</h2>

		{#if profileError}
			<p class="msg error" role="alert">{profileError}</p>
		{/if}
		{#if profileSuccess}
			<p class="msg success" role="status">Saved.</p>
		{/if}

		<div class="field">
			<label class="label" for="username">Username</label>
			<input
				id="username"
				class="input"
				type="text"
				bind:value={userName}
				required
				autocomplete="username"
				placeholder="Your display name"
			/>
		</div>

		<div class="field">
			<label class="label" for="password">New password</label>
			<input
				id="password"
				class="input"
				type="password"
				bind:value={password}
				autocomplete="new-password"
				placeholder="Leave blank to keep current"
			/>
		</div>

		{#if password}
			<div class="field">
				<label class="label" for="password-confirm">Confirm password</label>
				<input
					id="password-confirm"
					class="input"
					type="password"
					bind:value={passwordConfirm}
					autocomplete="new-password"
					placeholder="Repeat new password"
				/>
			</div>
		{/if}

		<div class="row-actions">
			<button
				class="btn primary"
				onclick={saveProfile}
				disabled={profileSaving || !userName.trim()}
			>
				{profileSaving ? 'Saving…' : 'Save changes'}
			</button>
		</div>
	</section>

	<!-- ====== Appearance ====== -->
	<section class="card">
		<h2 class="section-title">Appearance</h2>
		<div class="toggle-row">
			<span class="toggle-label">
				{$themeStore === 'light' ? 'Light theme' : 'Dark theme'}
			</span>
			<button
				class="theme-toggle"
				onclick={toggleTheme}
				title="Toggle theme"
				aria-label="Toggle theme"
			>
				{#if $themeStore === 'light'}
					<!-- Sun icon -->
					<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
						<circle cx="9" cy="9" r="3.5" stroke="currentColor" stroke-width="1.5"/>
						<path d="M9 1v2M9 15v2M1 9h2M15 9h2M3.22 3.22l1.41 1.41M13.36 13.36l1.42 1.42M3.22 14.78l1.41-1.41M13.36 4.64l1.42-1.42" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
					</svg>
					Switch to dark
				{:else}
					<!-- Moon icon -->
					<svg width="18" height="18" viewBox="0 0 18 18" fill="none" aria-hidden="true">
						<path d="M15 11.5A7 7 0 0 1 6.5 3a7.001 7.001 0 1 0 8.5 8.5z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
					</svg>
					Switch to light
				{/if}
			</button>
		</div>
	</section>

	<!-- ====== PWA ====== -->
	<section class="card">
		<h2 class="section-title">App cache</h2>
		<p class="hint-text">Clear service worker and cached assets. Useful if the app feels stale after an update.</p>
		{#if pwaSuccess}
			<p class="msg success" role="status">Cache cleared. Reload the page to fetch fresh assets.</p>
		{/if}
		<div class="row-actions">
			<button class="btn danger-outline" onclick={resetPwa} disabled={pwaResetting}>
				{pwaResetting ? 'Clearing…' : 'Clear PWA cache'}
			</button>
		</div>
	</section>

	<!-- ====== App settings ====== -->
	<section class="card">
		<h2 class="section-title">Behaviour</h2>

		<div class="field">
			<label class="label" for="file-limit">Files per page</label>
			<p class="hint-text">How many files to load in one batch when scrolling the file list.</p>
			<input
				id="file-limit"
				class="input input-narrow"
				type="number"
				min="10"
				max="500"
				step="1"
				value={$appSettings.fileLoadLimit}
				oninput={(e) => {
					const v = parseInt((e.currentTarget as HTMLInputElement).value, 10);
					if (!isNaN(v) && v >= 10 && v <= 500)
						appSettings.update((s) => ({ ...s, fileLoadLimit: v }));
				}}
			/>
		</div>

		<div class="toggle-row">
			<div>
				<span class="toggle-label">Apply new tag rules to existing files</span>
				<p class="hint-text">When a tag rule is created or activated, automatically add the implied tag to all files that already have the source tag.</p>
			</div>
			<button
				class="toggle"
				class:on={$appSettings.tagRuleApplyToExisting}
				role="switch"
				aria-checked={$appSettings.tagRuleApplyToExisting}
				aria-label="Apply activated tag rules to existing files"
				onclick={() => appSettings.update((s) => ({ ...s, tagRuleApplyToExisting: !s.tagRuleApplyToExisting }))}
			>
				<span class="thumb"></span>
			</button>
		</div>
	</section>

	<!-- ====== Sessions ====== -->
	<section class="card">
		<h2 class="section-title">
			Active sessions
			{#if sessionsTotal > 0}<span class="count">({sessionsTotal})</span>{/if}
		</h2>

		{#if sessionsError}
			<p class="msg error" role="alert">{sessionsError}</p>
		{:else if sessionsLoading}
			<p class="msg muted">Loading…</p>
		{:else if sessions.length === 0}
			<p class="msg muted">No active sessions.</p>
		{:else}
			<ul class="sessions-list">
				{#each sessions as session (session.id)}
					<li class="session-item" class:current={session.is_current}>
						<div class="session-info">
							<span class="session-ua">{shortUserAgent(session.user_agent)}</span>
							{#if session.is_current}
								<span class="current-badge">current</span>
							{/if}
							<span class="session-meta">
								Started {formatDate(session.started_at)}
								{#if session.expires_at}· Expires {formatDate(session.expires_at)}{/if}
							</span>
						</div>
						{#if !session.is_current}
							<button
								class="terminate-btn"
								onclick={() => session.id != null && terminateSession(session.id)}
								disabled={terminatingIds.has(session.id ?? -1)}
								aria-label="Terminate session"
							>
								{terminatingIds.has(session.id ?? -1) ? '…' : 'End'}
							</button>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</div>

<style>
	.page {
		flex: 1;
		overflow-y: auto;
		padding: 16px 12px calc(70px + 16px);
		display: flex;
		flex-direction: column;
		gap: 14px;
		max-width: 600px;
		margin: 0 auto;
		width: 100%;
		box-sizing: border-box;
	}

	.card {
		background-color: var(--color-bg-elevated);
		border-radius: 12px;
		padding: 16px;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.section-title {
		font-size: 0.9rem;
		font-weight: 700;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.07em;
		margin: 0;
	}

	.count {
		font-weight: 400;
		text-transform: none;
		letter-spacing: 0;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 5px;
	}

	.label {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}

	.input {
		height: 36px;
		padding: 0 10px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 30%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.9rem;
		font-family: inherit;
		outline: none;
	}

	.input:focus {
		border-color: var(--color-accent);
	}

	.row-actions {
		display: flex;
		gap: 8px;
	}

	.btn {
		height: 34px;
		padding: 0 16px;
		border-radius: 7px;
		border: none;
		font-size: 0.875rem;
		font-family: inherit;
		font-weight: 600;
		cursor: pointer;
	}

	.btn:disabled {
		opacity: 0.5;
		cursor: default;
	}

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
		padding: 6px 0;
	}

	.msg.error { color: var(--color-danger); }
	.msg.success { color: #7ECBA1; }
	.msg.muted { color: var(--color-text-muted); }

	/* ---- Appearance toggle ---- */
	.toggle-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.toggle-label {
		font-size: 0.9rem;
	}

	.theme-toggle {
		display: inline-flex;
		align-items: center;
		gap: 7px;
		height: 34px;
		padding: 0 14px;
		border-radius: 7px;
		border: 1px solid color-mix(in srgb, var(--color-accent) 35%, transparent);
		background-color: var(--color-bg-primary);
		color: var(--color-text-primary);
		font-size: 0.85rem;
		font-family: inherit;
		cursor: pointer;
		white-space: nowrap;
	}

	.theme-toggle:hover {
		border-color: var(--color-accent);
		color: var(--color-accent);
	}

	.input-narrow {
		max-width: 100px;
	}

	/* On/off toggle switch */
	.toggle {
		flex-shrink: 0;
		position: relative;
		width: 42px;
		height: 24px;
		border-radius: 12px;
		border: none;
		background-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-bg-primary));
		cursor: pointer;
		padding: 0;
		transition: background-color 0.15s;
	}

	.toggle.on {
		background-color: var(--color-accent);
	}

	.toggle .thumb {
		position: absolute;
		top: 3px;
		left: 3px;
		width: 18px;
		height: 18px;
		border-radius: 50%;
		background-color: #fff;
		transition: transform 0.15s;
	}

	.toggle.on .thumb {
		transform: translateX(18px);
	}

	/* ---- PWA ---- */
	.hint-text {
		font-size: 0.82rem;
		color: var(--color-text-muted);
		margin: 0;
		line-height: 1.5;
	}

	/* ---- Sessions ---- */
	.sessions-list {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: 2px;
	}

	.session-item {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 0;
		border-bottom: 1px solid color-mix(in srgb, var(--color-accent) 12%, transparent);
	}

	.session-item:last-child {
		border-bottom: none;
		padding-bottom: 0;
	}

	.session-item.current {
		background: none;
	}

	.session-info {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 3px;
		min-width: 0;
	}

	.session-ua {
		font-size: 0.875rem;
		color: var(--color-text-primary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.current-badge {
		display: inline-block;
		font-size: 0.7rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-accent);
		background-color: color-mix(in srgb, var(--color-accent) 15%, transparent);
		border-radius: 4px;
		padding: 1px 6px;
		width: fit-content;
	}

	.session-meta {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.terminate-btn {
		flex-shrink: 0;
		height: 28px;
		padding: 0 12px;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-danger) 50%, transparent);
		background: none;
		color: var(--color-danger);
		font-size: 0.8rem;
		font-family: inherit;
		cursor: pointer;
	}

	.terminate-btn:hover:not(:disabled) {
		background-color: color-mix(in srgb, var(--color-danger) 12%, transparent);
	}

	.terminate-btn:disabled {
		opacity: 0.45;
		cursor: default;
	}
</style>