import { api } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";
import { workspacePath } from "$lib/workspace/navigation";

export type SuggestionGroup = {
	label: string;
	items: Suggestion[];
};

export type Suggestion =
	| {
			kind: "person" | "team" | "agent";
			key: string;
			id: string;
			label: string;
			hint: string;
	  }
	| {
			kind: "issue";
			key: string;
			id: string;
			label: string;
			hint: string;
			reference: string;
			href: string;
			status: string;
	  }
	| {
			kind: "project";
			key: string;
			id: string;
			label: string;
			hint: string;
			href: string;
	  };

export type SuggestionOutcome =
	| { state: "ready"; groups: SuggestionGroup[] }
	| { state: "typing" }
	| { state: "empty" }
	| { state: "failed" };

export const suggestionLimit = 8;

const referenceShape = /^[A-Za-z][A-Za-z0-9]*-\d+$/;

/**
 * Autocomplete searches what the person may read in the whole workspace rather than what the
 * page happens to have loaded. A teammate who is not on this board, or an issue from another
 * team, is exactly what somebody reaches for and exactly what a local list cannot offer.
 */
export async function findIssues(
	workspaceId: string,
	workspace: string,
	query: string
): Promise<SuggestionOutcome> {
	const asked = query.trim();

	if (asked === "") return { state: "typing" };

	if (referenceShape.test(asked)) {
		const named = await namedIssue(workspaceId, workspace, asked);

		if (named) return { state: "ready", groups: [{ label: "Issues", items: [named] }] };
	}

	const found = await search(workspaceId, asked, ["issue"]);

	if (found === "failed") return { state: "failed" };

	const issues = issuesOf(found, workspace);

	if (issues.length === 0) return { state: "empty" };

	return { state: "ready", groups: [{ label: "Issues", items: issues }] };
}

export async function findMentions(
	workspaceId: string,
	workspace: string,
	query: string
): Promise<SuggestionOutcome> {
	const asked = query.trim();

	// The search asks for something to search for, so an empty query is a question nobody has
	// finished asking rather than one with no answer.
	if (asked === "") return { state: "typing" };

	const found = await search(workspaceId, asked, ["person", "team", "issue", "project"]);

	if (found === "failed") return { state: "failed" };

	const groups: SuggestionGroup[] = [];

	const people = resultsOf(found, "person").map(
		(result: Result): Suggestion => ({
			kind: result.status === "agent" ? "agent" : "person",
			key: `person:${result.id}`,
			id: result.id,
			label: result.title,
			hint: result.excerpt ?? "",
		})
	);

	const teams = resultsOf(found, "team").map(
		(result: Result): Suggestion => ({
			kind: "team",
			key: `team:${result.id}`,
			id: result.id,
			label: result.title,
			hint: result.teamKey ?? "",
		})
	);

	const projects = resultsOf(found, "project").map(
		(result: Result): Suggestion => ({
			kind: "project",
			key: `project:${result.id}`,
			id: result.id,
			label: result.title,
			hint: result.excerpt ?? "",
			href: workspacePath(workspace, `/projects/${result.slug ?? result.id}`),
		})
	);

	for (const [label, items] of [
		["People", people],
		["Teams", teams],
		["Projects", projects],
		["Issues", issuesOf(found, workspace)],
	] as const) {
		if (items.length > 0) groups.push({ label, items });
	}

	if (groups.length === 0) return { state: "empty" };

	return { state: "ready", groups };
}

type Found = components["schemas"]["SearchResults"];
type Result = components["schemas"]["SearchResult"];

async function search(
	workspaceId: string,
	query: string,
	kinds: ("issue" | "person" | "team" | "project" | "comment")[]
): Promise<Found | "failed"> {
	const found = await api
		.GET("/workspaces/{workspaceId}/search", {
			params: { path: { workspaceId }, query: { q: query, kinds, limit: suggestionLimit } },
			// The spec asks for the kinds as one comma-separated value, and the client repeats
			// the parameter unless told otherwise; the server refuses the repeated form.
			querySerializer: { array: { style: "form", explode: false } },
		})
		.catch(() => undefined);

	if (!found || found.error || !found.data) return "failed";

	return found.data;
}

function resultsOf(found: Found | "failed", kind: string): Result[] {
	if (found === "failed" || !found) return [];

	return found.groups.find((group) => group.kind === kind)?.results ?? [];
}

function issuesOf(found: Found | "failed", workspace: string): Suggestion[] {
	return resultsOf(found, "issue")
		.filter((result: Result) => Boolean(result.reference))
		.map(
			(result: Result): Suggestion => ({
				kind: "issue",
				key: `issue:${result.id}`,
				id: result.id,
				label: result.title,
				hint: result.reference ?? "",
				reference: result.reference ?? "",
				status: result.status ?? "",
				href: workspacePath(workspace, `/issues/${result.reference}`),
			})
		);
}

async function namedIssue(
	workspaceId: string,
	workspace: string,
	reference: string
): Promise<Suggestion | null> {
	const found = await api
		.GET("/workspaces/{workspaceId}/issues/by-reference/{reference}", {
			params: { path: { workspaceId, reference: reference.toUpperCase() } },
		})
		.catch(() => undefined);

	if (!found || found.error || !found.data) return null;

	return {
		kind: "issue",
		key: `issue:${found.data.id}`,
		id: found.data.id,
		label: found.data.title,
		hint: found.data.reference,
		reference: found.data.reference,
		status: found.data.state.name,
		href: workspacePath(workspace, `/issues/${found.data.reference}`),
	};
}
