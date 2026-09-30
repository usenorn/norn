import type { components } from "$lib/api/dashboard.gen";
import type { CodeLink } from "$lib/source-control/source-control";
import type { ExecutionPlan } from "./plans";
import type { DecisionRight } from "./reviews";

export type Execution = components["schemas"]["Execution"];
export type ExecutionState = components["schemas"]["ExecutionState"];
export type ExecutionQueuedReason = components["schemas"]["ExecutionQueuedReason"];
export type ExecutionEvent = components["schemas"]["ExecutionEvent"];
export type ExecutionEventKind = components["schemas"]["ExecutionEventKind"];
export type ExecutionService = components["schemas"]["ExecutionService"];
export type ExecutionRunner = components["schemas"]["ExecutionRunner"];
export type ExecutionPreview = components["schemas"]["ExecutionPreview"];
export type IssueQuestion = components["schemas"]["IssueQuestion"];
export type ExecutionChangeSet = components["schemas"]["ExecutionChangeSet"];
export type ExecutionRepositoryChange = components["schemas"]["ExecutionRepositoryChange"];
export type ExecutionValidation = components["schemas"]["ExecutionValidation"];
export type ExecutionPreviewDetail = components["schemas"]["ExecutionPreviewDetail"];
export type PreviewShareLink = components["schemas"]["PreviewShareLink"];
export type ExecutionSummary = components["schemas"]["ExecutionSummary"];
export type ExecutionChangeSummary = components["schemas"]["ExecutionChangeSummary"];
export type IssueChangeSet = components["schemas"]["IssueChangeSet"];
export type IssueRepositoryChange = components["schemas"]["IssueRepositoryChange"];

export type RunView =
	| { kind: "loading" }
	| { kind: "not_found" }
	| { kind: "unavailable" }
	| {
			kind: "ready";
			execution: Execution;
			timeline: ExecutionEvent[];
			services: ExecutionService[];
			previews: ExecutionPreviewDetail[];
			runner?: ExecutionRunner;
			questions: IssueQuestion[];
			changeset?: ExecutionChangeSet;
			codeLinks: CodeLink[];
			plans: ExecutionPlan[];
			right: DecisionRight;
	  };

export const timelinePageSize = 100;

export const timelinePreviewSize = 50;

const working: ExecutionState[] = ["preparing", "running", "finalizing"];

const settled: ExecutionState[] = ["completed", "failed", "cancelled", "interrupted"];

export function isWorking(state: ExecutionState): boolean {
	return working.includes(state);
}

export function isSettled(state: ExecutionState): boolean {
	return settled.includes(state);
}

export function stateLabel(state: ExecutionState): string {
	switch (state) {
		case "queued":
			return "Queued";
		case "leased":
			return "Taken";
		case "preparing":
			return "Preparing";
		case "running":
			return "Running";
		case "waiting_for_input":
			return "Waiting on you";
		case "awaiting_plan_approval":
			return "Plan waiting for approval";
		case "queued_for_resume":
			return "Waiting for a slot";
		case "finalizing":
			return "Finishing";
		case "awaiting_review":
			return "Waiting for review";
		case "approved":
			return "Approved";
		case "completed":
			return "Completed";
		case "failed":
			return "Failed";
		case "cancelled":
			return "Cancelled";
		case "interrupted":
			return "Interrupted";
	}
}

export function elapsedLabel(state: ExecutionState): string {
	if (isWorking(state)) return "running for";
	if (isSettled(state)) return "ran for";

	switch (state) {
		case "queued":
		case "leased":
		case "queued_for_resume":
		case "waiting_for_input":
		case "awaiting_plan_approval":
			return "waiting for";
		default:
			return "open for";
	}
}

export type StateTone = "waiting" | "working" | "attention" | "done" | "bad";

export function stateTone(state: ExecutionState): StateTone {
	switch (state) {
		case "queued":
		case "leased":
		case "queued_for_resume":
			return "waiting";
		case "preparing":
		case "running":
		case "finalizing":
			return "working";
		case "waiting_for_input":
		case "awaiting_plan_approval":
		case "awaiting_review":
			return "attention";
		case "approved":
		case "completed":
			return "done";
		case "failed":
		case "cancelled":
		case "interrupted":
			return "bad";
	}
}

