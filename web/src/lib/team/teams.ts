import type { components, operations } from "$lib/api/dashboard.gen";

export type Team = components["schemas"]["Team"];
export type TeamStatus = components["schemas"]["TeamStatus"];
export type TeamVisibility = components["schemas"]["TeamVisibility"];

export const teamKeyPattern = /^[A-Z]{2,5}$/;

export type TeamListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; teams: Team[] }
	| { kind: "created"; teams: Team[]; team: Team }
	| { kind: "unavailable" };

export type TeamListView = "list" | "create";

export const teamListTabs = ["active", "archived"] as const;

export type TeamListTab = (typeof teamListTabs)[number];

export type TeamCreationFailure =
	| { kind: "key_taken"; key: string; suggestions: string[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

type CreateResponses = operations["createWorkspaceTeam"]["responses"];
type UpdateResponses = operations["updateWorkspaceTeam"]["responses"];

export type CreateTeamProblem =
	CreateResponses[401 | 403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export type UpdateTeamProblem =
	UpdateResponses[401 | 403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export const visibilityLabels: Record<TeamVisibility, string> = {
	public: "Everyone in the workspace",
	private: "Only team members",
};

export const visibilityNotes: Record<TeamVisibility, string> = {
	public: "Anyone in the workspace can find this team and read its issues.",
	private: "Only members of this team, and workspace admins, can see that it exists.",
};

export function listingFor(teams: Team[] | null): TeamListing {
	if (teams === null) return { kind: "unavailable" };
	if (teams.length === 0) return { kind: "empty" };

	return { kind: "ready", teams };
}

export function teamsIn(teams: Team[], tab: TeamListTab): Team[] {
	return teams.filter((team) => (tab === "archived" ? team.status === "archived" : team.status === "active"));
}

export function teamSummary(teamIds: string[], teams: Team[]): string {
	if (teamIds.length === 0) return "No team";
	if (teamIds.length > 1) return `${teamIds.length} teams`;

	return teams.find((team) => team.id === teamIds[0])?.key ?? "1 team";
}

const teamKeyFiller = new Set(["THE", "AND", "OF", "FOR", "TEAM", "SQUAD", "GROUP", "CREW", "UNIT"]);

export function teamKeyFromName(name: string): string {
	const words = name
		.normalize("NFD")
		.replace(/\p{Diacritic}/gu, "")
		.toUpperCase()
		.split(/[^A-Z]+/)
		.filter(Boolean);

	if (words.length === 0) return "";

	const meaningful = words.filter((word) => !teamKeyFiller.has(word));
	const chosen = meaningful.length > 0 ? meaningful : words;

	const key =
		chosen.length > 1
			? chosen
					.map((word) => word[0])
					.join("")
					.slice(0, 5)
			: chosen[0].length <= 5
				? chosen[0]
				: chosen[0].slice(0, 3);

	return key.length < 2 ? "" : key;
}

export function teamKeySuggestions(name: string, rejected: string, taken: string[]): string[] {
	const letters = name.toUpperCase().replace(/[^A-Z]/g, "");
	const skeleton = letters.replace(/[AEIOU]/g, "");
	const unusable = new Set([rejected.toUpperCase(), ...taken.map((key) => key.toUpperCase())]);

	const candidates = [
		letters.slice(0, 4),
		letters.slice(0, 5),
		skeleton.slice(0, 3),
		skeleton.slice(0, 4),
		letters.slice(0, 2) + letters.slice(-1),
	];

	const offered: string[] = [];

	for (const candidate of candidates) {
		if (!teamKeyPattern.test(candidate)) continue;
		if (unusable.has(candidate) || offered.includes(candidate)) continue;

		offered.push(candidate);
	}

	return offered.slice(0, 3);
}

export function teamNameMessage(code: string): string {
	switch (code) {
		case "required":
			return "Enter a team name.";
		case "too_long":
			return "Keep the name under 80 characters.";
		default:
			return "That name cannot be used.";
	}
}

export function teamKeyMessage(code: string): string {
	switch (code) {
		case "required":
			return "Enter a key.";
		case "too_short":
			return "Use at least 2 letters.";
		case "too_long":
			return "Use at most 5 letters.";
		case "malformed":
			return "Two to five letters, A–Z.";
		default:
			return "That key cannot be used.";
	}
}

export function visibilityMessage(code: string): string {
	switch (code) {
		case "required":
			return "Choose who can see this team.";
		default:
			return "That visibility cannot be used.";
	}
}
