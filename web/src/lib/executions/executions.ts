import type { components } from "$lib/api/dashboard.gen";
import type { CodeLink } from "$lib/source-control/source-control";

export type Execution = components["schemas"]["Execution"];
export type ExecutionState = components["schemas"]["ExecutionState"];
export type ExecutionQueuedReason = components["schemas"]["ExecutionQueuedReason"];
export type ExecutionEvent = components["schemas"]["ExecutionEvent"];
export type ExecutionEventKind = components["schemas"]["ExecutionEventKind"];
export type ExecutionService = components["schemas"]["ExecutionService"];
export type ExecutionRunner = components["schemas"]["ExecutionRunner"];
export type ExecutionPreview = components["schemas"]["ExecutionPreview"];
export type ExecutionLogEntry = components["schemas"]["ExecutionLogEntry"];
export type ExecutionTranscriptEntry = components["schemas"]["ExecutionTranscriptEntry"];
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
			transcript: ExecutionTranscriptEntry[];
			logs: ExecutionLogEntry[];
			transcriptCursor?: number;
			logCursor?: number;
			changeset?: ExecutionChangeSet;
			codeLinks: CodeLink[];
	  };

export const chunkPageSize = 8;

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
		case "queued_for_resume":
			return "The answer is in. This run starts again as soon as the machine has a slot free.";
		case "finalizing":
			return "The coding agent has finished. The machine is pushing branches and collecting what changed.";
		case "awaiting_review":
			return "The work is done and waiting for somebody to accept or reject it.";
		case "approved":
			return "Somebody accepted this work. The machine is giving the workspace back.";
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

export function canApprove(execution: Execution): boolean {
	return execution.state === "awaiting_review";
}

export function canRequestChanges(execution: Execution): boolean {
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
	| { kind: "no_runner" }
	| { kind: "already_live" }
	| { kind: "not_delegated" }
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
		case "execution_no_runner":
			return { kind: "no_runner" };
		case "execution_already_live":
			return { kind: "already_live" };
		case "execution_not_delegated":
			return { kind: "not_delegated" };
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
		case "no_runner":
			return "This agent has no machine to hand the work to.";
		case "already_live":
			return "A newer attempt at this is already going. Open it rather than starting another.";
		case "not_delegated":
			return "Nobody is holding this issue any more. Hand it to an agent again to run it.";
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

export function logsFor(logs: ExecutionLogEntry[], service: string): ExecutionLogEntry[] {
	return logs.filter((entry) => entry.source === service);
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

export function transcriptSpeaker(entry: ExecutionTranscriptEntry): string {
	switch (entry.type) {
		case "message":
			return "Said";
		case "tool_call":
			return "Used";
		case "tool_result":
			return "Answered";
		case "usage":
			return "Reported what the turn cost";
		default:
			return entry.type;
	}
}

export function transcriptText(entry: ExecutionTranscriptEntry): string {
	const payload = entry.payload ?? {};

	for (const field of ["text", "content", "tool", "name"]) {
		const held = payload[field];

		if (typeof held === "string" && held !== "") return held;
	}

	return JSON.stringify(payload);
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

export function noPullRequestLine(change: ExecutionRepositoryChange): string {
	if (change.branch) return `Pushed to ${change.branch}. No pull request was opened.`;

	return "No pull request was opened.";
}

export type DiffReach = { kind: "available"; artifactId: string } | { kind: "absent" };

export function diffReach(change: ExecutionRepositoryChange): DiffReach {
	if (change.diffArtifactId) return { kind: "available", artifactId: change.diffArtifactId };

	return { kind: "absent" };
}

export const noDiffLine =
	"The full diff was not kept for this repository. The branch still has every commit.";

export type DiffLineKind = "add" | "remove" | "context";

export type DiffLine = { kind: DiffLineKind; text: string };

export type DiffHunk = { header: string; lines: DiffLine[] };

export type DiffFile = {
	path: string;
	additions: number;
	deletions: number;
	hunks: DiffHunk[];
	binary: boolean;
};

export type DiffView =
	| { kind: "idle" }
	| { kind: "loading" }
	| { kind: "absent" }
	| { kind: "failed"; message: string }
	| { kind: "ready"; files: DiffFile[]; truncated: boolean };

export const diffFailedLine =
	"The diff could not be read. It may have been swept with the rest of this run's uploads.";

export const diffTruncatedLine =
	"This is the beginning of the diff. Download it to read the rest.";

function pathOfDiffHeader(line: string): string {
	const parts = line.split(" ");
	const named = parts.at(-1) ?? "";

	return named.startsWith("b/") ? named.slice(2) : named;
}

export function parseDiff(patch: string): DiffFile[] {
	const files: DiffFile[] = [];

	let file: DiffFile | undefined;
	let hunk: DiffHunk | undefined;

	for (const line of patch.split("\n")) {
		if (line.startsWith("diff --git ")) {
			file = { path: pathOfDiffHeader(line), additions: 0, deletions: 0, hunks: [], binary: false };
			hunk = undefined;
			files.push(file);

			continue;
		}

		if (!file) continue;

		if (line.startsWith("Binary files ") || line.startsWith("GIT binary patch")) {
			file.binary = true;

			continue;
		}

		if (line.startsWith("+++ b/")) {
			file.path = line.slice(6);

			continue;
		}

		if (line.startsWith("@@")) {
			hunk = { header: line, lines: [] };
			file.hunks.push(hunk);

			continue;
		}

		if (!hunk) continue;

		if (line.startsWith("+")) {
			file.additions += 1;
			hunk.lines.push({ kind: "add", text: line.slice(1) });

			continue;
		}

		if (line.startsWith("-")) {
			file.deletions += 1;
			hunk.lines.push({ kind: "remove", text: line.slice(1) });

			continue;
		}

		if (line.startsWith("\\")) continue;

		hunk.lines.push({ kind: "context", text: line.startsWith(" ") ? line.slice(1) : line });
	}

	return files;
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

export const diffMaxBytes = 512 * 1024;
