import { apiFor } from "$lib/api";
import type { IssueProgress } from "$lib/issues/board";
import type { ProjectDetail } from "$lib/projects/projects";
import { projectPreviewStates } from "./preview";
import type { components } from "$lib/api/dashboard.gen";
import type { ActivityFeed } from "$lib/activity/activity";
import type { PageLoad } from "./$types";

export type ProjectPageData = {
	detail: ProjectDetail;
	progress: IssueProgress | undefined;
};

const unavailable: ProjectPageData = {
	detail: { kind: "unavailable" },
	progress: undefined,
};

export const load: PageLoad = async ({ fetch, params, parent, url }): Promise<ProjectPageData> => {
	const api = apiFor(url);

	const { workspace } = await parent();

	if (import.meta.env.DEV && projectPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { detail: { kind: "loading" }, progress: undefined };
	}

	const projects = await api.GET("/workspaces/{workspaceId}/projects", {
		fetch,
		params: { path: { workspaceId: workspace.id }, query: { archived: true } },
	});

	if (projects.error || !projects.data) return unavailable;

	const project = projects.data.find((candidate) => candidate.slug === params.slug);

	if (!project) return { ...unavailable, detail: { kind: "not_found" } };

	const path = { workspaceId: workspace.id, projectId: project.id };

	const [members, updates, issues, activity, progress] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/projects/{projectId}/members", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/projects/{projectId}/status", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/issues", {
			fetch,
			params: {
				path: { workspaceId: workspace.id },
				query: { projectId: project.id, limit: 200 },
			},
		}),
		api.GET("/workspaces/{workspaceId}/projects/{projectId}/activity", {
			fetch,
			params: { path: { workspaceId: workspace.id, projectId: project.id } },
		}),
		api.GET("/workspaces/{workspaceId}/issues/progress", {
			fetch,
			params: { path: { workspaceId: workspace.id }, query: { projectId: project.id } },
		}),
	]);

	return {
		detail: {
			kind: "ready",
			project,
			members: members.data ?? [],
			updates: updates.data ?? [],
			issues: issues.data?.issues ?? [],
			activity: readActivity(activity.data),
		},
		progress: progress.data,
	};
};

function readActivity(page: components["schemas"]["ActivityPage"] | undefined): ActivityFeed {
	if (!page) return { kind: "unavailable" };
	if (page.events.length === 0) return { kind: "empty" };

	return { kind: "ready", events: page.events, nextCursor: page.nextCursor };
}
