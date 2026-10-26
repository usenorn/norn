import { exchangeFrom } from "#lib/auth/sso.js";
import { keys } from "#lib/api/keys.js";
import { beginSso } from "#lib/auth/sign-in-step.js";
import { reachWorkspaceSignIn } from "#lib/auth/workspace-sign-in.js";
import type { SsoExchange } from "#lib/auth/types.js";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	url,
}): Promise<{ exchange: SsoExchange; workspace: string; provider: string }> => {
	depends(keys.page(route.id));

	const workspace = url.searchParams.get("workspace") ?? "";

	const entry = workspace
		? await reachWorkspaceSignIn(locals.api, workspace)
		: ({ kind: "none" } as const);

	const workspaceProvider =
		entry.kind === "ready"
			? (entry.signIn.options.find((option) => option.method === "sso")?.workspace ?? null)
			: null;
	const provider = workspaceProvider?.host ?? "";

	const returned = exchangeFrom(url);
	if (returned) return { exchange: returned, workspace, provider };

	if (!workspace) {
		return {
			exchange: {
				status: "failed",
				failure: {
					kind: "stage",
					stage: "discovery",
					message: "This sign-in link does not say which workspace to sign into.",
					diagnostics: [{ key: "workspace", value: "missing" }],
					reference: "",
				},
			},
			workspace,
			provider,
		};
	}

	const { data, error } = await beginSso(
		locals.api,
		{ workspace, protocol: workspaceProvider?.protocol },
		url.searchParams.get("return_to") ?? ""
	);

	if (error) {
		return {
			exchange: {
				status: "failed",
				failure:
					"code" in error && error.code === "sso_failed"
						? {
								kind: "stage",
								stage: error.stage,
								message: error.detail ?? "This workspace could not start a sign-in.",
								diagnostics: [{ key: "workspace", value: workspace }],
								reference: "",
							}
						: {
								kind: "stage",
								stage: "discovery",
								message: "This workspace does not use single sign-on.",
								diagnostics: [{ key: "workspace", value: workspace }],
								reference: "",
							},
			},
			workspace,
			provider,
		};
	}

	return {
		exchange: { status: "pending", phase: "redirecting", authorizationUrl: data.authorizationUrl },
		workspace,
		provider,
	};
};
