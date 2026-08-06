import { exchangeFrom } from "$lib/auth/sso";
import { reachWorkspaceSignIn } from "$lib/auth/workspace-sign-in";
import type { SsoExchange } from "$lib/auth/types";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({
	locals,
	url,
}): Promise<{ exchange: SsoExchange; workspace: string; provider: string }> => {
	const workspace = url.searchParams.get("workspace") ?? "";

	const entry = workspace
		? await reachWorkspaceSignIn(locals.api, workspace)
		: ({ kind: "none" } as const);

	const provider = entry.kind === "ready" ? (entry.signIn.host ?? "") : "";

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

	const { data, error } = await locals.api.POST("/sso/oidc/login", {
		body: { workspace, returnTo: url.searchParams.get("return_to") ?? undefined },
	});

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
