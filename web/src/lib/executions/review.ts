import type { components } from "$lib/api/dashboard.gen";
import type { DiffAnchor, DiffFile } from "./diff";
import type { Execution, ExecutionChangeSet, IssueQuestion } from "./executions";
import type { DecisionRight } from "./reviews";

export type ReviewComment = components["schemas"]["ReviewComment"];
export type ExecutionReview = components["schemas"]["ExecutionReview"];
export type ReviewState = components["schemas"]["ExecutionReviewState"];
export type ReviewVerdict = components["schemas"]["ExecutionReviewVerdict"];
export type ReviewHead = components["schemas"]["ReviewHead"];

export type RepositoryDiff =
	| { kind: "absent" }
	| { kind: "failed" }
	| { kind: "ready"; files: DiffFile[]; truncated: boolean };

export type ReviewedRepository = {
	repository: string;
	branch?: string;
	headSha: string;
	additions: number;
	deletions: number;
	artifactId?: string;
	diff: RepositoryDiff;
};

export type ReviewView =
	| { kind: "loading" }
	| { kind: "not_found" }
	| { kind: "unavailable" }
	| {
			kind: "ready";
			execution: Execution;
			changeset?: ExecutionChangeSet;
			review: ReviewState;
			repositories: ReviewedRepository[];
			questions: IssueQuestion[];
			right: DecisionRight;
	  };

export type ReviewLayout = "unified" | "split";

export type ListedFile = { id: string; file: DiffFile; threads: number; viewed: boolean };

export type ListedRepository = { repository: string; files: ListedFile[]; note?: string };

export type ReviewThread = { root: ReviewComment; replies: ReviewComment[] };

export function threadsOf(comments: ReviewComment[]): ReviewThread[] {
	const threads = new Map<string, ReviewThread>();

	for (const comment of comments) {
		if (!comment.parentId) threads.set(comment.id, { root: comment, replies: [] });
	}

	for (const comment of comments) {
		if (comment.parentId) threads.get(comment.parentId)?.replies.push(comment);
	}

	return [...threads.values()];
}

export function threadsOn(
	threads: ReviewThread[],
	repository: string,
	path: string
): ReviewThread[] {
	return threads.filter(
		(thread) => thread.root.repository === repository && thread.root.path === path
	);
}

export function threadsAt(threads: ReviewThread[], anchor: DiffAnchor): ReviewThread[] {
	return threads.filter(
		(thread) =>
			!thread.root.outdated && thread.root.side === anchor.side && thread.root.line === anchor.line
	);
}

export function outdatedThreads(threads: ReviewThread[]): ReviewThread[] {
	return threads.filter((thread) => thread.root.outdated);
}

export function draftCount(comments: ReviewComment[]): number {
	return comments.filter((comment) => comment.pending && comment.mine).length;
}

export function repositoryNote(repository: ReviewedRepository): string | undefined {
	switch (repository.diff.kind) {
		case "absent":
			return "The diff was not kept, so it cannot be read here. The branch still has every commit.";
		case "failed":
			return "The diff could not be read. Download it from the run page instead.";
		case "ready":
			return repository.diff.truncated
				? "This diff is too long to show whole. The files it runs out on are left off here."
				: undefined;
	}
}

export function fileId(repositoryIndex: number, fileIndex: number): string {
	return `file-${repositoryIndex}-${fileIndex}`;
}

export function reviewOpen(execution: Execution): boolean {
	return execution.state === "awaiting_review";
}

export function reviewClosedLine(execution: Execution): string {
	switch (execution.state) {
		case "approved":
			return "These changes were approved. The machine is pushing the branch and opening the pull request.";
		case "completed":
			return "These changes were approved and published. The review is kept here as it was.";
		case "queued_for_resume":
		case "running":
		case "finalizing":
			return "Somebody asked for changes. The coding agent is working on them, and the review opens again once it has finished.";
		case "failed":
		case "cancelled":
		case "interrupted":
			return "This run stopped, so its changes are no longer waiting for review.";
		default:
			return "These changes are not waiting for review.";
	}
}

export function verdictLabel(verdict: ReviewVerdict): string {
	switch (verdict) {
		case "approve":
			return "Approved";
		case "request_changes":
			return "Asked for changes";
		case "comment":
			return "Commented";
	}
}

export function verdictChoice(verdict: ReviewVerdict): string {
	switch (verdict) {
		case "approve":
			return "Approve";
		case "request_changes":
			return "Request changes";
		case "comment":
			return "Comment";
	}
}

export function draftsLine(drafts: number): string {
	if (drafts === 0) return "No draft comments";
	if (drafts === 1) return "1 draft comment is published with it";

	return `${drafts} draft comments are published with it`;
}

export function verdictLine(verdict: ReviewVerdict): string {
	switch (verdict) {
		case "approve":
			return "Push the branch and open the pull request.";
		case "request_changes":
			return "Send every comment to the coding agent, anchored to its line, and review again after.";
		case "comment":
			return "Publish your comments without deciding yet.";
	}
}

export function submitLabel(verdict: ReviewVerdict, drafts: number): string {
	switch (verdict) {
		case "approve":
			return "Approve and publish";
		case "request_changes":
			return "Send it back";
		case "comment":
			return drafts === 1 ? "Publish 1 comment" : `Publish ${drafts} comments`;
	}
}

export function headsOf(repositories: ReviewedRepository[]): ReviewHead[] {
	return repositories.map((repository) => ({
		repository: repository.repository,
		headSha: repository.headSha,
	}));
}

export function viewedKey(executionId: string, repository: ReviewedRepository, path: string): string {
	return `norn:review-viewed:${executionId}:${repository.repository}:${repository.headSha}:${path}`;
}

export const reviewBodyMaxLength = 4000;

export const questionsOpenLine =
	"Answer every open question on this run before approving or sending it back.";
