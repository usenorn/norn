import type { components } from "$lib/api/dashboard.gen";

export type TeamMember = components["schemas"]["TeamMember"];

export type TeamRoster =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; members: TeamMember[] }
	| { kind: "added"; members: TeamMember[]; member: TeamMember }
	| { kind: "unavailable" };

export type MemberFailure =
	| { kind: "already_member" }
	| { kind: "not_in_workspace" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function rosterFor(members: TeamMember[] | undefined): TeamRoster {
	if (!members) return { kind: "unavailable" };
	if (members.length === 0) return { kind: "empty" };

	return { kind: "ready", members };
}

export function membersOf(roster: TeamRoster): TeamMember[] {
	return "members" in roster ? roster.members : [];
}

export function memberFailureMessage(failure: MemberFailure): string {
	switch (failure.kind) {
		case "already_member":
			return "They are already on this team.";
		case "not_in_workspace":
			return "That person is not in this workspace yet. Invite them first.";
		case "forbidden":
			return "Only workspace admins can change who is on a team.";
		default:
			return "Nothing changed. Wait a moment and try again.";
	}
}

export function initialsOf(name: string): string {
	return name
		.trim()
		.split(/\s+/)
		.slice(0, 2)
		.map((part) => part[0])
		.join("");
}
