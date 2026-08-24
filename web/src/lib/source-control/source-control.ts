import type { components, operations } from "$lib/api/dashboard.gen";
import { workspacePath } from "$lib/workspace/navigation";

export type SourceControlConnection = components["schemas"]["SourceControlConnection"];
export type SourceControlRepository = components["schemas"]["SourceControlRepository"];
export type MintedSourceControlRepository =
	components["schemas"]["MintedSourceControlRepository"];
export type SourceControlRoute = components["schemas"]["SourceControlRoute"];
export type SourceControlTransitionRule =
	components["schemas"]["SourceControlTransitionRule"];
export type SourceControlProvider = components["schemas"]["SourceControlProvider"];
export type SourceControlStatus = components["schemas"]["SourceControlStatus"];
export type SourceControlBrokenReason = components["schemas"]["SourceControlBrokenReason"];
export type CodeLink = components["schemas"]["CodeLink"];
export type CodeLinkKind = components["schemas"]["CodeLinkKind"];
export type CodeChangeState = components["schemas"]["CodeChangeState"];
export type IssueMirror = components["schemas"]["IssueMirror"];
export type SourceControlDelivery = components["schemas"]["SourceControlDelivery"];
export type CodeChecks = components["schemas"]["CodeChecks"];
export type ReviewVerdict = components["schemas"]["ReviewVerdict"];
export type TeamSourceControlSettings =
	components["schemas"]["TeamSourceControlSettings"];
export type MirrorDirection = components["schemas"]["MirrorDirection"];
export type SCMIdentity = components["schemas"]["SCMIdentity"];
export type MirrorConflict = components["schemas"]["MirrorConflict"];
export type SourceControlCapability = components["schemas"]["SourceControlCapability"];
export type SourceControlApplication = components["schemas"]["SourceControlApplication"];
export type SourceControlAppRegistration =
	components["schemas"]["SourceControlAppRegistration"];
export type SourceControlInstallation = components["schemas"]["SourceControlInstallation"];
export type AvailableSourceControlRepository =
	components["schemas"]["AvailableSourceControlRepository"];
export type IssueShipping = components["schemas"]["IssueShipping"];
export type SCMRelease = components["schemas"]["SCMRelease"];
export type SCMDeployment = components["schemas"]["SCMDeployment"];
export type SourceControlDeliveryOutcome =
	components["schemas"]["SourceControlDeliveryOutcome"];

type ConnectResponses = operations["connectWorkspaceSourceControl"]["responses"];

export type SourceControlProblem =
	ConnectResponses[403 | 404 | 409 | 422 | 429 | 500 | 503]["content"]["application/problem+json"];

