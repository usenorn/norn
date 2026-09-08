import type { CycleDetail } from "$lib/cycles/cycles";
import type { TeamMember } from "$lib/team/members";
import { keys } from "$lib/api/keys";
import type { IssueProgress } from "$lib/issues/board";
import type { WorkflowState } from "$lib/team/states";
import { cyclePreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export type CyclePageData = {
	detail: CycleDetail;
	progress: IssueProgress | undefined;
	states: WorkflowState[] | undefined;
	teamMembers: TeamMember[] | undefined;
};

const unavailable: CyclePageData = {
	detail: { kind: "unavailable" },
	progress: undefined,
	states: undefined,
	teamMembers: undefined,
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<CyclePageData> => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();

	depends(keys.cycles(workspace.id));
	depends(keys.issues(workspace.id));

	if (import.meta.env.DEV && cyclePreviewStates[url.searchParams.get("state") ?? ""]) {
		return {
			detail: { kind: "loading" },
			progress: undefined,
			states: undefined,
			teamMembers: undefined,
		};
	}

	if (!teams) return unavailable;

	const team = teams.find((candidate) => candidate.key === params.teamKey.toUpperCase());

	if (!team) return { ...unavailable, detail: { kind: "not_found" } };

	const cycles = await locals.api.GET("/workspaces/{workspaceId}/cycles", {
		params: { path: { workspaceId: workspace.id }, query: { teamId: team.id } },
	});

	if (cycles.error || !cycles.data) return unavailable;

	const cycle = cycles.data.find((candidate) => candidate.number === Number(params.number));

	if (!cycle) return { ...unavailable, detail: { kind: "not_found" } };

	const later = cycles.data
		.filter((candidate) => candidate.startsOn > cycle.endsOn && candidate.phase !== "closed")
		.sort((a, b) => a.startsOn.localeCompare(b.startsOn));

	const earlier = cycles.data
		.filter((candidate) => candidate.endsOn < cycle.startsOn)
		.sort((a, b) => b.endsOn.localeCompare(a.endsOn));

	const previous = earlier.length > 0 ? earlier[0] : undefined;

	const [scope, report, progress, states, members, before] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/cycles/{cycleId}/scope", {
			params: { path: { workspaceId: workspace.id, cycleId: cycle.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/cycles/{cycleId}/report", {
			params: { path: { workspaceId: workspace.id, cycleId: cycle.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/progress", {
			params: { path: { workspaceId: workspace.id }, query: { cycleId: cycle.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			params: { path: { workspaceId: workspace.id, teamId: team.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/members", {
			params: { path: { workspaceId: workspace.id, teamId: team.id } },
		}),
		previous
			? locals.api.GET("/workspaces/{workspaceId}/cycles/{cycleId}/report", {
					params: { path: { workspaceId: workspace.id, cycleId: previous.id } },
				})
			: Promise.resolve(undefined),
	]);

	if (scope.error || !scope.data || report.error || !report.data) return unavailable;

	return {
		detail: {
			kind: "ready",
			cycle,
			scope: scope.data,
			report: report.data,
			others: cycles.data.filter((candidate) => candidate.id !== cycle.id),
			previous: before?.data,
			nextNumber: later.length > 0 ? later[0].number : null,
		},
		progress: progress.data,
		states: states.data,
		teamMembers: members.data,
	};
};
