import type { Client } from "openapi-fetch";
import type { components, paths } from "$lib/api/dashboard.gen";

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

export async function reachInstance(api: Client<paths>, url: URL): Promise<Instance> {
	try {
		const { data } = await api.GET("/instance");

		return data ?? fallbackInstance(url);
	} catch {
		return fallbackInstance(url);
	}
}
