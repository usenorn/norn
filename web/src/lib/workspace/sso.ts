import type { components, operations } from "$lib/api/dashboard.gen";

export type OidcConnection = components["schemas"]["WorkspaceOidcConnection"];
export type OidcEndpoints = components["schemas"]["OidcEndpoints"];
export type SsoStage = components["schemas"]["SsoStage"];
export type SsoProtocol = components["schemas"]["SsoProtocol"];

type SaveResponses = operations["setWorkspaceOidcConnection"]["responses"];

type SsoProblem = components["schemas"]["SsoProblem"];

export type SaveProblem = SaveResponses[403 | 422 | 500]["content"]["application/problem+json"];

export type SsoConfiguration =
	| { kind: "loading" }
	| { kind: "unconfigured" }
	| { kind: "configured"; connection: OidcConnection }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SsoFailure =
	| { kind: "stage"; stage: SsoStage; message: string; providerMessage?: string }
	| { kind: "forbidden" }
	| { kind: "unavailable"; detail?: string };

export type SsoOutcome =
	| { kind: "idle" }
	| { kind: "saved" }
	| { kind: "verified" }
	| { kind: "removed" }
	| { kind: "failed"; failure: SsoFailure };

export const stageLabels: Record<SsoStage, string> = {
	discovery: "Reading the discovery document",
	endpoints: "Checking the endpoints",
	jwks: "Fetching the signing keys",
	authorization: "At your provider's sign-in page",
	token_exchange: "Exchanging the sign-in code",
	id_token: "Verifying the ID token",
	claims: "Reading the claims",
	matching: "Matching the identity to a member",
	provisioning: "Creating the account",
	metadata: "Reading the provider metadata",
	certificate: "Reading the signing certificate",
	request: "Building the sign-in request",
	response: "Reading the provider's response",
	signature: "Verifying the signature",
	conditions: "Checking the assertion's validity window",
	replay: "Checking the assertion has not been used",
	attributes: "Reading the attributes",
};

export const stageAdvice: Record<SsoStage, string> = {
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
	metadata:
		"Norn could not read the provider metadata. Check the URL, and that this instance can reach it.",
	certificate:
		"The signing certificate could not be read, or it has expired. Copy the current one from your provider.",
	request: "Norn could not build a sign-in request. Check the sign-in URL.",
	response: "The provider's response could not be parsed as SAML.",
	signature:
		"The response was unsigned, or was not signed by the certificate configured here. Norn will not accept either.",
	conditions:
		"The assertion arrived outside the window it is valid for. This is almost always clock skew between this instance and the provider.",
	replay: "That assertion has already been used. Each one is accepted exactly once.",
	attributes:
		"The assertion did not carry an email address. Check the attribute mapping and what the provider releases.",
};

function isSsoProblem(problem: SaveProblem): problem is SsoProblem {
	return "code" in problem && problem.code === "sso_failed";
}

export function saveFailure(problem: SaveProblem): SsoFailure {
	if (problem.status === 403) return { kind: "forbidden" };

	if (isSsoProblem(problem)) {
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

export type SamlConnection = components["schemas"]["WorkspaceSamlConnection"];
export type SamlDescriptor = components["schemas"]["SamlDescriptor"];

export type SsoProviderConfiguration =
	| { kind: "loading" }
	| { kind: "unconfigured" }
	| { kind: "oidc"; connection: OidcConnection }
	| { kind: "saml"; connection: SamlConnection }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export const expiryWarningDays = 30;

export function certificateUrgent(daysLeft: number | undefined): boolean {
	return daysLeft !== undefined && daysLeft <= expiryWarningDays;
}

export function certificateLine(daysLeft: number | undefined): string {
	if (daysLeft === undefined) return "";
	if (daysLeft < 0) return `Expired ${Math.abs(daysLeft)} days ago`;
	if (daysLeft === 0) return "Expires today";
	if (daysLeft === 1) return "Expires tomorrow";

	return `Expires in ${daysLeft} days`;
}

export function certificateAdvice(daysLeft: number | undefined): string {
	if (daysLeft === undefined) return "";
	if (daysLeft < 0) {
		return "Nobody can sign in through this provider until the certificate is replaced.";
	}

	return "When it expires nobody will be able to sign in through this provider.";
}
