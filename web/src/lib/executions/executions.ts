import type { components } from "$lib/api/dashboard.gen";

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

export type RunView =
	| { kind: "loading" }
	| { kind: "not_found" }
	| { kind: "unavailable" }
	| {
			kind: "ready";
			execution: Execution;
			timeline: ExecutionEvent[];
			services: ExecutionService[];
			previews: ExecutionPreview[];
			runner?: ExecutionRunner;
			questions: IssueQuestion[];
			transcript: ExecutionTranscriptEntry[];
			logs: ExecutionLogEntry[];
			transcriptCursor?: number;
			logCursor?: number;
	  };

export type RunCost = { kind: "unrecorded" } | { kind: "known"; text: string };

export const chunkPageSize = 8;

export const timelinePageSize = 100;

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
	| { kind: "no_runner" }
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
		case "execution_no_runner":
			return { kind: "no_runner" };
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
		case "no_runner":
			return "This agent has no machine to hand the work to.";
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

export function costLine(cost: RunCost): string {
	return cost.kind === "known" ? cost.text : "Not recorded yet";
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

export function eventLine(event: ExecutionEvent): string {
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
