import { error, redirect } from "@sveltejs/kit";
import { apiFor } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";
import type { Team } from "$lib/team/teams";
import type { NavProject } from "$lib/workspace/navigation";
import type { LayoutLoad } from "./$types";

export type WorkspaceSummary = components["schemas"]["Workspace"];

export type WorkspaceScope = {
	now: string;
	workspace: WorkspaceSummary;
	workspaces: WorkspaceSummary[];
	member: { id: string; name: string };
	teams: Team[] | null;
	projects: NavProject[];
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

	const teams = await api.GET("/workspaces/{workspaceId}/teams", {
		fetch,
		params: { path: { workspaceId: workspace.id } },
	});

	return {
		narrowed: requiresProvider(teams.error),
		now: new Date().toISOString(),
		workspace,
		workspaces: workspaces.data,
		member: { id: account.data?.id ?? "", name: account.data?.displayName ?? "" },
		teams: teams.data ?? null,
		projects: [
			{ name: "Mobile", slug: "mobile", color: "var(--label-blue)" },
			{ name: "Billing", slug: "billing", color: "var(--label-cyan)" },
			{ name: "Growth", slug: "growth", color: "var(--label-violet)" },
		],
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
