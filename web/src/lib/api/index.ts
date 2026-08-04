import createClient from "openapi-fetch";
import type { paths } from "./dashboard.gen";

export const api = createClient<paths>({ baseUrl: "/v1" });

export function apiFor(url: URL) {
	return createClient<paths>({ baseUrl: `${url.origin}/v1` });
}
