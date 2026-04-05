import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';
import { mockApiPlugin } from './vite-mock-plugin';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit(), mockApiPlugin()]
});
