import type { components, operations } from "$lib/api/dashboard.gen";

export { joinedOn, lastActive } from "$lib/time";

export type Membership = components["schemas"]["Membership"];
export type AccountKind = components["schemas"]["AccountKind"];
export type MembershipRole = components["schemas"]["MembershipRole"];
export type MembershipSource = components["schemas"]["MembershipSource"];
export type MemberAuthMethod = components["schemas"]["SessionAuthMethod"];
export type MemberPage = components["schemas"]["MemberPage"];
export type MemberRemovalPreview = components["schemas"]["MemberRemovalPreview"];
export type MemberTeam = components["schemas"]["Team"];

export const memberPageSize = 25;
export const searchDebounceMs = 250;

export const membershipRoles: MembershipRole[] = ["admin", "member", "viewer"];

export function machine(kind: AccountKind | undefined): boolean {
	return kind === "agent" || kind === "integration";
}

export function assignable(kind: AccountKind | undefined): boolean {
	return !machine(kind);
}

export function assignees(members: Membership[]): Membership[] {
	return members.filter((member) => assignable(member.kind));
}

export const roleLabels: Record<MembershipRole, string> = {
	admin: "Admin",
	member: "Member",
	viewer: "Viewer",
};

export const roleNotes: Record<MembershipRole, string> = {
	admin: "Everything a member can do, plus workspace settings, members, and teams.",
	member: "Creates and works on issues in the teams they can see.",
	viewer: "Reads and comments. Cannot create or change issues.",
};

export type MemberListing =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "results"; members: Membership[]; nextCursor?: string }
	| { kind: "no_matches"; query: string }
	| { kind: "unavailable" };

export type MemberPaging = { kind: "idle" } | { kind: "loading" } | { kind: "unavailable" };

export type MemberRemoval =
	| { kind: "closed" }
	| { kind: "loading"; accountId: string }
	| { kind: "not_in_view"; accountId: string }
	| { kind: "ready"; member: Membership; teams: MemberTeam[]; soleAdmin: boolean }
	| { kind: "removing"; member: Membership; teams: MemberTeam[]; soleAdmin: boolean }
	| { kind: "unavailable"; accountId: string };

export type MembershipFailure =
	| { kind: "last_admin"; name: string; intent: "demote" | "remove" }
	| { kind: "self_role" }
	| { kind: "directory_managed"; name: string }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type MembershipNotice =
	| { kind: "role_changed"; name: string; role: MembershipRole }
	| { kind: "removed"; name: string; reassigned: string | null };

export type MemberAction =
	| { kind: "idle" }
	| { kind: "changing_role"; accountId: string }
	| { kind: "removing"; accountId: string };

type ChangeRoleResponses = operations["changeWorkspaceMemberRole"]["responses"];
type RemoveResponses = operations["removeWorkspaceMember"]["responses"];

export type ChangeMemberRoleProblem =
	ChangeRoleResponses[401 | 403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export type RemoveMemberProblem =
	RemoveResponses[401 | 403 | 404 | 409 | 422 | 500]["content"]["application/problem+json"];

export function listingFor(page: MemberPage | undefined, query: string): MemberListing {
	if (!page) return { kind: "unavailable" };

	if (page.members.length > 0) {
		return { kind: "results", members: page.members, nextCursor: page.nextCursor };
	}

	return query ? { kind: "no_matches", query } : { kind: "empty" };
}

export function membersOf(listing: MemberListing): Membership[] {
	return listing.kind === "results" ? listing.members : [];
}

export function hasMore(listing: MemberListing): boolean {
	return listing.kind === "results" && listing.nextCursor !== undefined;
}

export function memberName(member: Membership): string {
	return member.displayName || member.email || member.accountId;
}

export function isDeactivated(member: Membership): boolean {
	return Boolean(member.deactivatedAt);
}

export function isDirectoryManaged(member: Membership): boolean {
	return member.source === "directory";
}


export function authMethodLabel(method: MemberAuthMethod | undefined): string | null {
	if (!method) return null;

	return method === "sso" ? "Single sign-on" : "Password";
}

export function roleFailureFor(
	problem: ChangeMemberRoleProblem | undefined,
	member: Membership
): MembershipFailure {
	if (problem?.status === 403) return { kind: "forbidden" };

	if (problem && "code" in problem) {
		if (problem.code === "self_role_change") return { kind: "self_role" };
		if (problem.code === "directory_managed") {
			return { kind: "directory_managed", name: memberName(member) };
		}
		if (problem.code === "last_admin") {
			return { kind: "last_admin", name: memberName(member), intent: "demote" };
		}
	}

	return { kind: "unavailable" };
}

export function removalFailureFor(
	problem: RemoveMemberProblem | undefined,
	member: Membership
): MembershipFailure {
	if (problem?.status === 403) return { kind: "forbidden" };

	if (problem && "code" in problem) {
		if (problem.code === "directory_managed") {
			return { kind: "directory_managed", name: memberName(member) };
		}
		if (problem.code === "last_admin") {
			return { kind: "last_admin", name: memberName(member), intent: "remove" };
		}
	}

	return { kind: "unavailable" };
}

export function membershipFailureTitle(failure: MembershipFailure): string {
	switch (failure.kind) {
		case "last_admin":
			return `${failure.name} is the only admin`;
		case "self_role":
			return "You cannot change your own role";
		case "directory_managed":
			return `${failure.name} is managed by your directory`;
		case "forbidden":
			return "Only admins can manage members";
		default:
			return "Something went wrong";
	}
}

export function membershipFailureMessage(failure: MembershipFailure): string {
	switch (failure.kind) {
		case "last_admin":
			return failure.intent === "demote"
				? "A workspace always needs one administrator. Make someone else an admin first, then change this."
				: "A workspace always needs one administrator. Make someone else an admin first, then remove this one.";
		case "self_role":
			return "Ask another admin to change it for you.";
		case "directory_managed":
			return "Roles and membership for this account come from your identity provider. Change them there — Norn follows what the directory says.";
		case "forbidden":
			return "Ask a workspace administrator to make this change.";
		default:
			return "Nothing changed. Wait a moment and try again.";
	}
}

export function roleMessage(code: string): string {
	switch (code) {
		case "required":
			return "Choose a role.";
		case "unsupported_value":
			return "That is not a role.";
		default:
			return "That role cannot be used.";
	}
}
