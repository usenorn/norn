import type { Diagnostic, SsoExchange, SsoFailure, SsoStage } from "$lib/auth/types";

const stages: SsoStage[] = [
	"discovery",
	"endpoints",
	"jwks",
	"authorization",
	"token_exchange",
	"id_token",
	"claims",
	"matching",
	"provisioning",
];

const stageTitles: Record<SsoStage, string> = {
	discovery: "Your provider could not be reached",
	endpoints: "Your provider is not set up correctly",
	jwks: "The signing keys could not be read",
	authorization: "Your provider wouldn't let you in",
	token_exchange: "Norn and your provider disagree",
	id_token: "The response could not be trusted",
	claims: "Your provider left something out",
	matching: "Signed in, but there's no account here",
	provisioning: "Signed in, but there's no account here",
};

const stageAdvice: Record<SsoStage, string[]> = {
	discovery: [
		"Confirm the issuer URL in Settings → Authentication.",
		"Confirm this instance can reach your provider over the network.",
	],
	endpoints: [
		"An administrator should re-run Discover in Settings → Authentication.",
		"Every endpoint has to be an https address.",
	],
	jwks: [
		"Confirm the JWKS URI is reachable from this instance.",
		"Some providers block requests from unknown networks.",
	],
	authorization: [
		"Are you assigned to the Norn app in your provider?",
		"If you're signed into two accounts, sign out of the other one first.",
	],
	token_exchange: [
		"An administrator should re-enter the client secret.",
		"The redirect URI registered with your provider has to match Norn's exactly.",
	],
	id_token: [
		"Confirm the client ID matches the one your provider issued.",
		"Confirm the issuer in Norn matches the one in the token.",
	],
	claims: [
		"Your provider has to release an email address to Norn.",
		"An administrator should check the scopes and claim mappings.",
	],
	matching: [
		"An admin invites you to this workspace and you sign in again.",
		"Nothing was created. You can close this tab safely.",
	],
	provisioning: [
		"An admin invites you and you sign in again.",
		"Or an admin turns on just-in-time provisioning in Settings → Authentication.",
		"Nothing was created. You can close this tab safely.",
	],
};

function isStage(value: string): value is SsoStage {
	return (stages as string[]).includes(value);
}

export function stageTitle(stage: SsoStage): string {
	return stageTitles[stage];
}

export function stageFixes(stage: SsoStage): string[] {
	return stageAdvice[stage];
}

export function exchangeFrom(url: URL): SsoExchange | null {
	const stage = url.searchParams.get("stage");

	if (!stage || !isStage(stage)) return null;

	const message = url.searchParams.get("message") ?? "";
	const providerMessage = url.searchParams.get("provider_message");
	const subject = url.searchParams.get("subject");
	const reference = url.searchParams.get("reference") ?? "";

	const diagnostics: Diagnostic[] = [{ key: "stage", value: stage }];

	if (subject) diagnostics.push({ key: "subject", value: subject });
	if (providerMessage) diagnostics.push({ key: "provider", value: providerMessage });

	const failure: SsoFailure =
		subject && (stage === "matching" || stage === "provisioning")
			? { kind: "no_account", subject, diagnostics, reference }
			: { kind: "stage", stage, message, diagnostics, reference };

	return { status: "failed", failure };
}

export function referenceLine(reference: string): string {
	return reference ? `Reference ${reference}` : "";
}