export function standingLine(execution: Execution): string {
	switch (execution.state) {
		case "queued":
			return waitingLine(execution.queuedReason);
		case "leased":
			return "A machine has taken this run and is about to start it.";
		case "preparing":
			return "The machine is copying the folder, making branches and starting the coding agent.";
		case "running":
			return "The coding agent is working.";
		case "waiting_for_input":
			return "The coding agent stopped to ask something and cannot go on until somebody answers.";
		case "awaiting_plan_approval":
			return "The coding agent has proposed a plan. Nothing is written until somebody approves it.";
		case "queued_for_resume":
			return "A decision is in. This run starts again as soon as the machine has a slot free.";
		case "finalizing":
			return "The coding agent has finished. The machine is collecting what changed for review.";
		case "awaiting_review":
			return "The changes are ready to review. Nothing has been pushed yet.";
		case "approved":
			return "Somebody approved the changes. The machine is pushing the branch and opening the pull request.";
		case "completed":
			return "This run is finished and the machine has given the workspace back.";
		case "failed":
			return execution.reason || "This run stopped without finishing.";
		case "cancelled":
			return execution.reason || "Somebody stopped this run.";
		case "interrupted":
			return (
				execution.reason ||
				"The machine stopped reporting and its lease lapsed, so norn marked this run interrupted."
			);
	}
}

export function waitingLine(reason: ExecutionQueuedReason | undefined): string {
	switch (reason) {
		case "no_runner":
			return "This agent has no machine connected, so there is nothing to hand the work to.";
		case "runners_offline":
			return "This agent's machines are all offline. The run starts the moment one comes back.";
		case "runners_paused":
			return "This agent's machines are all paused. The run starts the moment one takes work again.";
		case "runners_busy":
			return "This agent's machines are all busy. The run starts the moment a slot frees.";
		default:
			return "This run is waiting for a machine.";
	}
}

export function canCancel(execution: Execution): boolean {
	return !isSettled(execution.state) && execution.state !== "approved";
}

export function canRestart(execution: Execution): boolean {
	return execution.restartable === true;
}

export function canRetain(execution: Execution): boolean {
	return execution.state === "awaiting_review";
}

export function blockingQuestion(questions: IssueQuestion[]): IssueQuestion | undefined {
	return questions.find(
		(question) => question.blocking && question.state === "asked" && !question.expired
	);
}

export type RunFailure =
	| { kind: "transition" }
	| { kind: "finished" }
	| { kind: "unfinished" }
	| { kind: "not_reviewable" }
	| { kind: "self_approval" }
	| { kind: "not_planning" }
	| { kind: "plan_missing" }
	| { kind: "plan_stale" }
	| { kind: "questions_open" }
	| { kind: "decision_forbidden" }
	| { kind: "review_closed" }
	| { kind: "review_stale" }
	| { kind: "publication_not_pending" }
	| { kind: "review_empty" }
	| { kind: "comment_not_yours" }
	| { kind: "comment_reply" }
	| { kind: "comment_anchor" }
	| { kind: "comments_full" }
	| { kind: "no_runner" }
	| { kind: "preview_closed" }
	| { kind: "preview_not_routable" }
	| { kind: "share_crowded" }
	| { kind: "share_gone" }
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export function readRunFailure(error: unknown): RunFailure {
	if (!error || typeof error !== "object") return { kind: "unavailable" };

	const problem = error as { code?: string; status?: number };

	switch (problem.code) {
		case "execution_transition":
			return { kind: "transition" };
		case "execution_finished":
			return { kind: "finished" };
		case "execution_unfinished":
			return { kind: "unfinished" };
		case "execution_not_reviewable":
			return { kind: "not_reviewable" };
		case "execution_self_approval":
			return { kind: "self_approval" };
		case "execution_not_planning":
			return { kind: "not_planning" };
		case "execution_plan_missing":
			return { kind: "plan_missing" };
		case "execution_plan_stale":
			return { kind: "plan_stale" };
		case "execution_questions_open":
			return { kind: "questions_open" };
		case "decision_forbidden":
			return { kind: "decision_forbidden" };
		case "review_closed":
			return { kind: "review_closed" };
		case "review_stale":
			return { kind: "review_stale" };
		case "publication_not_pending":
			return { kind: "publication_not_pending" };
		case "review_empty":
			return { kind: "review_empty" };
		case "review_comment_not_yours":
			return { kind: "comment_not_yours" };
		case "review_comment_reply":
			return { kind: "comment_reply" };
		case "review_comment_anchor":
			return { kind: "comment_anchor" };
		case "review_comments_full":
			return { kind: "comments_full" };
		case "execution_no_runner":
			return { kind: "no_runner" };
		case "preview_closed":
			return { kind: "preview_closed" };
		case "preview_not_routable":
			return { kind: "preview_not_routable" };
		case "preview_share_crowded":
			return { kind: "share_crowded" };
		case "preview_share_expired":
		case "preview_share_revoked":
			return { kind: "share_gone" };
		default:
			break;
	}

	if (problem.status === 404) return { kind: "gone" };
	if (problem.status === 403) return { kind: "forbidden" };

	return { kind: "unavailable" };
}

