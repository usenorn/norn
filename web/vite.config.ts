import { sentrySvelteKit } from '@sentry/sveltekit';
import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, '.', 'SENTRY_');

	const sourceMapUpload =
		env.SENTRY_URL && env.SENTRY_ORG && env.SENTRY_PROJECT
			? [
					sentrySvelteKit({
						org: env.SENTRY_ORG,
						project: env.SENTRY_PROJECT,
						sentryUrl: env.SENTRY_URL
					})
				]
			: [];

	return {
		ssr: {
			noExternal: ['morphicons']
		},
		server: {
			port: 5174,
			strictPort: true,
			allowedHosts: true,
			proxy: {
				'/v1': {
					target: 'http://127.0.0.1:8080',
					changeOrigin: true,
					proxyTimeout: 0,
					timeout: 0
				},
				'/mcp': {
					target: 'http://127.0.0.1:8080',
					changeOrigin: true
				},
				'/oauth': {
					target: 'http://127.0.0.1:8080',
					changeOrigin: true
				},
				'/.well-known': {
					target: 'http://127.0.0.1:8080',
					changeOrigin: true
				}
			}
		},
		plugins: [
			...sourceMapUpload,
			tailwindcss(),
			sveltekit({
				experimental: {
					instrumentation: {
						server: true
					}
				},
				compilerOptions: {
					// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
					runes: ({ filename }) => filename.split(/[/\\]/).includes('node_modules') ? undefined : true
				},
				adapter: adapter()
			})
		]
	};
});
