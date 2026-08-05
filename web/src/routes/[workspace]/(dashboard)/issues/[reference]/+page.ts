import { redirect } from "@sveltejs/kit";
import { apiFor } from "$lib/api";
import type { components } from "$lib/api/dashboard.gen";
import type { Cycle } from "$lib/cycles/cycles";
import type { Project } from "$lib/projects/projects";
import type { Issue } from "$lib/issues/issues";
import type { Label, LabelGroup } from "$lib/labels/labels";
import type { CommentThread } from "$lib/comments/comments";
import type { FollowState } from "$lib/notifications/notifications";
import type { AttachmentPanel } from "$lib/attachments/attachments";
import type { WorkflowState } from "$lib/team/states";
import type { PageLoad } from "./$types";

export type Member = components["schemas"]["Membership"];

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

import type { ActivityFeed } from "$lib/activity/activity";

export type IssueDetail =
	| { kind: "loading" }
	| { kind: "not_found" }
	| {
			kind: "ready";
			issue: Issue;
			states: WorkflowState[];
			labels: Label[];
			groups: LabelGroup[];
			activity: ActivityFeed;
			members: Member[];
			children: Issue[];
			relations: components["schemas"]["IssueRelationGroup"][];
			childProgress: components["schemas"]["IssueProgress"];
			candidates: Issue[];
			cycles: Cycle[];
			projects: Project[];
			comments: CommentThread;
			follow: FollowState;
			attachments: AttachmentPanel;
	  }
	| { kind: "unavailable" };

export type IssuePageData = { detail: IssueDetail };

export const load: PageLoad = async ({ fetch, params, parent, url }): Promise<IssuePageData> => {
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

	const [
		states,
		labels,
		groups,
		activity,
		members,
		children,
		candidates,
		relations,
		cycles,
		projects,
		comments,
		attachments,
		follow,
	] = await Promise.all([
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
		api.GET("/workspaces/{workspaceId}/issues/{issueId}/comments", {
			fetch,
			params: { path, query: around(url) },
		}),
		api.GET("/workspaces/{workspaceId}/issues/{issueId}/attachments", { fetch, params: { path } }),
		api.GET("/workspaces/{workspaceId}/issues/{issueId}/follow", { fetch, params: { path } }),
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
			activity: readActivity(activity.data),
			members: members.data.members,
			children: children.data.issues,
			childProgress: children.data.progress,
			candidates: candidates.data?.issues ?? [],
			relations: relations.data.groups,
			cycles: (cycles.data ?? []).filter((cycle) => cycle.phase !== "closed"),
			projects: (projects.data ?? []).filter((project) => !project.archived),
			comments: readThread(comments.data),
			attachments: readAttachments(attachments.data),
			follow: follow.data?.state ?? "muted",
		},
	};
};

function around(url: URL): { around?: string } {
	const comment = url.searchParams.get("comment");

	return comment ? { around: comment } : {};
}

function readThread(page: components["schemas"]["IssueCommentPage"] | undefined): CommentThread {
	if (!page) return { kind: "unavailable" };
	if (page.comments.length === 0) return { kind: "empty" };

	return { kind: "ready", comments: page.comments, nextCursor: page.nextCursor };
}

function readAttachments(
	page: components["schemas"]["AttachmentList"] | undefined
): AttachmentPanel {
	if (!page) return { kind: "unavailable" };
	if (page.attachments.length === 0) return { kind: "empty" };

	return { kind: "ready", attachments: page.attachments };
}

function readActivity(page: components["schemas"]["ActivityPage"] | undefined): ActivityFeed {
	if (!page) return { kind: "unavailable" };
	if (page.events.length === 0) return { kind: "empty" };

	return { kind: "ready", events: page.events, nextCursor: page.nextCursor };
}
