import type { components, operations } from "$lib/api/dashboard.gen";

export type OidcConnection = components["schemas"]["WorkspaceOidcConnection"];
export type OidcEndpoints = components["schemas"]["OidcEndpoints"];
export type OidcStage = components["schemas"]["OidcStage"];

type SaveResponses = operations["setWorkspaceOidcConnection"]["responses"];

type OidcProblem = components["schemas"]["OidcProblem"];

export type SaveProblem = SaveResponses[403 | 422 | 500]["content"]["application/problem+json"];

export type SsoConfiguration =
	| { kind: "loading" }
	| { kind: "unconfigured" }
	| { kind: "configured"; connection: OidcConnection }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SsoFailure =
	| { kind: "stage"; stage: OidcStage; message: string; providerMessage?: string }
	| { kind: "forbidden" }
	| { kind: "unavailable"; detail?: string };

export type SsoOutcome =
	| { kind: "idle" }
	| { kind: "saved" }
	| { kind: "verified" }
	| { kind: "removed" }
	| { kind: "failed"; failure: SsoFailure };

export const stageLabels: Record<OidcStage, string> = {
	discovery: "Reading the discovery document",
	endpoints: "Checking the endpoints",
	jwks: "Fetching the signing keys",
	authorization: "At your provider's sign-in page",
	token_exchange: "Exchanging the sign-in code",
	id_token: "Verifying the ID token",
	claims: "Reading the claims",
	matching: "Matching the identity to a member",
	provisioning: "Creating the account",
};

export const stageAdvice: Record<OidcStage, string> = {
	discovery:
		"Norn could not read /.well-known/openid-configuration at that issuer. Check the URL, and that this instance can reach it.",
	endpoints: "One of the endpoints is missing or is not an https address.",
	jwks: "The signing keys could not be fetched, so no ID token can be verified.",
	authorization: "Your provider refused before Norn was involved. Check that the app is assigned to you.",
	token_exchange:
		"The provider would not swap the code for a token. This is almost always the client secret or the redirect URI.",
	id_token: "The token came back but did not verify. Check the client ID and that the issuer matches.",
	claims: "The token verified but did not carry what Norn needs. Check the scopes and claim mappings.",
	matching: "The identity is genuine but is not a member of this workspace.",
	provisioning: "There is no account for that address and just-in-time provisioning is off.",
};

function isOidcProblem(problem: SaveProblem): problem is OidcProblem {
	return "code" in problem && problem.code === "oidc_failed";
}

export function saveFailure(problem: SaveProblem): SsoFailure {
	if (problem.status === 403) return { kind: "forbidden" };

	if (isOidcProblem(problem)) {
		return {
			kind: "stage",
			stage: problem.stage,
			message: problem.detail ?? stageAdvice[problem.stage],
			providerMessage: problem.providerMessage,
		};
	}

	return { kind: "unavailable", detail: problem.detail };
}

export function failureMessage(failure: SsoFailure): string {
	switch (failure.kind) {
		case "stage":
			return failure.message;
		case "forbidden":
			return "You may not change how this workspace signs people in.";
		case "unavailable":
			return failure.detail ?? "We could not reach the server. Nothing changed.";
	}
}

export function failureTitle(failure: SsoFailure): string {
	if (failure.kind !== "stage") return "Something went wrong";

	return stageLabels[failure.stage];
}

export function endpointsFrom(connection: OidcConnection | null): OidcEndpoints {
	return (
		connection?.endpoints ?? {
			issuer: "",
			authorizationEndpoint: "",
			tokenEndpoint: "",
			jwksUri: "",
		}
	);
}

export function scopeText(scopes: string[]): string {
	return scopes.join(" ");
}

export function parseScopes(text: string): string[] {
	return text.split(/[\s,]+/).filter(Boolean);
}
