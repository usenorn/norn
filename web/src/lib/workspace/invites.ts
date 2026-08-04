import type { components } from "$lib/api/dashboard.gen";
import type { MembershipRole } from "./members";

export type InviteStatus =
	| "pending"
	| "invalid"
	| "existing_member"
	| "sent"
	| "failed"
	| "link_only";

export type Invite = {
	email: string;
	role: MembershipRole;
	teamIds: string[];
	status: InviteStatus;
	invitationId?: string;
	url?: string;
};

export type InviteBatch =
	| { kind: "composing" }
	| { kind: "results"; rows: Invite[] }
	| { kind: "unavailable" };

type InvitationResult = components["schemas"]["InvitationResult"];
type InvitationDelivery = components["schemas"]["InvitationDelivery"];

const deliveryStatus: Record<InvitationDelivery, InviteStatus> = {
	pending: "pending",
	sent: "sent",
	failed: "failed",
	link_only: "link_only",
};

export function inviteFromResult(
	result: InvitationResult,
	role: MembershipRole,
	teamIds: string[]
): Invite {
	if (result.outcome === "invalid_email") {
		return { email: result.email, role, teamIds, status: "invalid" };
	}

	if (result.outcome === "already_member") {
		return { email: result.email, role, teamIds, status: "existing_member" };
	}

	return {
		email: result.email,
		role: result.invitation?.role ?? role,
		teamIds: result.invitation?.teamIds ?? teamIds,
		status: result.invitation ? deliveryStatus[result.invitation.delivery] : "pending",
		invitationId: result.invitation?.id,
		url: result.url,
	};
}

export function parseAddresses(text: string): string[] {
	const seen = new Set<string>();
	const addresses: string[] = [];
	for (const raw of text.split(/[\s,;]+/)) {
		const address = raw.trim();
		if (!address) continue;
		const key = address.toLowerCase();
		if (seen.has(key)) continue;
		seen.add(key);
		addresses.push(address);
	}
	return addresses;
}

export function isEmailAddress(value: string): boolean {
	const at = value.indexOf("@");
	return at > 0 && at === value.lastIndexOf("@") && at < value.length - 1;
}
