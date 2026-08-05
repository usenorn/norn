import { error, redirect } from "@sveltejs/kit";
import { apiFor } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";
import type { TeamCycle } from "$lib/cycles/cycles";
import type { Project } from "$lib/projects/projects";
import type { SavedView } from "$lib/views/views";
import { waitingTotal } from "$lib/triage/triage";
import type { Team } from "$lib/team/teams";
import type { LayoutLoad } from "./$types";

export type WorkspaceSummary = components["schemas"]["Workspace"];

export type WorkspaceScope = {
	now: string;
	workspace: WorkspaceSummary;
	workspaces: WorkspaceSummary[];
	member: { id: string; name: string };
	teams: Team[] | null;
	cycles: TeamCycle[];
	projects: Project[];
	views: SavedView[] | null;
	waiting: number;
	unread: number;
	narrowed: boolean;
};

export const load: LayoutLoad = async ({ fetch, params, url}): Promise<WorkspaceScope> => {
	const api = apiFor(url);

	const [workspaces, account] = await Promise.all([
		api.GET("/workspaces", { fetch }),
		api.GET("/accounts/me", { fetch }),
	]);

	if (workspaces.error || !workspaces.data) redirect(307, "/sign-in");

	const workspace = workspaces.data.find((candidate) => candidate.slug === params.workspace);

	if (!workspace) error(404, "That workspace does not exist, or you are not a member of it.");

	const [teams, cycles, projects, views, triage, inbox] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/teams", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/cycles/current", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/projects", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { mine: true } },
		}),
		api.GET("/workspaces/{workspaceId}/saved-views", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/triage", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { limit: 1 } },
		}),
		api.GET("/workspaces/{workspaceId}/notifications", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { limit: 1 } },
		}),
	]);

	return {
		narrowed: requiresProvider(teams.error),
		now: new Date().toISOString(),
		workspace,
		workspaces: workspaces.data,
		member: { id: account.data?.id ?? "", name: account.data?.displayName ?? "" },
		teams: teams.data ?? null,
		cycles: cycles.data ?? [],
		projects: projects.data ?? [],
		views: views.data ?? null,
		waiting: waitingTotal(triage.data),
		unread: inbox.data?.unread ?? 0,
	};
};

function requiresProvider(problem: unknown): boolean {
	return (
		typeof problem === "object" &&
		problem !== null &&
		"reason" in problem &&
		problem.reason === "auth_method_not_permitted"
	);
}
