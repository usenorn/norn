import type { components } from "$lib/api/dashboard.gen";

export type Notification = components["schemas"]["Notification"];
export type NotificationPage = components["schemas"]["NotificationPage"];
export type NotificationKind = components["schemas"]["NotificationKind"];
export type NotificationReason = components["schemas"]["NotificationReason"];
export type NotificationSubjectKind = components["schemas"]["NotificationSubjectKind"];
export type NotificationActorKind = components["schemas"]["NotificationActorKind"];
export type NotificationChannels = components["schemas"]["NotificationChannels"];
export type NotificationPreferences = components["schemas"]["NotificationPreferences"];
export type NotificationSettings = components["schemas"]["NotificationSettings"];
export type FollowState = components["schemas"]["FollowState"];

export type InboxListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "caught_up" }
	| { kind: "ready"; notifications: Notification[]; nextCursor?: string }
	| { kind: "unavailable" };

export type TeamNotificationSetting =
	| { kind: "loading" }
	| { kind: "ready"; settings: NotificationSettings }
	| { kind: "unavailable" };

export type InboxFilter = "unread" | "all";

export type NotificationFailure =
	| { kind: "snooze_past" }
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

const failureMessages: Record<NotificationFailure["kind"], string> = {
	snooze_past: "Pick a time that has not already passed.",
	gone: "That has already left your inbox.",
	forbidden: "You cannot change this.",
	unavailable: "Nothing changed. Wait a moment and try again.",
};

export function notificationFailureMessage(failure: NotificationFailure): string {
	return failureMessages[failure.kind];
}

export function readNotificationFailure(problem: unknown): NotificationFailure {
	if (typeof problem !== "object" || problem === null || !("status" in problem)) {
		return { kind: "unavailable" };
	}

	const status = (problem as { status?: number }).status;

	if (status === 404) return { kind: "gone" };
	if (status === 403) return { kind: "forbidden" };
	if (status === 422) return { kind: "snooze_past" };

	return { kind: "unavailable" };
}

export function listingFor(page: NotificationPage | undefined, filter: InboxFilter): InboxListing {
	if (!page) return { kind: "unavailable" };

	if (page.notifications.length === 0) {
		return filter === "unread" && page.unread === 0 ? { kind: "caught_up" } : { kind: "empty" };
	}

	return {
		kind: "ready",
		notifications: page.notifications,
		nextCursor: page.nextCursor,
	};
}

export const reasonLabels: Record<NotificationReason, string> = {
	mentioned: "Mentioned you",
	approval: "Waiting on you",
	assigned: "Assigned to you",
	membership: "Added you",
	following: "Following",
};

export const actorKindLabels: Record<NotificationActorKind, string> = {
	user: "A person did this",
	token: "An integration did this",
	agent: "An agent did this",
	system: "Norn did this",
};

const kindVerbs: Record<NotificationKind, string> = {
	assigned: "assigned this to you",
	mentioned: "mentioned you",
	commented: "commented",
	state_changed: "changed the state",
	membership: "added you",
	approval_waiting: "is waiting for you to approve what it proposed",
};

export function summary(notification: Notification): string {
	const verb =
		notification.reason === "mentioned" ? kindVerbs.mentioned : kindVerbs[notification.kind];
	const actor = notification.actorName ?? "Someone";

	if (notification.unreadCount > 1) {
		return `${actor} ${verb} and ${notification.unreadCount - 1} more`;
	}

	return `${actor} ${verb}`;
}

export function subjectPath(workspace: string, notification: Notification): string | null {
	if (notification.kind === "approval_waiting") {
		return `/${workspace}/agents/approvals`;
	}

	if (notification.subjectKind === "project") {
		return notification.projectSlug ? `/${workspace}/projects/${notification.projectSlug}` : null;
	}

	if (notification.subjectKind === "team") {
		return notification.teamKey ? `/${workspace}/settings/teams/${notification.teamKey}` : null;
	}

	return `/${workspace}/issues/${notification.reference ?? notification.subjectId}`;
}

export type PreferenceRow = {
	key: keyof Omit<NotificationPreferences, "agents">;
	label: string;
	description: string;
};

export const preferenceRows: PreferenceRow[] = [
	{
		key: "assigned",
		label: "Assigned to you",
		description: "Someone put an issue in your hands.",
	},
	{
		key: "mentioned",
		label: "Mentions",
		description: "Someone wrote your name in a comment.",
	},
	{
		key: "commented",
		label: "Comments",
		description: "Someone replied on something you follow.",
	},
	{
		key: "stateChanged",
		label: "State changes",
		description: "Something you follow moved, or changed team.",
	},
	{
		key: "membership",
		label: "Added to a project or team",
		description: "Someone brought you into a group.",
	},
	{
		key: "approvals",
		label: "Waiting on you",
		description: "An agent has stopped and cannot carry on until you approve what it proposed.",
	},
];

export function defaultPreferences(): NotificationPreferences {
	return {
		assigned: { inbox: true, email: true },
		mentioned: { inbox: true, email: true },
		commented: { inbox: true, email: false },
		stateChanged: { inbox: true, email: false },
		membership: { inbox: true, email: false },
		approvals: { inbox: true, email: true },
		agents: { inbox: true, email: true },
	};
}
