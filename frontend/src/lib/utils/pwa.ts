/**
 * Unregisters all service workers and clears all caches, then reloads.
 * Use this when the app feels stale or to force a clean re-fetch of all assets.
 */
export async function resetPwa(): Promise<void> {
	if ('serviceWorker' in navigator) {
		const registrations = await navigator.serviceWorker.getRegistrations();
		await Promise.all(registrations.map((r) => r.unregister()));
	}
	if ('caches' in window) {
		const keys = await caches.keys();
		await Promise.all(keys.map((k) => caches.delete(k)));
	}
}