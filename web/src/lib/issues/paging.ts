import type { Issue } from "./issues";

export type ColumnPaging = { kind: "idle" } | { kind: "loading" } | { kind: "unavailable" };

export type ColumnPage = {
	issues: Issue[];
	cursor: string | undefined;
	paging: ColumnPaging;
};

export type BoardPages = {
	source: string;
	columns: Record<string, ColumnPage>;
};

export type ColumnLoad =
	| { kind: "complete" }
	| { kind: "more"; remaining: number; cursor: string | undefined }
	| { kind: "loading" }
	| { kind: "unavailable"; remaining: number; cursor: string | undefined };

export const noPages: BoardPages = { source: "", columns: {} };

export function pagesOf(held: BoardPages, source: string): Record<string, ColumnPage> {
	return held.source === source ? held.columns : {};
}

export function pageOf(
	pages: Record<string, ColumnPage>,
	key: string
): ColumnPage | undefined {
	return pages[key];
}

function written(
	held: BoardPages,
	source: string,
	key: string,
	page: (previous: ColumnPage) => ColumnPage
): BoardPages {
	const columns = pagesOf(held, source);
	const previous = columns[key] ?? { issues: [], cursor: undefined, paging: { kind: "idle" } };

	return { source, columns: { ...columns, [key]: page(previous) } };
}

export function withLoading(held: BoardPages, source: string, key: string): BoardPages {
	return written(held, source, key, (previous) => ({ ...previous, paging: { kind: "loading" } }));
}

export function withPage(
	held: BoardPages,
	source: string,
	key: string,
	issues: Issue[],
	cursor: string | undefined
): BoardPages {
	return written(held, source, key, (previous) => ({
		issues: [...previous.issues, ...issues],
		cursor,
		paging: { kind: "idle" },
	}));
}

export function withFailure(held: BoardPages, source: string, key: string): BoardPages {
	return written(held, source, key, (previous) => ({
		...previous,
		paging: { kind: "unavailable" },
	}));
}

export function loadFor(
	loaded: number,
	total: number,
	cursor: string | undefined,
	paging: ColumnPaging
): ColumnLoad {
	if (paging.kind === "loading") return { kind: "loading" };
	if (loaded >= total) return { kind: "complete" };

	const remaining = total - loaded;

	return paging.kind === "unavailable"
		? { kind: "unavailable", remaining, cursor }
		: { kind: "more", remaining, cursor };
}
