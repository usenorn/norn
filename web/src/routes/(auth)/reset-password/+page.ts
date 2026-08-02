import type { PasswordReset } from "$lib/auth/types";
import type { PageLoad } from "./$types";

export const load: PageLoad = ({ url }): { token: string | null; reset: PasswordReset } => {
	const token = url.searchParams.get("token");

	return { token, reset: token ? { kind: "form" } : { kind: "request" } };
};
