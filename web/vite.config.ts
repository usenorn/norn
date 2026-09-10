import { sentrySvelteKit } from '@sentry/sveltekit';
import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-node';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, '.', ['SENTRY_', 'NORN_']);

	// Two branches checked out side by side each need their own backend, and a hardcoded port
	// sends one of them at the other's server, where every write is refused as cross-site.
	const api = env.NORN_DEV_API_ORIGIN ?? 'http://127.0.0.1:8080';
	const port = Number(env.NORN_DEV_PORT ?? 5174);

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
			port,
			strictPort: true,
			allowedHosts: true,
			proxy: {
				'/v1': {
					target: api,
					changeOrigin: true,
					proxyTimeout: 0,
					timeout: 0
				},
				'/mcp': {
					target: api,
					changeOrigin: true
				},
				'/oauth': {
					target: api,
					changeOrigin: true
				},
				'/.well-known': {
					target: api,
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