export function runFailureMessage(failure: RunFailure): string {
	switch (failure.kind) {
		case "transition":
			return "This run has moved on since the page was loaded. Reload to see where it got to.";
		case "finished":
			return "This run has already finished, so there is nothing to stop.";
		case "unfinished":
			return "This run has not finished, so it cannot be started again yet.";
		case "not_reviewable":
			return "This run is not waiting to be reviewed.";
		case "self_approval":
			return "A machine may not accept its own work. Somebody else has to review this run.";
		case "not_planning":
			return "This run is not waiting for its plan to be approved any more. Reload to see where it got to.";
		case "plan_missing":
			return "The coding agent has not proposed a plan yet.";
		case "plan_stale":
			return "The coding agent has proposed a newer revision of the plan. Read that one before deciding.";
		case "questions_open":
			return "The run is still waiting on an answer. Answer every open question before deciding.";
		case "decision_forbidden":
			return "Only the assignee or a workspace admin can decide this.";
		case "review_closed":
			return "These changes are no longer waiting for review. Reload to see where the run got to.";
		case "review_stale":
			return "The changes moved on since you opened them. Reload to review what is there now.";
		case "publication_not_pending":
			return "This run is not waiting to publish any more. Reload to see where it got to.";
		case "review_empty":
			return "Say what should change, in the summary or on a line, before sending the work back.";
		case "comment_not_yours":
			return "Only the person who wrote a comment can change or remove it.";
		case "comment_reply":
			return "Reply to the first comment of a thread, not to a reply.";
		case "comment_anchor":
			return "That line is not part of the changes under review.";
		case "comments_full":
			return "This run already carries as many review comments as norn keeps.";
		case "no_runner":
			return "This agent has no machine to hand the work to.";
		case "preview_closed":
			return "This preview has been closed, so there is nothing left to share.";
		case "preview_not_routable":
			return "This server serves no preview domain, so there is no address to share.";
		case "share_crowded":
			return "This preview already has as many share links as norn keeps. Revoke one first.";
		case "share_gone":
			return "That link is already gone.";
		case "gone":
			return "This run is no longer here.";
		case "forbidden":
			return "You may not act on this run.";
		default:
			return "Something went wrong and nothing changed. Wait a moment and try again.";
	}
}

export type PreviewReach =
	| { kind: "open"; url: string }
	| { kind: "not_routable" }
	| { kind: "machine_offline" }
	| { kind: "closed" };

export function previewReach(
	preview: ExecutionPreview,
	runner: ExecutionRunner | undefined
): PreviewReach {
	if (preview.state === "closed") return { kind: "closed" };
	if (!preview.url) return { kind: "not_routable" };
	if (runner && !runner.load.connected) return { kind: "machine_offline" };

	return { kind: "open", url: preview.url };
}

export function previewReachLine(reach: PreviewReach): string {
	switch (reach.kind) {
		case "open":
			return reach.url;
		case "not_routable":
			return "This server serves no preview domain, so there is no address that reaches anybody.";
		case "machine_offline":
			return "The machine running this preview is offline, so the address will not open.";
		case "closed":
			return "This preview has been closed.";
	}
}

export function noPreviewsLine(execution: Execution): string {
	if (isSettled(execution.state)) return "This run opened no previews.";

	if (isWorking(execution.state) || execution.state === "leased") {
		return "Nothing is up yet. A preview appears here when the coding agent exposes a service.";
	}

	return "Nothing is up yet.";
}

export function noServicesLine(execution: Execution): string {
	if (isSettled(execution.state)) return "This run started no services.";

	return "Nothing is running yet.";
}

export function serviceStateLabel(state: ExecutionService["state"]): string {
	switch (state) {
		case "starting":
			return "Starting";
		case "healthy":
			return "Healthy";
		case "unhealthy":
			return "Unhealthy";
		case "stopped":
			return "Stopped";
	}
}

export function probeLine(probe: ExecutionService["probe"]): string {
	switch (probe) {
		case "http":
			return "Checked over HTTP";
		case "tcp":
			return "Checked by connecting";
		case "log":
			return "Checked by what it prints";
		default:
			return "Nothing checks it";
	}
}

