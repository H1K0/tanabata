// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// interface Error {}
		// interface Locals {}
		// interface PageData {}
		interface PageState {
			/** Set via shallow routing when the file viewer is open as an overlay
			 *  on top of the files list. */
			fileId?: string;
		}
		// interface Platform {}
	}
}

export {};
