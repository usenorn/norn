import type { IssueProgress } from "$lib/issues/board";
import { keys } from "$lib/api/keys";
import type { ProjectDetail } from "$lib/projects/projects";
import { projectPreviewStates } from "./preview";
import type { components } from "$lib/api/dashboard.gen";
import type { ActivityFeed } from "$lib/activity/activity";
import type { PageServerLoad } from "./$types";

export type ProjectPageData = {
	detail: ProjectDetail;
	progress: IssueProgress | undefined;
};

const unavailable: ProjectPageData = {
	detail: { kind: "unavailable" },
	progress: undefined,
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<ProjectPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.projects(workspace.id));
	depends(keys.issues(workspace.id));

	if (import.meta.env.DEV && projectPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { detail: { kind: "loading" }, progress: undefined };
	}

	const projects = await locals.api.GET("/workspaces/{workspaceId}/projects", {
		params: { path: { workspaceId: workspace.id }, query: { archived: true } },
	});

	if (projects.error || !projects.data) return unavailable;

	const project = projects.data.find((candidate) => candidate.slug === params.slug);

	if (!project) return { ...unavailable, detail: { kind: "not_found" } };

	const path = { workspaceId: workspace.id, projectId: project.id };

	const [members, updates, issues, activity, progress] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/projects/{projectId}/members", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/projects/{projectId}/status", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues", {
			params: {
				path: { workspaceId: workspace.id },
				query: { projectId: project.id, limit: 200 },
			},
		}),
		locals.api.GET("/workspaces/{workspaceId}/projects/{projectId}/activity", {
			params: { path: { workspaceId: workspace.id, projectId: project.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/progress", {
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
