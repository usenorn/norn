import type { components, operations } from "$lib/api/dashboard.gen";
import { workspacePath } from "$lib/workspace/navigation";

export type Webhook = components["schemas"]["Webhook"];
export type MintedWebhook = components["schemas"]["MintedWebhook"];
export type WebhookEventType = components["schemas"]["WebhookEventType"];
export type WebhookDisableReason = components["schemas"]["WebhookDisableReason"];
export type WebhookDelivery = components["schemas"]["WebhookDelivery"];
export type WebhookDeliveryDetail = components["schemas"]["WebhookDeliveryDetail"];
export type WebhookDeliveryEventType = components["schemas"]["WebhookDeliveryEventType"];
export type WebhookDeliveryState = components["schemas"]["WebhookDeliveryState"];
export type WebhookAttempt = components["schemas"]["WebhookAttempt"];
export type WebhookAttemptOutcome = components["schemas"]["WebhookAttemptOutcome"];

type CreateResponses = operations["createWorkspaceWebhook"]["responses"];

export type WebhookProblem =
	CreateResponses[403 | 422 | 500 | 503]["content"]["application/problem+json"];

export type WebhooksView =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "list"; webhooks: Webhook[] }
	| { kind: "created"; webhooks: Webhook[]; webhook: Webhook; secret: string }
	| { kind: "signing_unavailable" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type WebhookDetailView =
	| { kind: "loading" }
	| {
			kind: "detail";
			webhook: Webhook;
			deliveries: WebhookDelivery[];
			nextCursor?: string;
			secret?: string;
			lastAttempt?: WebhookAttempt;
	  }
	| { kind: "not_found" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type WebhookFailure =
	| { kind: "destination_refused" }
	| { kind: "signing_unavailable" }
	| { kind: "not_replayable" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function webhookFailure(problem: WebhookProblem): WebhookFailure {
	if ("code" in problem) {
		switch (problem.code) {
			case "webhook_destination_refused":
				return { kind: "destination_refused" };
			case "webhook_signing_unavailable":
				return { kind: "signing_unavailable" };
			case "forbidden":
				return { kind: "forbidden" };
		}
	}

	if (problem.status === 403) return { kind: "forbidden" };
	if (problem.status === 409) return { kind: "not_replayable" };

	return { kind: "unavailable" };
}

export function failureMessage(failure: WebhookFailure): string {
	switch (failure.kind) {
		case "destination_refused":
			return "That address resolves inside this instance's own network, so Norn will not post to it — otherwise a webhook becomes a way to probe the network Norn runs in. An operator can allow specific ranges with NORN_WEBHOOKS_ALLOWED_DESTINATIONS.";
		case "signing_unavailable":
			return "This instance has no encryption key, so nothing can be signed and no secret can be stored. An operator must set NORN_SECURITY_ENCRYPTION_KEY.";
		case "not_replayable":
			return "That delivery has not settled yet. Wait for it to succeed or give up, then replay it.";
		case "forbidden":
			return "You need to administer this workspace to manage its webhooks.";
		case "unavailable":
			return "Check your connection and try again.";
	}
}

export type WebhookEventGroup = {
	subject: string;
	label: string;
	events: WebhookEventType[];
};

export const eventGroups: WebhookEventGroup[] = [
	{
		subject: "issue",
		label: "Issues",
		events: [
			"issue.created",
			"issue.updated",
			"issue.status_changed",
			"issue.triaged",
			"issue.merged",
			"issue.deleted",
		],
	},
	{
		subject: "comment",
		label: "Comments",
		events: ["comment.posted", "comment.edited", "comment.deleted"],
	},
	{
		subject: "project",
		label: "Projects",
		events: [
			"project.created",
			"project.updated",
			"project.status_posted",
			"project.members_changed",
			"project.deleted",
		],
	},
	{
		subject: "cycle",
		label: "Cycles",
		events: ["cycle.started", "cycle.closed", "cycle.cadence_changed"],
	},
	{
		subject: "membership",
		label: "Members",
		events: ["membership.added", "membership.changed", "membership.removed"],
	},
	{
		subject: "team_membership",
		label: "Team rosters",
		events: ["team_membership.added", "team_membership.removed"],
	},
];

export const eventCatalog: WebhookEventType[] = eventGroups.flatMap((group) => group.events);

const eventLabels: Record<WebhookDeliveryEventType, string> = {
	"issue.created": "Issue raised",
	"issue.updated": "Issue changed",
	"issue.status_changed": "Issue status changed",
	"issue.deleted": "Issue deleted",
	"issue.triaged": "Issue triaged",
	"issue.merged": "Issue merged",
	"comment.posted": "Comment posted",
	"comment.edited": "Comment edited",
	"comment.deleted": "Comment deleted",
	"project.created": "Project created",
	"project.updated": "Project changed",
	"project.deleted": "Project deleted",
	"project.members_changed": "Project members changed",
	"project.status_posted": "Project update posted",
	"cycle.started": "Cycle started",
	"cycle.closed": "Cycle closed",
	"cycle.cadence_changed": "Cycle cadence changed",
	"membership.added": "Member added",
	"membership.changed": "Member changed",
	"membership.removed": "Member removed",
	"team_membership.added": "Added to team",
	"team_membership.removed": "Removed from team",
	"webhook.test": "Test event",
};

export function eventLabel(event: WebhookDeliveryEventType): string {
	return eventLabels[event] ?? event;
}

const stateLabels: Record<WebhookDeliveryState, string> = {
	pending: "Waiting",
	succeeded: "Delivered",
	failed: "Failed",
};

export function stateLabel(state: WebhookDeliveryState): string {
	return stateLabels[state] ?? state;
}

const outcomeLabels: Record<WebhookAttemptOutcome, string> = {
	succeeded: "Accepted",
	rejected: "Rejected",
	refused: "Connection refused",
	timed_out: "Timed out",
	failed: "Failed",
};

export function outcomeLabel(outcome: WebhookAttemptOutcome): string {
	return outcomeLabels[outcome] ?? outcome;
}

const disableReasonLabels: Record<WebhookDisableReason, string> = {
	by_owner: "Somebody turned this subscription off",
	sustained_failure: "Norn turned this subscription off after repeated failures",
};

export function disableReasonLabel(reason: WebhookDisableReason): string {
	return disableReasonLabels[reason] ?? reason;
}

export function deliverySummary(delivery: WebhookDelivery): string {
	const attempts = `${delivery.attempt} attempt${delivery.attempt === 1 ? "" : "s"}`;
	const lead = delivery.replayOf ? "Replay · " : "";

	switch (delivery.state) {
		case "pending":
			return delivery.attempt === 0
				? `${lead}Queued, not sent yet`
				: `${lead}${attempts} so far, trying again`;
		case "succeeded":
			return `${lead}Delivered on attempt ${delivery.attempt}`;
		case "failed":
			return `${lead}Gave up after ${attempts}`;
	}
}

export function webhooksPath(workspace: string): string {
	return workspacePath(workspace, "/settings/webhooks");
}

export function webhookPath(workspace: string, webhookId: string): string {
	return workspacePath(workspace, `/settings/webhooks/${webhookId}`);
}
