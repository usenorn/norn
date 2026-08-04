import { apiFor } from "$lib/api";
import { exchangeFrom } from "$lib/auth/sso";
import type { SsoExchange } from "$lib/auth/types";
import type { PageLoad } from "./$types";

export const load: PageLoad = async ({
	fetch,
	url,
}): Promise<{ exchange: SsoExchange; workspace: string }> => {
	const workspace = url.searchParams.get("workspace") ?? "";

	const returned = exchangeFrom(url);
	if (returned) return { exchange: returned, workspace };

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
		};
	}

	const api = apiFor(url);

	const { data, error } = await api.POST("/sso/oidc/login", {
		fetch,
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
		};
	}

	return {
		exchange: { status: "pending", phase: "redirecting", authorizationUrl: data.authorizationUrl },
		workspace,
	};
};
