import type { Handle } from "@sveltejs/kit";

const serialized = new Set(["content-length", "content-type"]);

export const handle: Handle = ({ event, resolve }) =>
	resolve(event, {
		filterSerializedResponseHeaders: (name) => serialized.has(name),
	});
