import type { components } from "$lib/api/dashboard.gen";

export type SearchResults = components["schemas"]["SearchResults"];
export type SearchGroup = components["schemas"]["SearchGroup"];
export type SearchResult = components["schemas"]["SearchResult"];
export type SearchKind = components["schemas"]["SearchKind"];

export type SearchListing =
	| { kind: "idle" }
	| { kind: "searching" }
	| { kind: "no_matches"; fuzzy: boolean }
	| { kind: "results"; groups: SearchGroup[]; fuzzy: boolean }
	| { kind: "unavailable" };

export const searchDebounceMs = 200;

export const kindLabels: Record<SearchKind, string> = {
	issue: "Issues",
	comment: "Comments",
	project: "Projects",
	team: "Teams",
	person: "People",
};

export const kindOrder: SearchKind[] = ["issue", "comment", "project", "team", "person"];

export function listingFor(results: SearchResults | undefined): SearchListing {
	if (!results) return { kind: "unavailable" };

	const groups = results.groups.filter((group) => group.results.length > 0);

	if (groups.length === 0) return { kind: "no_matches", fuzzy: results.fuzzy };

	return { kind: "results", groups: ordered(groups), fuzzy: results.fuzzy };
}

function ordered(groups: SearchGroup[]): SearchGroup[] {
	return [...groups].sort((a, b) => kindOrder.indexOf(a.kind) - kindOrder.indexOf(b.kind));
}

export function resultPath(workspace: string, result: SearchResult): string {
	switch (result.kind) {
		case "comment":
			return (
				`/${workspace}/issues/${result.reference ?? result.issueId}` +
				`?comment=${result.id}#comment-${result.id}`
			);
		case "project":
			return `/${workspace}/projects/${result.slug ?? result.id}`;
		case "team":
			return `/${workspace}/teams/${result.teamKey ?? result.id}`;
		case "person":
			return `/${workspace}/settings/members?q=${encodeURIComponent(result.title)}`;
		default:
			return `/${workspace}/issues/${result.reference ?? result.id}`;
	}
}

export function searchPath(workspace: string, query: string): string {
	return `/${workspace}/search?q=${encodeURIComponent(query)}`;
}

export function resultCount(groups: SearchGroup[]): number {
	return groups.reduce((total, group) => total + group.results.length, 0);
}

export function flatten(groups: SearchGroup[]): SearchResult[] {
	return groups.flatMap((group) => group.results);
}
