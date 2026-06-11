import { writable, derived } from 'svelte/store';

interface SelectionState {
	active: boolean;
	ids: Set<string>;
}

function createSelectionStore() {
	const { subscribe, update, set } = writable<SelectionState>({
		active: false,
		ids: new Set()
	});

	return {
		subscribe,

		enter() {
			update((s) => ({ ...s, active: true }));
		},

		exit() {
			set({ active: false, ids: new Set() });
		},

		toggle(id: string) {
			update((s) => {
				const ids = new Set(s.ids);
				if (ids.has(id)) {
					ids.delete(id);
				} else {
					ids.add(id);
				}
				// Exit selection mode automatically when last item is deselected
				const active = ids.size > 0;
				return { active, ids };
			});
		},

		select(id: string) {
			update((s) => {
				const ids = new Set(s.ids);
				ids.add(id);
				return { active: true, ids };
			});
		},

		deselect(id: string) {
			update((s) => {
				const ids = new Set(s.ids);
				ids.delete(id);
				const active = ids.size > 0;
				return { active, ids };
			});
		},

		clear() {
			set({ active: false, ids: new Set() });
		}
	};
}

export const selectionStore = createSelectionStore();

export const selectionCount = derived(selectionStore, ($s) => $s.ids.size);
export const selectionActive = derived(selectionStore, ($s) => $s.active);
