import type { Team } from "./teams";

export type TeamSettings =
	| { kind: "loading" }
	| { kind: "ready"; team: Team }
	| { kind: "saved"; team: Team }
	| { kind: "archived"; team: Team }
	| { kind: "read_only"; team: Team }
	| { kind: "not_found" }
	| { kind: "unavailable" };

export function settingsFor(team: Team): TeamSettings {
	return team.status === "archived" ? { kind: "archived", team } : { kind: "ready", team };
}

export function teamOf(settings: TeamSettings): Team | null {
	return "team" in settings ? settings.team : null;
}
