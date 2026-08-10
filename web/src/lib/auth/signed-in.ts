import { redirect } from "@sveltejs/kit";
import { safeReturn } from "./return-to";

export function addingAccount(url: URL): boolean {
	return url.searchParams.get("add") === "1";
}

// Reaching a sign-in form while already signed in is how a second account is added, so the bounce
// only applies when nothing asked for it — and it honours where the caller was going.
export async function leaveIfSignedIn(locals: App.Locals, url: URL): Promise<void> {
	if (addingAccount(url)) return;

	const accounts = await locals.signedIn;

	if (accounts.length > 0) redirect(307, safeReturn(url.searchParams.get("return")));
}
