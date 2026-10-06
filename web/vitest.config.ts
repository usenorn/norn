import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [svelte()],
	resolve: {
		alias: {
			$lib: new URL('./src/lib', import.meta.url).pathname,
			'$app/navigation': new URL('./test/app-navigation.ts', import.meta.url).pathname,
			'$app/state': new URL('./test/app-state.ts', import.meta.url).pathname
		},
		conditions: ['browser']
	},
	test: {
		environment: 'jsdom',
		include: ['src/**/*.test.ts'],
		restoreMocks: true
	}
});
