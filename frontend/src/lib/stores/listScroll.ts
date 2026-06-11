// Reapply a restored scroll offset to a list's scroller, retrying across frames
// because the list may not be laid out yet right after a cache rehydrate (and
// SvelteKit resets scroll to the top on navigation, so this has to win after).
export function restoreListScroll(getEl: () => HTMLElement | undefined, top: number): void {
	let tries = 12;
	const apply = () => {
		const el = getEl();
		if (!el) {
			if (tries-- > 0) requestAnimationFrame(apply);
			return;
		}
		if (el.scrollHeight > top + el.clientHeight || tries-- <= 0) {
			el.scrollTop = top;
			return;
		}
		requestAnimationFrame(apply);
	};
	requestAnimationFrame(apply);
}
