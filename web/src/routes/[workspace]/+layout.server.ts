import { error, redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import type { components } from "$lib/api/dashboard.gen";
import type { TeamCycle } from "$lib/cycles/cycles";
import type { Project } from "$lib/projects/projects";
import type { SavedView } from "$lib/views/views";
import { waitingTotal } from "$lib/triage/triage";
import type { Team } from "$lib/team/teams";
import type { LayoutServerLoad } from "./$types";

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

export const load: LayoutServerLoad = async ({
	depends,
	locals,
	params,
}): Promise<WorkspaceScope> => {

	const [workspaces, account] = await Promise.all([
		locals.api.GET("/workspaces"),
		locals.api.GET("/accounts/me"),
	]);

	if (workspaces.error || !workspaces.data) redirect(307, "/sign-in");

	const workspace = workspaces.data.find((candidate) => candidate.slug === params.workspace);

	if (!workspace) error(404, "That workspace does not exist, or you are not a member of it.");

	depends(keys.workspaceScope(workspace.id));
	depends(keys.projects(workspace.id));
	depends(keys.cycles(workspace.id));
	depends(keys.views(workspace.id));
	depends(keys.triage(workspace.id));
	depends(keys.inbox(workspace.id));

	const path = { workspaceId: workspace.id };

	const [teams, cycles, projects, views, triage, inbox] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/teams", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/cycles/current", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/projects", {
			params: { path, query: { mine: true } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/saved-views", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/triage", { params: { path, query: { limit: 1 } } }),
		locals.api.GET("/workspaces/{workspaceId}/notifications", {
			params: { path, query: { limit: 1 } },
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
