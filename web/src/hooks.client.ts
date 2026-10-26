import type { HandleClientError } from "@sveltejs/kit/hooks";
import { startEarlyKeys } from "#lib/shortcuts/early-keys.js";

startEarlyKeys();

export const handleError: HandleClientError = (caught) => {
	if (caught.kind === "framework" && caught.error.status === 404) return { code: "not_found" };
	if (caught.kind !== "unknown") return;

	console.error(caught.error);

	return { message: "Something went wrong.", code: "unexpected" };
};
