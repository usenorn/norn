import type { ApiResult } from "./attempt";

export type Listed<T> =
	| { kind: "loading" }
	| { kind: "ready"; rows: T[]; nextCursor?: string }
	| { kind: "empty" }
	| { kind: "no_matches" }
	| { kind: "unavailable" };

export type Page<T> = { rows: T[]; nextCursor?: string };

export function listed<T>(result: ApiResult<Page<T>> | undefined, filtered = false): Listed<T> {
	if (!result || result.error !== undefined || result.data === undefined) {
		return { kind: "unavailable" };
	}

	return held(result.data.rows, result.data.nextCursor, filtered);
}

export function taken<T>(rows: T[] | undefined, filtered = false): Listed<T> {
	return rows === undefined ? { kind: "unavailable" } : held(rows, undefined, filtered);
}

function held<T>(rows: T[], nextCursor: string | undefined, filtered: boolean): Listed<T> {
	if (rows.length > 0) return { kind: "ready", rows, nextCursor };

	return filtered ? { kind: "no_matches" } : { kind: "empty" };
}

export function rowsOf<T>(listing: Listed<T>): T[] {
	return listing.kind === "ready" ? listing.rows : [];
}

export function cursorOf<T>(listing: Listed<T>): string | undefined {
	return listing.kind === "ready" ? listing.nextCursor : undefined;
}

export function grew<T>(listing: Listed<T>, page: Page<T>): Listed<T> {
	if (listing.kind !== "ready") return listed({ data: page });

	return { kind: "ready", rows: [...listing.rows, ...page.rows], nextCursor: page.nextCursor };
}

export const moreFailedLine = "We could not load any more. Nothing changed — try again.";
