import { redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import type { components } from "$lib/api/dashboard.gen";
import type { Cycle } from "$lib/cycles/cycles";
import type { Project } from "$lib/projects/projects";
import { candidateOf, type Issue, type IssueCandidate } from "$lib/issues/issues";
import type { Label, LabelGroup } from "$lib/labels/labels";
import type { CommentThread } from "$lib/comments/comments";
import { readReceipt, type AssigneeReceipt, type FollowState } from "$lib/notifications/notifications";
import { currentDelegation, type DelegationPanel } from "$lib/agents/delegation";
import type { Execution, IssueChangeSet } from "$lib/executions/executions";
import type { IssueQuestion } from "$lib/questions/questions";
import type { AttachmentPanel } from "$lib/attachments/attachments";
import type {
	CodeLink,
	IssueShipping,
	MirrorConflict,
} from "$lib/source-control/source-control";
import type { WorkflowState } from "$lib/team/states";
import type { PageServerLoad } from "./$types";

export type Member = components["schemas"]["Membership"];

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const candidateLimit = 200;

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
			candidates: IssueCandidate[];
			cycles: Cycle[];
			projects: Project[];
			comments: CommentThread;
			follow: FollowState;
			watchers: string[];
			attachments: AttachmentPanel;
			codeLinks: CodeLink[];
			delegation: DelegationPanel;
			runs: Execution[];
			changeset?: IssueChangeSet;
			questions: IssueQuestion[];
			mirrorConflicts: MirrorConflict[];
			shipping: IssueShipping;
			receipt: AssigneeReceipt;
	  }
	| { kind: "unavailable" };

export type IssuePageData = { detail: IssueDetail };

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
	url,
}): Promise<IssuePageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();
	const identifier = params.reference;

	const issue = uuid.test(identifier)
		? await locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}", {
				params: { path: { workspaceId: workspace.id, issueId: identifier } },
			})
		: await locals.api.GET("/workspaces/{workspaceId}/issues/by-reference/{reference}", {
				params: { path: { workspaceId: workspace.id, reference: identifier } },
			});

	if (issue.error?.status === 404) return { detail: { kind: "not_found" } };
	if (issue.error?.status === 422) return { detail: { kind: "not_found" } };
	if (!issue.data) return { detail: { kind: "unavailable" } };

	if (issue.data.reference !== identifier) {
		redirect(308, `/${params.workspace}/issues/${issue.data.reference}`);
	}

	depends(keys.issue(issue.data.id));

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
		watchers,
		codeLinks,
		mirrorConflicts,
		shipping,
		delegations,
		questions,
		runs,
		changeset,
		directed,
	] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
			params: { path: { workspaceId: workspace.id, teamId: issue.data.teamId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/labels", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/label-groups", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/activity", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/members", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/children", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues", {
			params: { path: { workspaceId: workspace.id }, query: { limit: candidateLimit } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/relations", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/cycles", {
			params: { path: { workspaceId: workspace.id }, query: { teamId: issue.data.teamId } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/projects", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/comments", {
			params: { path, query: around(url) },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/attachments", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/follow", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/followers", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/code-links", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/mirror-conflicts", {
			params: { path },
		}),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/shipping", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/delegation", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/questions", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/executions", { params: { path } }),
		locals.api.GET("/workspaces/{workspaceId}/issues/{issueId}/changeset", { params: { path } }),
		issue.data.assigneeAccountId
			? locals.api.GET("/workspaces/{workspaceId}/notifications/directed", {
					params: {
						path: { workspaceId: workspace.id },
						query: {
							recipientId: issue.data.assigneeAccountId,
							subjectId: issue.data.id,
							limit: 1,
						},
					},
				})
			: Promise.resolve(undefined),
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
			candidates: (candidates.data?.issues ?? []).map(candidateOf),
			relations: relations.data.groups,
			cycles: (cycles.data ?? []).filter((cycle) => cycle.phase !== "closed"),
			projects: (projects.data ?? []).filter((project) => !project.archived),
			comments: readThread(comments.data),
			attachments: readAttachments(attachments.data),
			follow: follow.data?.state ?? "muted",
			watchers: (watchers.data?.followers ?? []).map((watcher) => watcher.accountId),
			codeLinks: codeLinks.data ?? [],
			delegation: delegations.data
				? currentDelegation(delegations.data)
				: { kind: "unavailable" },
			runs: runs.data ?? [],
			changeset: changeset.data,
			questions: questions.data?.questions ?? [],
			receipt: readReceipt(directed?.data),
			mirrorConflicts: mirrorConflicts.data ?? [],
			shipping: shipping.data ?? { releases: [], deployments: [] },
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
