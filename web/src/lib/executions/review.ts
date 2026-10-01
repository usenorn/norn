import type { components } from "$lib/api/dashboard.gen";
import { diffFilesMax, type DiffAnchor, type DiffFile } from "./diff";
import type { ChangeTotals, Execution, IssueQuestion } from "./executions";
import type { DecisionRight } from "./reviews";

export type ReviewComment = components["schemas"]["ReviewComment"];
export type ExecutionReview = components["schemas"]["ExecutionReview"];
export type ReviewState = components["schemas"]["ExecutionReviewState"];
export type ReviewVerdict = components["schemas"]["ExecutionReviewVerdict"];
export type ReviewHead = components["schemas"]["ReviewHead"];
export type ReviewCommit = components["schemas"]["ReviewCommit"];
export type ReviewPreview = components["schemas"]["ReviewPreview"];
export type ReviewRevision = components["schemas"]["ReviewRevision"];
export type ReviewRepository = components["schemas"]["ReviewRepository"];

export type RepositoryDiff =
	| { kind: "absent" }
	| { kind: "failed" }
	| { kind: "ready"; files: DiffFile[]; truncated: boolean };

export type ReviewedRepository = {
	repository: string;
	branch?: string;
	baseSha: string;
	headSha: string;
	commits: ReviewCommit[];
	additions: number;
	deletions: number;
	artifactId?: string;
	diff: RepositoryDiff;
};

export type ReviewView =
	| { kind: "loading" }
	| { kind: "not_found" }
	| { kind: "revision_not_found"; executionId: string }
	| { kind: "unavailable" }
	| {
			kind: "ready";
			execution: Execution;
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
				? `This change touches more than ${diffFilesMax.toLocaleString("en")} files. The first ${diffFilesMax.toLocaleString("en")} are listed here; the rest are in the full diff.`
				: undefined;
	}
}

export function fileId(repositoryIndex: number, fileIndex: number): string {
	return `file-${repositoryIndex}-${fileIndex}`;
}

export function reviewOpen(execution: Execution): boolean {
	return execution.state === "awaiting_review";
}

export function readingLatest(review: ReviewState): boolean {
	return review.revision === review.latestRevision;
}

export function revisionLabel(revision: number): string {
	return `Revision ${revision}`;
}

export function olderRevisionLine(review: ReviewState): string {
	return `You are reading revision ${review.revision} of ${review.latestRevision}. It is kept as it was; comments and decisions go on the latest.`;
}

export type PreviewsView =
	| { kind: "none_configured" }
	| { kind: "listed"; previews: ReviewPreview[] };

export function previewsView(review: ReviewState): PreviewsView {
	if (review.previews.length === 0) return { kind: "none_configured" };

	return { kind: "listed", previews: review.previews };
}

export const noPlanPreviewsLine =
	"This codebase's Run Plan declares no previews, so there is nothing running to open. Add services and previews to .norn/run-plan.yaml to get them on every review.";

export function reviewPreviewLine(preview: ReviewPreview): string {
	switch (preview.state) {
		case "ready":
			return preview.url
				? `Running on ${preview.service} and healthy.`
				: `Running on ${preview.service} and healthy, but this server serves no preview domain, so nothing reaches it.`;
		case "failed":
			return preview.reason
				? `${preview.service} did not come up: ${preview.reason}`
				: `${preview.service} did not come up.`;
		case "unsupported":
			return preview.reason
				? `Not supported here: ${preview.reason}`
				: "Not supported here.";
	}
}

export function unchangedTitle(review: ReviewState): string {
	return `Nothing changed since revision ${review.revision - 1}`;
}

export const unchangedLine =
	"These are the commits you already reviewed. What the coding agent said below explains why.";

const previewFixReasonMax = 300;

export function failedPreviews(review: ReviewState): ReviewPreview[] {
	return review.previews.filter((preview) => preview.state === "failed");
}

export function previewFixRequest(failed: ReviewPreview[]): string {
	const lines = failed.map((preview) => {
		const reason = preview.reason?.trim() || "no reason was given";
		const clipped =
			[...reason].length > previewFixReasonMax ? [...reason].slice(0, previewFixReasonMax).join("") + "…" : reason;

		return `- ${preview.name}: ${clipped}`;
	});

	const request =
		"These previews did not start, so nobody could try the change:\n\n" +
		lines.join("\n") +
		"\n\nMake them start. If the fix is in the code, change it on this branch; " +
		"if it is in how the codebase is run, say what has to change.";

	return [...request].slice(0, reviewBodyMaxLength).join("");
}

export function previewFixLabel(agentName: string | undefined): string {
	return `Ask ${agentName || "the coding agent"} to fix the preview`;
}

export function previewStateLabel(preview: ReviewPreview): string {
	switch (preview.state) {
		case "ready":
			return "Ready";
		case "failed":
			return "Failed";
		case "unsupported":
			return "Unsupported";
	}
}

export function snapshotTotals(repositories: ReviewRepository[]): ChangeTotals {
	return repositories.reduce(
		(running, held) => ({
			repositories: running.repositories + 1,
			commits: running.commits + held.commits.length,
			additions: running.additions + held.additions,
			deletions: running.deletions + held.deletions,
			filesChanged: running.filesChanged + held.filesChanged,
		}),
		{ repositories: 0, commits: 0, additions: 0, deletions: 0, filesChanged: 0 }
	);
}

export function shortSha(sha: string): string {
	return sha.slice(0, 7);
}

export function commitsLine(commits: ReviewCommit[]): string {
	return commits.length === 1 ? "1 commit" : `${commits.length} commits`;
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
