import { attachmentMarkdown, type Attachment } from "$lib/attachments/attachments";
import { newTask, upload, type UploadTask } from "$lib/attachments/upload";

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
	files: PendingFile[],
	onprogress: (tasks: UploadTask[]) => void = () => {}
): Promise<AttachOutcome> {
	const attached: Attachment[] = [];
	const failed: string[] = [];
	const tasks = files.map((held) => newTask(held.key, held.file));

	onprogress([...tasks]);

	for (const [index, held] of files.entries()) {
		await upload(
			{ workspaceId, issueId },
			held.file,
			tasks[index],
			(task) => {
				tasks[index] = task;
				onprogress([...tasks]);
			},
			() => {}
		);

		const settled = tasks[index];

		if (settled.state === "done" && settled.attachment) {
			attached.push(settled.attachment);
		} else {
			failed.push(held.name);
		}
	}

	return { markdown: attached.map(attachmentMarkdown).join("\n\n"), attached, failed };
}
