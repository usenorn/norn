import { error, redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import { reachOfSlug, type SignedInAccount } from "$lib/account/accounts";
import { lastWorkspaceCookie } from "$lib/account/last-workspace";
import type { components } from "$lib/api/dashboard.gen";
import type { TeamCycle } from "$lib/cycles/cycles";
import type { Label } from "$lib/labels/labels";
import type { Membership } from "$lib/workspace/members";
import type { Project } from "$lib/projects/projects";
import type { SavedView } from "$lib/views/views";
import { waitingTotal } from "$lib/triage/triage";
import { expansionCookie, readExpanded } from "$lib/workspace/sidebar";
import type { Team } from "$lib/team/teams";
import type { WorkflowState } from "$lib/team/states";
import type { LayoutServerLoad } from "./$types";

export type WorkspaceSummary = components["schemas"]["Workspace"];

export type WorkspaceScope = {
	now: string;
	workspace: WorkspaceSummary;
	workspaces: WorkspaceSummary[];
	accounts: SignedInAccount[];
	member: { id: string; name: string; email: string; slot: string };
	teams: Team[] | null;
	cycles: TeamCycle[];
	projects: Project[];
	views: SavedView[] | null;
	members: Membership[];
	labels: Label[];
	states: WorkflowState[];
	waiting: number;
	unread: number;
	narrowed: boolean;
	expanded: string[];
};

export const load: LayoutServerLoad = async ({
	cookies,
	depends,
	locals,
	params,
	parent,
}): Promise<WorkspaceScope> => {
	const { accounts, acting } = await parent();

	if (!acting) redirect(307, "/sign-in");

	const reach = reachOfSlug(accounts, params.workspace, acting.slot);

	if (!reach) error(404, "That workspace does not exist, or you are not a member of it.");

	const workspace = reach.workspace.workspace;
	const signedIn = reach.account;

	cookies.set(lastWorkspaceCookie(signedIn.account.id), workspace.slug, {
		path: "/",
		httpOnly: true,
		sameSite: "lax",
		maxAge: 60 * 60 * 24 * 365,
	});

	depends(keys.workspaceScope(workspace.id));
	depends(keys.projects(workspace.id));
	depends(keys.cycles(workspace.id));
	depends(keys.views(workspace.id));
	depends(keys.triage(workspace.id));
	depends(keys.inbox(workspace.id));
	depends(keys.members(workspace.id));
	depends(keys.labels(workspace.id));
	depends(keys.states(workspace.id));

	const path = { workspaceId: workspace.id };

	const [teams, cycles, projects, views, triage, inbox, members, labels] = await Promise.all([
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
		locals.api.GET("/workspaces/{workspaceId}/members", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/labels", { params: { path } }),
	]);

	const reachable = teams.data ?? [];

	const workflows = await Promise.all(
		reachable.map((team) =>
			locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
				params: { path: { ...path, teamId: team.id } },
			})
		)
	);

	return {
		narrowed: requiresProvider(teams.error),
		now: new Date().toISOString(),
		workspace,
		workspaces: signedIn.workspaces.map((reach) => reach.workspace),
		accounts,
		member: {
			id: signedIn.account.id,
			name: signedIn.account.displayName,
			email: signedIn.account.email,
			slot: reach.workspace.slot,
		},
		teams: teams.data ?? null,
		cycles: cycles.data ?? [],
		projects: projects.data ?? [],
		views: views.data ?? null,
		members: members.data?.members ?? [],
		labels: labels.data ?? [],
		states: workflows.flatMap((workflow) => workflow.data ?? []),
		waiting: waitingTotal(triage.data),
		unread: inbox.data?.unread ?? 0,
		expanded: readExpanded(
			cookies.get(expansionCookie(signedIn.account.id, workspace.id)),
			reachable.filter((team) => team.status === "active").map((team) => team.key)
		),
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
