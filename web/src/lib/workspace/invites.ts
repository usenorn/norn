export const inviteRoles = ["Member", "Admin", "Guest"] as const;

export type InviteRole = (typeof inviteRoles)[number];

export type InviteStatus =
	| "pending"
	| "invalid"
	| "existing_member"
	| "sent"
	| "failed"
	| "link_only";

export type Invite = {
	email: string;
	role: InviteRole;
	status: InviteStatus;
};

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

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
	return emailPattern.test(value);
}
