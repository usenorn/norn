import type { components } from "$lib/api/dashboard.gen";

export type IssueComment = components["schemas"]["IssueComment"];
export type CommentMention = components["schemas"]["CommentMention"];
export type CommentReaction = components["schemas"]["CommentReaction"];
export type CommentReactionTally = components["schemas"]["CommentReactionTally"];
export type MentionTarget = components["schemas"]["MentionTarget"];

export type CommentThread =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; comments: IssueComment[]; nextCursor?: string }
	| { kind: "unavailable" };

export type CommentFailure =
	| { kind: "deleted" }
	| { kind: "not_replyable" }
	| { kind: "not_yours" }
	| { kind: "invalid"; fields: string[] }
	| { kind: "gone" }
	| { kind: "unavailable" };

export const reactions: CommentReaction[] = [
	"up",
	"down",
	"celebrate",
	"thinking",
	"eyes",
	"heart",
];

export const reactionGlyphs: Record<CommentReaction, string> = {
	up: "👍",
	down: "👎",
	celebrate: "🎉",
	thinking: "🤔",
	eyes: "👀",
	heart: "❤️",
};

export const reactionLabels: Record<CommentReaction, string> = {
	up: "Agree",
	down: "Disagree",
	celebrate: "Celebrate",
	thinking: "Thinking about it",
	eyes: "Looking at it",
	heart: "Love it",
};

const failureMessages: Record<CommentFailure["kind"], string> = {
	deleted: "This comment was deleted while you were looking at it.",
	not_replyable: "Replies go on a top-level comment, not on another reply.",
	not_yours: "Only the person who wrote a comment can edit it.",
	invalid: "Write something first — a comment cannot be empty.",
	gone: "This comment is no longer here.",
	unavailable: "Nothing was saved. Wait a moment and try again.",
};

export function commentFailureMessage(failure: CommentFailure): string {
	return failureMessages[failure.kind];
}

export function readCommentFailure(error: unknown): CommentFailure {
	if (!error || typeof error !== "object") return { kind: "unavailable" };

	const problem = error as {
		code?: string;
		errors?: { field?: string }[];
		status?: number;
	};

	if (problem.code === "comment_deleted") return { kind: "deleted" };
	if (problem.code === "comment_not_replyable") return { kind: "not_replyable" };

	if (problem.errors) {
		return {
			kind: "invalid",
			fields: problem.errors.map((entry) => entry.field ?? "").filter(Boolean),
		};
	}

	if (problem.status === 403) return { kind: "not_yours" };
	if (problem.status === 404) return { kind: "gone" };

	return { kind: "unavailable" };
}

export function authorLabel(comment: IssueComment): string {
	return comment.authorName || "Someone who has left";
}

export function reacted(tally: CommentReactionTally, accountId: string): boolean {
	return tally.accountIds.includes(accountId);
}

export function unreachableLine(mentions: CommentMention[]): string {
	const names = mentions.map((mention) => mention.name);

	if (names.length === 0) return "";

	const listed =
		names.length === 1
			? names[0]
			: `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;

	return `${listed} cannot see this issue, so they were not notified.`;
}
