import { getContext, setContext } from "svelte";
import type { Team } from "./teams";

export type TeamSettingsScope = {
	readonly team: Team;
	readonly archived: boolean;
	readonly readOnly: boolean;
};

const scopeKey = Symbol("team-settings");

export function provideTeamSettings(scope: TeamSettingsScope): void {
	setContext(scopeKey, scope);
}

export function useTeamSettings(): TeamSettingsScope {
	return getContext<TeamSettingsScope>(scopeKey);
}
