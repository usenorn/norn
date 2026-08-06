import { api } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";

export type Instance = components["schemas"]["Instance"];

export function fallbackInstance(url: URL): Instance {
	return {
		signupsOpen: true,
		password: true,
		breachCheck: false,
		selfHosted: false,
		host: url.host,
		version: "",
	};
}

export async function reachInstance(
	fetch: typeof globalThis.fetch,
	url: URL
): Promise<Instance> {
	try {
		const { data } = await api.GET("/instance", { fetch });

		return data ?? fallbackInstance(url);
	} catch {
		return fallbackInstance(url);
	}
}
