import type { components } from "$lib/api/dashboard.gen";

export type SignedInAccount = components["schemas"]["SignedInAccount"];
export type SignedInWorkspace = components["schemas"]["SignedInWorkspace"];
export type Account = components["schemas"]["Account"];

export type ActingSession = {
	slot: string;
	accountId: string;
};

export const sessionHeader = "x-norn-session";
export const sessionParam = "s";

export function accountOfSlot(
	accounts: SignedInAccount[],
	slot: string | null
): SignedInAccount | undefined {
	if (!slot) return undefined;

	return accounts.find(
		(signedIn) =>
			signedIn.defaultSlot === slot ||
			signedIn.workspaces.some((workspace) => workspace.slot === slot)
	);
}

// Two signed-in accounts can both be members of one workspace. `preferred` is how the switcher
// says which of them the person picked; without it the earliest signed in acts, deterministically.
export function reachOfSlug(
	accounts: SignedInAccount[],
	slug: string,
	preferred?: string | null
): { account: SignedInAccount; workspace: SignedInWorkspace } | undefined {
	const reaching = accounts.flatMap((account) =>
		account.workspaces
			.filter((workspace) => workspace.workspace.slug === slug)
			.map((workspace) => ({ account, workspace }))
	);

	return reaching.find((reach) => reach.workspace.slot === preferred) ?? reaching[0];
}

export function slotForSlug(accounts: SignedInAccount[], slug: string): string | null {
	return reachOfSlug(accounts, slug)?.workspace.slot ?? null;
}

export function defaultSlot(accounts: SignedInAccount[]): string | null {
	const current = accounts.find((account) => account.current);

	return (current ?? accounts[0])?.defaultSlot ?? null;
}

// A selector that names no signed-in session must fall back rather than resolve to nothing: a
// route that redirects to sign-in on an unresolved session, and a sign-in that redirects back to
// where it came from, otherwise bounce between each other for as long as the slot stays in the URL.
export function actingOf(
	accounts: SignedInAccount[],
	url: URL,
	slug: string | undefined
): ActingSession | undefined {
	const named = url.searchParams.get(sessionParam);
	const reached = slug ? reachOfSlug(accounts, slug, named)?.workspace.slot : undefined;
	const slot = reached ?? (accountOfSlot(accounts, named) ? named : null) ?? defaultSlot(accounts);
	const account = accountOfSlot(accounts, slot);

	return slot && account ? { slot, accountId: account.account.id } : undefined;
}

export function initialsOf(name: string): string {
	const parts = name.trim().split(/\s+/).filter(Boolean);

	if (parts.length === 0) return "?";

	return parts
		.slice(0, 2)
		.map((part) => part[0]?.toUpperCase() ?? "")
		.join("");
}

export function withSlot(path: string, slot: string | null): string {
	if (!slot) return path;

	const [base, query] = path.split("?");
	const params = new URLSearchParams(query);

	params.set(sessionParam, slot);

	return `${base}?${params}`;
}
