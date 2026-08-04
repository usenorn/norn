import { redirect } from "@sveltejs/kit";
import { apiFor } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";
import type { Cycle } from "$lib/cycles/cycles";
import type { Project } from "$lib/projects/projects";
import type { Issue } from "$lib/issues/issues";
import type { Label, LabelGroup } from "$lib/labels/labels";
import type { WorkflowState } from "$lib/team/states";
import type { PageLoad } from "./$types";

export type Member = components["schemas"]["Membership"];

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export type IssueActivity = components["schemas"]["IssueActivity"];

export type IssueDetail =
	| { kind: "loading" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			issue: Issue;
			states: WorkflowState[];
			labels: Label[];
			groups: LabelGroup[];
			activity: IssueActivity[];
			members: Member[];
			children: Issue[];
			relations: components["schemas"]["IssueRelationGroup"][];
			childProgress: components["schemas"]["IssueProgress"];
			candidates: Issue[];
			cycles: Cycle[];
			projects: Project[];
	  }
	| { kind: "unavailable" };

export type IssuePageData = { detail: IssueDetail };

export const load: PageLoad = async ({ fetch, params, parent, url}): Promise<IssuePageData> => {
	const api = apiFor(url);

	const { workspace } = await parent();
	const identifier = params.reference;

	const issue = uuid.test(identifier)
		? await api.GET("/workspaces/{workspaceId}/issues/{issueId}", {
				fetch,
				params: { path: { workspaceId: workspace.id, issueId: identifier } },
			})
		: await api.GET("/workspaces/{workspaceId}/issues/by-reference/{reference}", {
				fetch,
				params: { path: { workspaceId: workspace.id, reference: identifier } },
			});

	if (issue.error?.status === 404) return { detail: { kind: "not_found" } };
	if (issue.error?.status === 422) return { detail: { kind: "not_found" } };
	if (!issue.data) return { detail: { kind: "unavailable" } };

	if (issue.data.reference !== identifier) {
		redirect(308, `/${params.workspace}/issues/${issue.data.reference}`);
	}

	const path = { workspaceId: workspace.id, issueId: issue.data.id };

	const [states, labels, groups, activity, members, children, candidates, relations, cycles, projects] =
		await Promise.all([
		api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			fetch,
			params: { path: { workspaceId: workspace.id, teamId: issue.data.teamId } },
		}),
		api.GET("/workspaces/{workspaceId}/labels", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/label-groups", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/issues/{issueId}/activity", {
			fetch,
			params: { path },
		}),
		api.GET("/workspaces/{workspaceId}/members", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
		api.GET("/workspaces/{workspaceId}/issues/{issueId}/children", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/issues", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { limit: 200 } },
		}),
		api.GET("/workspaces/{workspaceId}/issues/{issueId}/relations", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/cycles", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { teamId: issue.data.teamId } },
		}),
		api.GET("/workspaces/{workspaceId}/projects", {
			fetch,
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	if (
		!states.data ||
		!labels.data ||
		!groups.data ||
		!activity.data ||
		!members.data ||
		!children.data ||
		!relations.data
	) {
		return { detail: { kind: "unavailable" } };
	}

	return {
		detail: {
			kind: "ready",
			issue: issue.data,
			states: states.data,
			labels: labels.data,
			groups: groups.data,
			activity: activity.data.entries,
			members: members.data.members,
			children: children.data.issues,
			childProgress: children.data.progress,
			candidates: candidates.data?.issues ?? [],
			relations: relations.data.groups,
			cycles: (cycles.data ?? []).filter((cycle) => cycle.phase !== "closed"),
			projects: (projects.data ?? []).filter((project) => !project.archived),
		},
	};
};