export function slotLine(runner: ExecutionRunner | undefined): string | undefined {
	if (!runner) return undefined;
	if (!runner.load.connected) return "offline";

	return `${runner.load.used} of ${runner.load.capacity} slots in use`;
}

export function eventLabel(kind: ExecutionEventKind): string {
	switch (kind) {
		case "transition":
			return "State";
		case "phase":
			return "Progress";
		case "command":
			return "Command";
		case "tool":
			return "Tool";
		case "service":
			return "Service";
		case "preview":
			return "Preview";
		case "question":
			return "Question";
		case "note":
			return "Note";
	}
}

const queuedReasons: string[] = [
	"no_runner",
	"runners_offline",
	"runners_paused",
	"runners_busy",
];

export function eventLine(event: ExecutionEvent): string {
	if (event.toState === "queued" && event.reason && queuedReasons.includes(event.reason)) {
		return waitingLine(event.reason as ExecutionQueuedReason);
	}

	if (event.reason) return event.reason;

	if (event.kind === "transition" && event.toState) {
		const from = event.fromState ? stateLabel(event.fromState) + " to " : "";

		return from + stateLabel(event.toState);
	}

	return eventLabel(event.kind);
}

export function actorLine(event: ExecutionEvent): string {
	switch (event.actor.kind) {
		case "agent":
			return "the machine";
		case "user":
			return "a person";
		case "token":
			return "a token";
		default:
			return "norn";
	}
}

export function mergeTimeline(held: ExecutionEvent[], arriving: ExecutionEvent[]): ExecutionEvent[] {
	const merged = new Map<string, ExecutionEvent>();

	for (const event of held) merged.set(event.id, event);
	for (const event of arriving) merged.set(event.id, event);

	return [...merged.values()].sort((left, right) => left.sequence - right.sequence);
}

export type ChangeTotals = {
	repositories: number;
	commits: number;
	additions: number;
	deletions: number;
	filesChanged: number;
};

export function changeTotals(changes: ExecutionRepositoryChange[]): ChangeTotals {
	return changes.reduce(
		(running, change) => ({
			repositories: running.repositories + 1,
			commits: running.commits + change.commits,
			additions: running.additions + change.additions,
			deletions: running.deletions + change.deletions,
			filesChanged: running.filesChanged + change.filesChanged,
		}),
		{ repositories: 0, commits: 0, additions: 0, deletions: 0, filesChanged: 0 }
	);
}

function counted(amount: number, one: string, many: string): string {
	return `${amount} ${amount === 1 ? one : many}`;
}

export function diffStatLine(change: {
	additions: number;
	deletions: number;
	filesChanged: number;
}): string {
	return `+${change.additions} −${change.deletions} · ${counted(change.filesChanged, "file", "files")}`;
}

export function changeStatLine(totals: ChangeTotals): string {
	if (totals.repositories === 0) return "Nothing changed.";

	return [
		counted(totals.commits, "commit", "commits"),
		`across ${counted(totals.repositories, "repository", "repositories")}`,
		`· ${diffStatLine(totals)}`,
	].join(" ");
}

export function noChangesLine(execution: Execution): string {
	if (isSettled(execution.state) || execution.state === "approved") {
		return "This run reported nothing it changed.";
	}

	if (execution.state === "awaiting_review") {
		return "The machine reported no repository it touched.";
	}

	if (execution.stage === "planning") {
		return "Nothing is built until somebody approves the plan.";
	}

	return "Nothing yet. What the run changed appears here once the coding agent has finished.";
}

export type PullRequestReach =
	| { kind: "linked"; link: CodeLink }
	| { kind: "address"; url: string }
	| { kind: "none" };

export function pullRequestReach(
	change: ExecutionRepositoryChange,
	links: CodeLink[]
): PullRequestReach {
	if (change.codeLinkId) {
		const link = links.find((held) => held.id === change.codeLinkId);

		if (link) return { kind: "linked", link };
	}

	if (change.pullRequestUrl) return { kind: "address", url: change.pullRequestUrl };

	return { kind: "none" };
}

export type RepositoryPublicationView =
	| { kind: "held" }
	| { kind: "publishing" }
	| { kind: "pushed"; branch?: string }
	| { kind: "failed"; step: "push" | "pull_request"; error: string };