export type SourceControlView =
	| { kind: "loading" }
	| { kind: "empty" }
	| {
			kind: "list";
			connections: SourceControlConnection[];
			repositories: SourceControlRepository[];
			/** True when the repository listing itself failed, which is not the same as none. */
			repositoriesUnavailable?: boolean;
	  }
	| { kind: "sealing_unavailable" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SourceControlDetailView =
	| { kind: "loading" }
	| {
			kind: "detail";
			connection: SourceControlConnection;
			repositories: SourceControlRepository[];
	  }
	| { kind: "not_found" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SourceControlRepositoryView =
	| { kind: "loading" }
	| {
			kind: "detail";
			connection: SourceControlConnection;
			repository: SourceControlRepository;
			routes: SourceControlRoute[];
			deliveries: SourceControlDelivery[];
	  }
	| { kind: "not_found" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type MintedRepository = {
	repository: SourceControlRepository;
	webhookUrl: string;
	webhookSecret: string;
};

export type SourceControlAppState =
	| { kind: "unsupported" }
	| { kind: "unregistered"; canRegister: boolean }
	| { kind: "registered"; slug: string; installUrl: string }
	| {
			kind: "choosing";
			handle: string;
			installUrl: string;
			installations: SourceControlInstallation[];
	  };

export type SourceControlAppNotice =
	| { kind: "registered" }
	| { kind: "expired" }
	| { kind: "refused" }
	| { kind: "unregistered" }
	| { kind: "exists" }
	| { kind: "unavailable" };

export function appNoticeMessage(notice: SourceControlAppNotice): string {
	switch (notice.kind) {
		case "registered":
			return "The application is registered. Install it on the repositories you want Norn to watch, then sign in to choose the installation.";
		case "expired":
			return "That took too long, or it had already been used. Start again.";
		case "refused":
			return "GitHub refused the exchange. Nothing was stored — start again.";
		case "unregistered":
			return "No application is registered on this instance yet.";
		case "exists":
			return "This instance already holds a GitHub application, and it serves every workspace on it. Nothing was changed — the application you just created on GitHub can be deleted there.";
		case "unavailable":
			return "GitHub could not be reached. Nothing was changed.";
	}
}

export function appNoticeFrom(value: string | null): SourceControlAppNotice | undefined {
	switch (value) {
		case "expired":
		case "refused":
		case "unregistered":
		case "exists":
		case "unavailable":
			return { kind: value };
		default:
			return undefined;
	}
}

export type SourceControlFailure =
	| { kind: "credentials_rejected" }
	| { kind: "repository_unreachable" }
	| { kind: "destination_refused" }
	| { kind: "provider_unsupported" }
	| { kind: "already_connected" }
	| { kind: "already_routed" }
	| { kind: "identity_mapped" }
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
			case "source_control_already_routed":
				return { kind: "already_routed" };
			case "source_control_identity_mapped":
				return { kind: "identity_mapped" };
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
			return "This workspace already holds a credential for that forge.";
		case "already_routed":
			return "That team already has a route for this path.";
		case "identity_mapped":
			return "That platform account is already mapped to somebody here.";
		case "already_mirrored":
			return "That issue is already paired with another one.";
		case "team_outside_connection":
			return "This issue belongs to a team no route sends this repository's changes to.";
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

const providers: Record<SourceControlProvider, string> = {
	github: "GitHub",
	gitlab: "GitLab",
	gitea: "Gitea or Forgejo",
};

export function providerLabel(provider: SourceControlProvider): string {
	return providers[provider];
}

const capabilities: Record<SourceControlCapability, string> = {
	webhooks: "Webhooks",
	reviews: "Reviews",
	checks: "Checks",
	changed_paths: "Which files a change touches",
	issues: "Issue sync",
	labels: "Labels",
	assignees: "Assignees",
	releases: "Releases",
	deployments: "Deployments",
};

const deploymentStates: Record<SCMDeployment["state"], string> = {
	pending: "waiting",
	running: "deploying",
	succeeded: "live",
	failed: "failed",
	inactive: "stopped",
};

export function deploymentLabel(deployment: SCMDeployment): string {
	return `${deployment.environment} — ${deploymentStates[deployment.state]}`;
}

export function releaseLabel(release: SCMRelease): string {
	return release.prerelease ? `${release.name} (prerelease)` : release.name;
}

export function capabilityLabel(capability: SourceControlCapability): string {
	return capabilities[capability];
}

export function requiresAddress(provider: SourceControlProvider): boolean {
	return provider === "gitea";
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
	draft: "Draft",
	open: "Open",
	review_requested: "Review requested",
	changes_requested: "Changes requested",
	approved: "Approved",
	merged: "Merged",
	closed: "Closed",
	reopened: "Reopened",
	conflicted: "Conflicted",
};

export const changeStateOrder: CodeChangeState[] = [
	"draft",
	"open",
	"review_requested",
	"changes_requested",
	"approved",
	"merged",
	"closed",
	"reopened",
	"conflicted",
];

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

const checkStates: Record<CodeChecks, string> = {
	pending: "Checks running",
	passing: "Checks passing",
	failing: "Checks failing",
};

const directions: Record<MirrorDirection, string> = {
	inbound: "Read only — bring issues here, never write back",
	outbound: "Write only — send issues out, never take them",
	both: "Both ways",
};

export function directionLabel(direction: MirrorDirection): string {
	return directions[direction];
}

export const directionOrder: MirrorDirection[] = ["both", "inbound", "outbound"];

export function conflictSummary(conflict: MirrorConflict): string {
	return conflict.winner === "source"
		? "The platform's edit was kept; yours is below."
		: "Your edit was kept; the platform's is below.";
}

export function checksLabel(checks: CodeChecks): string {
	return checkStates[checks];
}

const reviewVerdicts: Record<ReviewVerdict, string> = {
	requested: "asked",
	commented: "commented",
	approved: "approved",
	changes_requested: "asked for changes",
	dismissed: "dismissed",
};

export function reviewVerdictLabel(verdict: ReviewVerdict): string {
	return reviewVerdicts[verdict];
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

export function sourceControlRepositoryPath(slug: string, repositoryId: string): string {
	return workspacePath(slug, `/settings/source-control/repositories/${repositoryId}`);
}

export function sourceControlIdentitiesPath(slug: string): string {
	return workspacePath(slug, "/settings/source-control/identities");
}

export function connectionLabel(connection: SourceControlConnection): string {
	return (
		connection.label ||
		connection.identityLogin ||
		connection.baseUrl ||
		providerLabel(connection.provider)
	);
}

export function routeLabel(route: SourceControlRoute): string {
	return route.pathPrefix || "Everything else";
}

export function sourceControlConnectPath(slug: string): string {
	return workspacePath(slug, "/settings/source-control/repositories/connect");
}

export type SourceControlStage =
	| "unsupported"
	| "install"
	| "authorise"
	| "connect_repository"
	| "repositories_unknown"
	| "broken"
	| "watching";

export type SourceControlVerdict = {
	stage: SourceControlStage;
	tone: "warning" | "destructive" | "success" | "muted";
	title: string;
	detail: string;
	/** Present when there is something to click that moves the stage on. */
	action?: { label: string; href: string };
};

/**
 * The whole pipeline reduced to the one stage it is stuck at. Every screen renders this rather
 * than deciding for itself, because the failure this exists to prevent was each panel reporting
 * its own step as fine while the pipeline as a whole did nothing.
 */
export function sourceControlVerdict(input: {
	slug: string;
	application: SourceControlAppState;
	connections: SourceControlConnection[];
	repositories: SourceControlRepository[];
	repositoriesUnavailable?: boolean;
}): SourceControlVerdict {
	const { slug, application, connections, repositories, repositoriesUnavailable } = input;

	const broken = connections.find((connection) => connection.status === "broken");

	if (broken) {
		return {
			stage: "broken",
			tone: "destructive",
			title: `${connectionLabel(broken)} has stopped working`,
			detail: `${brokenLabel(broken)}. Nothing from it reaches Norn until it is working again.`,
			action: { label: "Open the connection", href: sourceControlConnectionPath(slug, broken.id) },
		};
	}

	// A listing that failed arrives here as an empty array, so without this the banner would
	// report a broken request as "nothing is connected" — the precise lie this verdict exists
	// to prevent, and one the section below it already contradicts.
	if (repositoriesUnavailable) {
		return {
			stage: "repositories_unknown",
			tone: "destructive",
			title: "What Norn watches is unknown",
			detail:
				"The connected repositories could not be read, which is not the same as having none. Check your connection and reload.",
		};
	}

	if (repositories.length > 0) {
		const watching =
			repositories.length === 1
				? "Watching 1 repository."
				: `Watching ${repositories.length} repositories.`;

		return {
			stage: "watching",
			tone: "success",
			title: watching,
			detail:
				"Branches, commits and pull requests naming an issue are linked to it as they happen.",
		};
	}

	if (connections.length > 0) {
		return {
			stage: "connect_repository",
			tone: "warning",
			title: "No repository is connected",
			detail:
				"The credential is held and working, but Norn is not watching anything yet, so every event GitHub sends is discarded.",
			action: { label: "Connect a repository", href: sourceControlConnectPath(slug) },
		};
	}

	if (application.kind === "choosing") {
		if (application.installations.length === 0) {
			return {
				stage: "install",
				tone: "warning",
				title: "The application is installed on no account you administer",
				detail:
					"You are signed in to GitHub, but there is nothing to choose. Install it on the repositories Norn should watch, then connect again.",
			};
		}

		return {
			stage: "authorise",
			tone: "warning",
			title: "Choose the installation this workspace uses",
			detail: "You are signed in to GitHub. Pick the account whose repositories Norn watches.",
		};
	}

	if (application.kind === "registered") {
		return {
			stage: "install",
			tone: "warning",
			title: "This workspace has not connected GitHub",
			detail:
				"Install the application on the repositories Norn should watch, then sign in to say which installation this workspace uses.",
		};
	}

	return {
		stage: "unsupported",
		tone: "muted",
		title: "No application is set up on this instance",
		detail: "A credential can still be held directly, which reaches as far as that token does.",
	};
}

/**
 * Routes only ever narrow. A repository carrying none reaches every team, so the absence of
 * routes is the working state and must never be reported as a problem.
 */
export function routingLabel(repository: SourceControlRepository): string {
	const routes = repository.routeCount ?? 0;

	if (routes === 0) return "Any team can be linked from it";
	if (routes === 1) return "Narrowed to 1 routed path";

	return `Narrowed to ${routes} routed paths`;
}
