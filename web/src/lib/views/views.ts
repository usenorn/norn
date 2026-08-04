import Bookmark from "@lucide/svelte/icons/bookmark";
import Globe from "@lucide/svelte/icons/globe";
import Users from "@lucide/svelte/icons/users";
import type { components } from "$lib/api/dashboard.gen";
import { workspacePath, type NavEntry } from "$lib/workspace/navigation";

export type SavedView = components["schemas"]["SavedView"];
export type SavedViewSharing = components["schemas"]["SavedViewSharing"];
export type ViewReference = components["schemas"]["IssueFilterReference"];
export type ViewReferenceState = components["schemas"]["IssueFilterReferenceState"];

export type ViewListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; views: SavedView[] }
	| { kind: "unavailable" };

export type ViewFailure =
	| { kind: "sharing_changed"; sharing: SavedViewSharing; team: string }
	| { kind: "team_required" }
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const sharings: SavedViewSharing[] = ["personal", "team", "workspace"];

export const sharingLabels: Record<SavedViewSharing, string> = {
	personal: "Only me",
	team: "One team",
	workspace: "Everyone here",
};

export const sharingNotes: Record<SavedViewSharing, string> = {
	personal: "Nobody else sees this view or knows it exists.",
	team: "Everyone on the team sees it in their sidebar, and sees it through their own permissions.",
	workspace: "Everyone in this workspace sees it, each through their own permissions.",
};

const sharingIcons: Record<SavedViewSharing, NavEntry["icon"]> = {
	personal: Bookmark,
	team: Users,
	workspace: Globe,
};

const failureMessages: Record<ViewFailure["kind"], string> = {
	sharing_changed: "Someone changed who this view is shared with. Check the warning and try again.",
	team_required: "Choose the team to share it with.",
	gone: "That view has already been removed.",
	forbidden: "Only whoever saved this view, or a workspace administrator, can change it.",
	unavailable: "Nothing changed. Wait a moment and try again.",
};

export function viewFailureMessage(failure: ViewFailure): string {
	return failureMessages[failure.kind];
}

export function readViewFailure(problem: unknown): ViewFailure {
	if (typeof problem !== "object" || problem === null) return { kind: "unavailable" };

	if ("code" in problem && problem.code === "saved_view_shared") {
		const sharing = "sharing" in problem ? (problem.sharing as SavedViewSharing) : "workspace";
		const team = "teamName" in problem ? String(problem.teamName ?? "") : "";

		return { kind: "sharing_changed", sharing, team };
	}

	if ("errors" in problem && Array.isArray(problem.errors)) {
		const fields = problem.errors.map((entry: { field?: string }) => entry.field ?? "");

		if (fields.includes("teamId")) return { kind: "team_required" };
	}

	if ("status" in problem) {
		if (problem.status === 403) return { kind: "forbidden" };
		if (problem.status === 404) return { kind: "gone" };
	}

	return { kind: "unavailable" };
}

export function listingFor(views: SavedView[] | null | undefined): ViewListing {
	if (!views) return { kind: "unavailable" };
	if (views.length === 0) return { kind: "empty" };

	return { kind: "ready", views };
}

export function viewsOf(listing: ViewListing): SavedView[] {
	return listing.kind === "ready" ? listing.views : [];
}

export function viewPath(workspace: string, view: SavedView): string {
	return workspacePath(workspace, `/issues?view=${view.id}`);
}

export function viewsPath(workspace: string): string {
	return workspacePath(workspace, "/views");
}

export function scopeOf(view: SavedView): string {
	if (view.sharing === "team") return view.teamName ?? "one team";

	return sharingLabels[view.sharing];
}

export const referenceFieldLabels: Record<string, string> = {
	team: "Team",
	state: "State",
	assignee: "Assignee",
	creator: "Created by",
	label: "Label",
	project: "Project",
	cycle: "Cycle",
};

export function missingReferences(references: ViewReference[]): ViewReference[] {
	return references.filter((reference) => reference.state === "missing");
}

export function referenceLabel(reference: ViewReference): string {
	const field = referenceFieldLabels[reference.field] ?? reference.field;

	switch (reference.state) {
		case "resolved":
			return `${field} ${reference.name ?? ""}`.trim();
		case "restricted":
			return `${field} on a team you cannot see`;
		default:
			return `${field} that no longer exists`;
	}
}

export function viewEntries(workspace: string, views: SavedView[]): NavEntry[] {
	return views.map((view) => ({
		label: view.name,
		href: viewPath(workspace, view),
		icon: sharingIcons[view.sharing],
	}));
}

export function reordered(views: SavedView[], id: string, delta: number): string[] {
	const ids = views.map((view) => view.id);
	const from = ids.indexOf(id);
	const to = from + delta;

	if (from < 0 || to < 0 || to >= ids.length) return ids;

	ids.splice(to, 0, ...ids.splice(from, 1));

	return ids;
}