export function publicationOf(
	execution: Execution | undefined,
	change: ExecutionRepositoryChange
): RepositoryPublicationView {
	if (execution && execution.stage !== "publication") return { kind: "held" };

	const publication = change.publication;

	if (publication?.state === "failed") {
		return { kind: "failed", step: publication.step ?? "push", error: publication.error ?? "" };
	}

	if (execution?.state === "approved" && publication?.state !== "pushed") {
		return { kind: "publishing" };
	}

	return { kind: "pushed", branch: change.branch };
}

export function publicationLine(view: RepositoryPublicationView): string {
	switch (view.kind) {
		case "held":
			return "Nothing is pushed until somebody approves the changes.";
		case "publishing":
			return "Pushing the branch and opening the pull request.";
		case "failed": {
			const step =
				view.step === "pull_request" ? "Opening the pull request failed" : "Pushing the approved commits failed";

			return view.error ? `${step}: ${view.error}` : `${step}.`;
		}
		case "pushed":
			return view.branch
				? `Pushed to ${view.branch}. No pull request was opened.`
				: "No pull request was opened.";
	}
}

export function publicationIncomplete(
	execution: Execution,
	changeset: ExecutionChangeSet | undefined
): boolean {
	if (execution.state !== "approved" || !changeset) return false;

	const states = changeset.repositories.map((change) => change.publication?.state);

	return states.includes("failed") && !states.includes("pending");
}

export type DiffReach = { kind: "available"; artifactId: string } | { kind: "absent" };

export function diffReach(change: ExecutionRepositoryChange): DiffReach {
	if (change.diffArtifactId) return { kind: "available", artifactId: change.diffArtifactId };

	return { kind: "absent" };
}

export function validationLabel(status: ExecutionValidation["status"]): string {
	switch (status) {
		case "passed":
			return "Passed";
		case "failed":
			return "Failed";
		case "skipped":
			return "Skipped";
	}
}

export type RetentionClock =
	| { kind: "deciding" }
	| { kind: "unsaid" }
	| { kind: "holding"; until: string }
	| { kind: "given_back"; at: string };

export function retentionClock(execution: Execution, now: string): RetentionClock {
	if (!execution.keepUntil) {
		return execution.state === "awaiting_review" ? { kind: "deciding" } : { kind: "unsaid" };
	}

	if (new Date(execution.keepUntil).getTime() <= new Date(now).getTime()) {
		return { kind: "given_back", at: execution.keepUntil };
	}

	return { kind: "holding", until: execution.keepUntil };
}

export function retentionLine(clock: RetentionClock, when: string): string {
	switch (clock.kind) {
		case "deciding":
			return "This machine is holding the workspace and its previews while you decide.";
		case "unsaid":
			return "This machine has not said when it gives the workspace and its previews back.";
		case "holding":
			return `The workspace and its previews go at ${when}.`;
		case "given_back":
			return `The workspace and its previews went at ${when}. Everything on this page stays.`;
	}
}

export function shouldShowRetention(execution: Execution): boolean {
	return (
		execution.state === "awaiting_review" ||
		execution.state === "approved" ||
		isSettled(execution.state)
	);
}

export type ShareStanding = "live" | "expired" | "revoked";

export function shareStanding(link: PreviewShareLink, now: string): ShareStanding {
	if (link.revokedAt) return "revoked";
	if (new Date(link.expiresAt).getTime() <= new Date(now).getTime()) return "expired";

	return "live";
}

export function shareStandingLabel(standing: ShareStanding): string {
	switch (standing) {
		case "live":
			return "Live";
		case "expired":
			return "Expired";
		case "revoked":
			return "Revoked";
	}
}

export function shareUseLine(link: PreviewShareLink): string {
	if (link.uses === 0) return "Nobody has opened it";

	return `Opened ${counted(link.uses, "time", "times")}`;
}

export const shareOnceLine =
	"This address is answered once and never again. Norn keeps only its fingerprint.";

export const noShareLinksLine = "Nobody outside the workspace can reach this preview.";

export const shareLifetimes = [
	{ label: "1 hour", seconds: 3600 },
	{ label: "8 hours", seconds: 28_800 },
	{ label: "1 day", seconds: 86_400 },
	{ label: "7 days", seconds: 604_800 },
];

export const retainLongerSeconds = 3600;

export const feedbackMaxLength = 4000;


export function reviewLinkLabel(execution: Execution | undefined): string {
	return execution?.state === "awaiting_review" ? "Review the changes" : "Read the review";
}

export const noDiffLine =
	"The full diff was not kept for this repository. The branch still has every commit.";
