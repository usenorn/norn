import type { components } from "$lib/api/dashboard.gen";

export type Attachment = components["schemas"]["Attachment"];
type DocumentNode = components["schemas"]["DocumentNode"];
export type AttachmentTransfer = components["schemas"]["AttachmentTransfer"];
export type WorkspaceStorage = components["schemas"]["WorkspaceStorage"];

export type AttachmentPanel =
	| { kind: "loading" }
	| { kind: "empty" }
	| { kind: "ready"; attachments: Attachment[] }
	| { kind: "unavailable" };

export type AttachmentFailure =
	| { kind: "too_large"; maxBytes: number }
	| { kind: "workspace_full"; storedBytes: number; maxBytes: number }
	| { kind: "expired" }
	| { kind: "gone" }
	| { kind: "forbidden" }
	| { kind: "invalid" }
	| { kind: "unavailable" };

const units = ["B", "KB", "MB", "GB", "TB"];

export function formatBytes(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;

	let value = bytes;
	let unit = 0;

	while (value >= 1024 && unit < units.length - 1) {
		value /= 1024;
		unit += 1;
	}

	const rounded = Number(value.toFixed(1));

	return `${rounded >= 10 ? Math.round(rounded) : rounded} ${units[unit]}`;
}

export function attachmentMarkdown(attachment: Attachment): string {
	const link = `[${attachment.fileName}](${attachment.contentPath})`;

	return attachment.inline ? `!${link}` : link;
}

/**
 * A stored file becomes a node carrying its identifier, so the text says which file it shows
 * rather than only where the bytes happen to live today.
 */
export function attachmentNode(attachment: Attachment): DocumentNode {
	if (attachment.inline) {
		return {
			type: "image",
			attrs: {
				src: attachment.contentPath,
				alt: attachment.fileName,
				attachmentId: attachment.id,
			},
		};
	}

	return {
		type: "attachment",
		attrs: {
			attachmentId: attachment.id,
			fileName: attachment.fileName,
			href: attachment.contentPath,
			contentType: attachment.contentType,
			byteSize: attachment.byteSize,
		},
	};
}

export function attachmentFailureMessage(failure: AttachmentFailure): string {
	switch (failure.kind) {
		case "too_large":
			return `That file is bigger than ${formatBytes(failure.maxBytes)}, which is the most one file may be.`;
		case "workspace_full":
			return `This workspace is storing ${formatBytes(failure.storedBytes)} of ${formatBytes(failure.maxBytes)}. Remove a file to make room.`;
		case "expired":
			return "That upload took too long to start. Try it again.";
		case "gone":
			return "This file is no longer here.";
		case "forbidden":
			return "You cannot attach files to this issue.";
		case "invalid":
			return "Norn could not accept that file's name.";
		default:
			return "Nothing was saved. Wait a moment and try again.";
	}
}

export function readAttachmentFailure(error: unknown): AttachmentFailure {
	if (!error || typeof error !== "object") return { kind: "unavailable" };

	const problem = error as {
		code?: string;
		byteSize?: number;
		storedBytes?: number;
		maxBytes?: number;
		errors?: { field?: string }[];
		status?: number;
	};

	switch (problem.code) {
		case "attachment_too_large":
			return { kind: "too_large", maxBytes: problem.maxBytes ?? 0 };
		case "workspace_storage_exhausted":
			return {
				kind: "workspace_full",
				storedBytes: problem.storedBytes ?? 0,
				maxBytes: problem.maxBytes ?? 0,
			};
		case "attachment_missing":
		case "attachment_not_pending":
			return { kind: "expired" };
	}

	if (problem.errors) return { kind: "invalid" };
	if (problem.status === 403) return { kind: "forbidden" };
	if (problem.status === 404) return { kind: "gone" };
	if (problem.status === 410) return { kind: "expired" };

	return { kind: "unavailable" };
}
