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

export async function issueItems(
	workspaceId: string,
	workspace: string,
	query: string
): Promise<SuggestionItem[]> {
	if (query.trim() === "") return [];

	const found = await api.GET("/workspaces/{workspaceId}/search", {
		params: {
			path: { workspaceId },
			query: { q: query, kinds: ["issue"], limit: suggestionLimit },
		},
	});

	if (found.error || !found.data) return [];

	const issues = found.data.groups.find((group) => group.kind === "issue");

	return (issues?.results ?? [])
		.filter((result) => Boolean(result.reference))
		.map((result) => ({
			key: result.id,
			label: result.title,
			hint: result.reference ?? "",
			text: `${result.reference} ${result.title}`,
			href: workspacePath(workspace, `/issues/${result.reference}`),
			agent: false,
		}));
}
