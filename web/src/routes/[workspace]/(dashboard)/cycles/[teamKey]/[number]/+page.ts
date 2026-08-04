import { apiFor } from "$lib/api";
import type { CycleDetail } from "$lib/cycles/cycles";
import type { IssueProgress } from "$lib/issues/board";
import type { WorkflowState } from "$lib/team/states";
import { cyclePreviewStates } from "./preview";
import type { PageLoad } from "./$types";

export type CyclePageData = {
	detail: CycleDetail;
	progress: IssueProgress | undefined;
	states: WorkflowState[] | undefined;
};

const unavailable: CyclePageData = {
	detail: { kind: "unavailable" },
	progress: undefined,
	states: undefined,
};

export const load: PageLoad = async ({ fetch, params, parent, url }): Promise<CyclePageData> => {
	const api = apiFor(url);

	const { workspace, teams } = await parent();

	if (import.meta.env.DEV && cyclePreviewStates[url.searchParams.get("state") ?? ""]) {
		return { detail: { kind: "loading" }, progress: undefined, states: undefined };
	}

	if (!teams) return unavailable;

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { ...unavailable, detail: { kind: "not_found" } };

	const cycles = await api.GET("/workspaces/{workspaceId}/cycles", {
		fetch,
		params: { path: { workspaceId: workspace.id }, query: { teamId: team.id } },
	});

	if (cycles.error || !cycles.data) return unavailable;

	const cycle = cycles.data.find((candidate) => candidate.number === Number(params.number));

	if (!cycle) return { ...unavailable, detail: { kind: "not_found" } };

	const later = cycles.data
		.filter((candidate) => candidate.startsOn > cycle.endsOn && candidate.phase !== "closed")
		.sort((a, b) => a.startsOn.localeCompare(b.startsOn));

	const [scope, progress, states] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/cycles/{cycleId}/scope", {
			fetch,
			params: { path: { workspaceId: workspace.id, cycleId: cycle.id } },
		}),
		api.GET("/workspaces/{workspaceId}/issues/progress", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { cycleId: cycle.id } },
		}),
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			fetch,
			params: { path: { workspaceId: workspace.id, teamId: team.id } },
		}),
	]);

	if (scope.error || !scope.data) return unavailable;

	return {
		detail: {
			kind: "ready",
			cycle,
			scope: scope.data,
			nextNumber: later.length > 0 ? later[0].number : null,
		},
		progress: progress.data,
		states: states.data,
	};
};
