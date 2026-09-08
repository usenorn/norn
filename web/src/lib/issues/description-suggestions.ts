import { api } from "$lib/api";
import type { AccountKind } from "$lib/workspace/members";
import type { Team } from "$lib/team/teams";
import { workspacePath } from "$lib/workspace/navigation";

export type MentionPerson = {
	accountId: string;
	displayName?: string;
	kind?: AccountKind;
	email?: string;
};

export type SuggestionItem = {
	key: string;
	label: string;
	hint: string;
	text: string;
	href?: string;
	agent: boolean;
};

const suggestionLimit = 8;

function matches(text: string, query: string): boolean {
	return text.toLowerCase().includes(query.toLowerCase());
}

export function mentionItems(
	members: MentionPerson[],
	teams: Team[],
	query: string
): SuggestionItem[] {
	const people = members
		.filter((member) => Boolean(member.displayName))
		.map((member) => ({
			key: `account:${member.accountId}`,
			label: member.displayName ?? "",
			hint: member.email ?? "",
			text: `@${member.displayName}`,
			agent: member.kind === "agent",
		}));

	const named = teams.map((team) => ({
		key: `team:${team.id}`,
		label: team.name,
		hint: team.key,
		text: `@${team.name}`,
		agent: false,
	}));

	return [...people, ...named]
		.filter((item) => query === "" || matches(item.label, query) || matches(item.hint, query))
		.slice(0, suggestionLimit);
}

const referenceShape = /^[A-Za-z][A-Za-z0-9]*-\d+$/;

function issueItem(reference: string, title: string, id: string, workspace: string): SuggestionItem {
	return {
		key: id,
		label: title,
		hint: reference,
		text: `${reference} ${title}`,
		href: workspacePath(workspace, `/issues/${reference}`),
		agent: false,
	};
}

async function namedIssue(
	workspaceId: string,
	workspace: string,
	reference: string
): Promise<SuggestionItem[]> {
	const found = await api
		.GET("/workspaces/{workspaceId}/issues/by-reference/{reference}", {
			params: { path: { workspaceId, reference: reference.toUpperCase() } },
		})
		.catch(() => undefined);

	if (!found || found.error || !found.data) return [];

	return [issueItem(found.data.reference, found.data.title, found.data.id, workspace)];
}

export async function issueItems(
	workspaceId: string,
	workspace: string,
	query: string
): Promise<SuggestionItem[]> {
	const asked = query.trim();

	if (asked === "") return [];

	if (referenceShape.test(asked)) {
		const named = await namedIssue(workspaceId, workspace, asked);

		if (named.length > 0) return named;
	}

	const found = await api
		.GET("/workspaces/{workspaceId}/search", {
			params: {
				path: { workspaceId },
				query: { q: asked, kinds: ["issue"], limit: suggestionLimit },
			},
		})
		.catch(() => undefined);

	if (!found || found.error || !found.data) return [];

	const issues = found.data.groups.find((group) => group.kind === "issue");

	return (issues?.results ?? [])
		.filter((result) => Boolean(result.reference))
		.map((result) => issueItem(result.reference ?? "", result.title, result.id, workspace));
}
