import { attachmentMarkdown, type Attachment } from "$lib/attachments/attachments";
import { newTask, upload } from "$lib/attachments/upload";

export type PendingFile = { key: string; name: string; size: number; file: File };

export type AttachOutcome = { markdown: string; attached: Attachment[]; failed: string[] };

export function describedWith(description: string, markdown: string): string {
	if (!markdown) return description;

	return description.trim() ? `${description.trim()}\n\n${markdown}` : markdown;
}

export function attachFailureMessage(failed: string[]): string {
	if (failed.length === 1) {
		return `${failed[0]} could not be attached. The issue was created without it.`;
	}

	return `${failed.length} files could not be attached. The issue was created without them.`;
}

export async function attachPending(
	workspaceId: string,
	issueId: string,
	files: PendingFile[]
): Promise<AttachOutcome> {
	const attached: Attachment[] = [];
	const failed: string[] = [];

	for (const held of files) {
		let settled = newTask(held.key, held.file);

		await upload(
			{ workspaceId, issueId },
			held.file,
			settled,
			(task) => (settled = task),
			() => {}
		);

		if (settled.state === "done" && settled.attachment) {
			attached.push(settled.attachment);
		} else {
			failed.push(held.name);
		}
	}

	return { markdown: attached.map(attachmentMarkdown).join("\n\n"), attached, failed };
}
