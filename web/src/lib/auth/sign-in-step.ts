import type { Client } from "openapi-fetch";
import type { components, paths } from "#lib/api/dashboard.gen.js";
import type { SignInFailure } from "./types";

export type SignInMethod = components["schemas"]["SignInMethod"];
export type SignInOption = components["schemas"]["SignInOption"];
export type SignInOptions = components["schemas"]["SignInOptions"];
export type SignInWorkspace = components["schemas"]["SignInWorkspace"];

export type SignInStep =
	| { kind: "identify" }
	| { kind: "methods"; email: string; workspace: string; options: SignInOption[] }
	| { kind: "closed"; email: string };

export type SignInOutcome = { step: SignInStep; failure?: SignInFailure };

export type MethodGroup =
	| { method: "sso"; workspaces: SignInWorkspace[] }
	| { method: "password" };

export function stepFor(options: SignInOptions, email: string, workspace = ""): SignInStep {
	if (options.options.length === 0) return { kind: "closed", email };

	return { kind: "methods", email, workspace, options: options.options };
}

export function methodGroups(options: SignInOption[]): MethodGroup[] {
	const groups: MethodGroup[] = [];

	for (const option of options) {
		if (option.method === "password") {
			groups.push({ method: "password" });
			continue;
		}

		if (!option.workspace) continue;

		const last = groups.at(-1);

		if (last?.method === "sso") last.workspaces.push(option.workspace);
		else groups.push({ method: "sso", workspaces: [option.workspace] });
	}

	return groups;
}

export function soleProvider(options: SignInOptions): SignInWorkspace | null {
	if (options.preferred !== "sso") return null;

	const workspaces = options.options.flatMap((option) => (option.workspace ? [option.workspace] : []));

	return workspaces.length === 1 ? workspaces[0] : null;
}

export function providerLabel(workspace: SignInWorkspace, among: number): string {
	const provider = workspace.host ?? "single sign-on";

	return among > 1 ? `${workspace.name} · ${provider}` : `Continue with ${provider}`;
}

export function beginSso(
	api: Client<paths>,
	workspace: Pick<SignInWorkspace, "workspace" | "protocol">,
	returnTo: string,
	loginHint = ""
) {
	const body = {
		workspace: workspace.workspace,
		returnTo: returnTo && returnTo !== "/" ? returnTo : undefined,
		loginHint: loginHint || undefined,
	};

	return workspace.protocol === "saml"
		? api.POST("/sso/saml/login", { body })
		: api.POST("/sso/oidc/login", { body });
}

export type Discovery =
	| { kind: "found"; options: SignInOptions }
	| { kind: "invalid" }
	| { kind: "failed"; failure: SignInFailure };

export async function discoverSignIn(api: Client<paths>, email: string): Promise<Discovery> {
	try {
		const { data, response } = await api.POST("/auth/discover", { body: { email } });

		if (data) return { kind: "found", options: data };
		if (response.status === 422) return { kind: "invalid" };
		if (response.status === 429) return { kind: "failed", failure: { kind: "rate_limited" } };

		return { kind: "failed", failure: { kind: "unavailable" } };
	} catch {
		return { kind: "failed", failure: { kind: "unavailable" } };
	}
}
