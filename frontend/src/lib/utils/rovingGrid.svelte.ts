// Reusable roving keyboard-focus for a wrap/grid of items navigated by id —
// the same model the Files grid uses (arrows move a focus ring, Enter opens,
// "/" jumps to search, Escape drops the ring), generalised so the tag and
// category lists can share it.
//
// Unlike the Files grid (fixed 160px cards → computable columns), tags and
// categories are variable-width pills that wrap, so vertical movement is
// geometric: pick the nearest item in the target row by horizontal centre.

interface Item {
	id?: string;
}

interface RovingGridOptions<T extends Item> {
	/** Reactive getter for the current item list (in render order). */
	items: () => T[];
	/** The scroll container holding the item elements. */
	container: () => HTMLElement | undefined;
	/** Open / activate the focused item (Enter). */
	onOpen: (item: T) => void;
	/** Focus the page's search box ("/"). Optional. */
	focusSearch?: () => void;
	/** CSS selector for the focusable item elements within the container. */
	itemSelector?: string;
}

export function createRovingGrid<T extends Item>(opts: RovingGridOptions<T>) {
	const selector = opts.itemSelector ?? '[data-item-index]';

	let focusedId = $state<string | null>(null);
	// Gate the focus ring so it only shows once the user navigates by keyboard.
	let kbActive = $state(false);

	// Drop the focus if its item leaves the loaded/filtered list.
	$effect(() => {
		const list = opts.items();
		if (focusedId && !list.some((it) => it.id === focusedId)) {
			focusedId = null;
		}
	});

	function isFormTarget(t: EventTarget | null): boolean {
		return (
			t instanceof HTMLElement &&
			(t.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT', 'BUTTON', 'A'].includes(t.tagName))
		);
	}

	function els(): HTMLElement[] {
		const root = opts.container();
		return root ? [...root.querySelectorAll<HTMLElement>(selector)] : [];
	}

	function currentIndex(list: T[]): number {
		return focusedId ? list.findIndex((it) => it.id === focusedId) : -1;
	}

	function focusAt(idx: number, list: T[]) {
		const id = list[idx]?.id;
		if (!id) return;
		kbActive = true;
		focusedId = id;
		requestAnimationFrame(() => {
			els()[idx]?.scrollIntoView({ block: 'nearest' });
		});
	}

	// Horizontal step is index-based; vertical step is geometric (items wrap at
	// variable widths, so there's no fixed column count to add/subtract).
	function move(dir: 'left' | 'right' | 'up' | 'down') {
		const list = opts.items();
		if (list.length === 0) return;
		const cur = currentIndex(list);
		if (cur < 0) {
			focusAt(0, list);
			return;
		}
		if (dir === 'left') {
			focusAt(Math.max(0, cur - 1), list);
			return;
		}
		if (dir === 'right') {
			focusAt(Math.min(list.length - 1, cur + 1), list);
			return;
		}
		// up / down: nearest item in the target direction, preferring the closest
		// row, then the closest horizontal centre.
		const nodes = els();
		const curRect = nodes[cur]?.getBoundingClientRect();
		if (!curRect) return;
		const curMidX = curRect.left + curRect.width / 2;
		let best = -1;
		let bestScore = Infinity;
		for (let i = 0; i < nodes.length; i++) {
			if (i === cur) continue;
			const r = nodes[i].getBoundingClientRect();
			const wanted = dir === 'down' ? r.top > curRect.top + 1 : r.top < curRect.top - 1;
			if (!wanted) continue;
			const dy = Math.abs(r.top - curRect.top);
			const dx = Math.abs(r.left + r.width / 2 - curMidX);
			const score = dy * 100000 + dx; // row distance dominates, x breaks ties
			if (score < bestScore) {
				bestScore = score;
				best = i;
			}
		}
		if (best >= 0) focusAt(best, list);
	}

	function handleKey(e: KeyboardEvent) {
		if (e.metaKey || e.ctrlKey || e.altKey) return;

		if (e.key === 'Escape') {
			if (focusedId) {
				focusedId = null;
				kbActive = false;
			}
			return;
		}

		// "/" focuses the search box from anywhere outside a field.
		if (e.key === '/' && opts.focusSearch && !isFormTarget(e.target)) {
			e.preventDefault();
			opts.focusSearch();
			return;
		}

		if (isFormTarget(e.target)) return;

		switch (e.key) {
			case 'ArrowRight':
				e.preventDefault();
				move('right');
				return;
			case 'ArrowLeft':
				e.preventDefault();
				move('left');
				return;
			case 'ArrowDown':
				e.preventDefault();
				move('down');
				return;
			case 'ArrowUp':
				e.preventDefault();
				move('up');
				return;
			case 'Enter': {
				const list = opts.items();
				const cur = currentIndex(list);
				if (cur >= 0) {
					e.preventDefault();
					opts.onOpen(list[cur]);
				}
				return;
			}
		}
	}

	return {
		get focusedId() {
			return focusedId;
		},
		get kbActive() {
			return kbActive;
		},
		/** Clear the keyboard ring (e.g. on a pointer interaction). */
		clearKbActive() {
			kbActive = false;
		},
		handleKey
	};
}
