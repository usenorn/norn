import type { components, operations } from "$lib/api/dashboard.gen";
import { workspacePath } from "$lib/workspace/navigation";

export type SourceControlConnection = components["schemas"]["SourceControlConnection"];
export type MintedSourceControlConnection =
	components["schemas"]["MintedSourceControlConnection"];
export type SourceControlProvider = components["schemas"]["SourceControlProvider"];
export type SourceControlStatus = components["schemas"]["SourceControlStatus"];
export type SourceControlBrokenReason = components["schemas"]["SourceControlBrokenReason"];
export type TeamSourceControlSettings = components["schemas"]["TeamSourceControlSettings"];
export type CodeLink = components["schemas"]["CodeLink"];
export type CodeLinkKind = components["schemas"]["CodeLinkKind"];
export type CodeChangeState = components["schemas"]["CodeChangeState"];
export type IssueMirror = components["schemas"]["IssueMirror"];
export type SourceControlDelivery = components["schemas"]["SourceControlDelivery"];
export type SourceControlDeliveryOutcome =
	components["schemas"]["SourceControlDeliveryOutcome"];

type ConnectResponses = operations["connectWorkspaceSourceControl"]["responses"];

export type SourceControlProblem =
	ConnectResponses[403 | 404 | 409 | 422 | 429 | 500 | 503]["content"]["application/problem+json"];

export type SourceControlView =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "list"; connections: SourceControlConnection[] }
	| {
			kind: "connected";
			connections: SourceControlConnection[];
			connection: SourceControlConnection;
			webhookUrl: string;
			webhookSecret: string;
	  }
	| { kind: "sealing_unavailable" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SourceControlDetailView =
	| { kind: "loading" }
	| {
			kind: "detail";
			connection: SourceControlConnection;
			links: CodeLink[];
			deliveries: SourceControlDelivery[];
	  }
	| { kind: "not_found" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SourceControlFailure =
	| { kind: "credentials_rejected" }
	| { kind: "repository_unreachable" }
	| { kind: "destination_refused" }
	| { kind: "provider_unsupported" }
	| { kind: "already_connected" }
	| { kind: "already_mirrored" }
	| { kind: "team_outside_connection" }
	| { kind: "sealing_unavailable" }
	| { kind: "rate_limited" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function sourceControlFailure(problem: SourceControlProblem): SourceControlFailure {
	if (problem && "code" in problem) {
		switch (problem.code) {
			case "source_control_credentials_rejected":
				return { kind: "credentials_rejected" };
			case "source_control_repository_unreachable":
				return { kind: "repository_unreachable" };
			case "source_control_destination_refused":
				return { kind: "destination_refused" };
			case "source_control_provider_unsupported":
				return { kind: "provider_unsupported" };
			case "source_control_already_connected":
				return { kind: "already_connected" };
			case "source_control_already_mirrored":
				return { kind: "already_mirrored" };
			case "source_control_team_outside_connection":
				return { kind: "team_outside_connection" };
			case "source_control_sealing_unavailable":
				return { kind: "sealing_unavailable" };
			case "forbidden":
				return { kind: "forbidden" };
		}
	}

	if (problem?.status === 429) return { kind: "rate_limited" };
	if (problem?.status === 403) return { kind: "forbidden" };

	return { kind: "unavailable" };
}

// detailOf prefers what the server actually said. Its detail names the repository and the
// exact refusal; the canned copy below cannot, and reading only that is what turned a wrong
// repository name into a long hunt through token scopes.
export function detailOf(
	problem: SourceControlProblem,
	failure: SourceControlFailure,
): string {
	const detail = problem?.detail?.trim();

	if (detail) return detail;

	return failureMessage(failure);
}

export function failureMessage(failure: SourceControlFailure): string {
	switch (failure.kind) {
		case "credentials_rejected":
			return "The token was refused. Check it has not expired and that it can read this repository.";
		case "repository_unreachable":
			return (
				"The token works, but that repository is not visible to it. Check the name is " +
				"owner/repository, and that the token reaches it — a fine-grained token has to " +
				"name the repository under Repository access, and a classic one needs the repo scope."
			);
		case "destination_refused":
			return "That address is not reachable from this instance.";
		case "provider_unsupported":
			return "This instance does not support that platform.";
		case "already_connected":
			return "This workspace is already connected to that repository.";
		case "already_mirrored":
			return "That issue is already paired with another one.";
		case "team_outside_connection":
			return "This issue belongs to a team the connection does not serve.";
		case "sealing_unavailable":
			return "An operator must set NORN_SECURITY_ENCRYPTION_KEY before a token can be stored.";
		case "rate_limited":
			return "The platform is rate limiting this instance. Try again shortly.";
		case "forbidden":
			return "You need to be an administrator of this workspace.";
		case "unavailable":
			return "Source control could not be reached.";
	}
}

export function providerLabel(provider: SourceControlProvider): string {
	return provider === "github" ? "GitHub" : "GitLab";
}

const brokenReasons: Record<SourceControlBrokenReason, string> = {
	credentials_rejected: "the token was refused",
	repository_unreachable: "the repository could not be reached",
	hook_missing: "the webhook is missing on the platform",
};

export function brokenLabel(connection: SourceControlConnection): string {
	if (!connection.brokenReason) return "the platform stopped answering";

	return brokenReasons[connection.brokenReason];
}

const changeStates: Record<CodeChangeState, string> = {
	open: "Open",
	draft: "Draft",
	in_review: "In review",
	merged: "Merged",
	closed: "Closed",
};

export function changeStateLabel(state: CodeChangeState): string {
	return changeStates[state];
}

const linkKinds: Record<CodeLinkKind, string> = {
	branch: "Branch",
	commit: "Commit",
	change: "Change",
};

export function linkKindLabel(kind: CodeLinkKind): string {
	return linkKinds[kind];
}

export function linkTitle(link: CodeLink): string {
	if (link.title) return link.title;
	if (link.number) return `${link.repository}#${link.number}`;

	return `${link.repository} ${link.externalId}`;
}

const deliveryOutcomes: Record<SourceControlDeliveryOutcome, string> = {
	applied: "Acted on",
	ignored: "Nothing to do",
	failed: "Failed",
	processed: "Handled",
};

export function deliveryOutcomeLabel(delivery: SourceControlDelivery): string {
	if (!delivery.outcome) return delivery.processedAt ? "Done" : "Waiting";

	return deliveryOutcomes[delivery.outcome];
}

export function sourceControlPath(slug: string): string {
	return workspacePath(slug, "/settings/source-control");
}

export function sourceControlConnectionPath(slug: string, connectionId: string): string {
	return workspacePath(slug, `/settings/source-control/${connectionId}`);
}
