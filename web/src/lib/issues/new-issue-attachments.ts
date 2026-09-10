import { api } from "$lib/api";
import { attachmentNode, type Attachment } from "$lib/attachments/attachments";
import type { Document } from "$lib/editor/document";
import { newTask, upload, type UploadTask } from "$lib/attachments/upload";

export type PendingFile = {
	key: string;
	name: string;
	size: number;
	file: File;
	sentAlready?: string;
};

export type AttachOutcome = {
	attached: Attachment[];
	failed: PendingFile[];
	cancelled: PendingFile[];
};

export function pendingFrom(files: File[], next: () => string): PendingFile[] {
	return files.map((file) => ({ key: next(), name: file.name, size: file.size, file }));
}

export function describedWith(description: Document, attached: Attachment[]): Document {
	if (attached.length === 0) return description;

	const held = new Set(attachmentsIn(description));
	const arriving = attached
		.filter((attachment) => !held.has(attachment.id))
		.map(attachmentNode);

	if (arriving.length === 0) return description;

	return { type: "doc", content: [...(description.content ?? []), ...arriving] };
}

export function attachmentsIn(description: Document): string[] {
	const found: string[] = [];

	for (const node of description.content ?? []) {
		const id = (node.attrs as Record<string, unknown> | undefined)?.attachmentId;

		if (typeof id === "string" && id !== "") found.push(id);
	}

	return found;
}

export function describedAll(description: Document, attached: Attachment[]): boolean {
	const held = new Set(attachmentsIn(description));

	return attached.every((attachment) => held.has(attachment.id));
}

export function attachFailureMessage(failed: PendingFile[]): string {
	if (failed.length === 1) {
		return `${failed[0].name} did not upload. The issue is saved; try that file again.`;
	}

	return `${failed.length} files did not upload. The issue is saved; try them again.`;
}

export function describeFailureMessage(kind: "conflict" | "unavailable" | "uncertain"): string {
	if (kind === "uncertain") {
		return (
			"The files are attached, but we could not tell whether the links reached the " +
			"description. Open the issue to check before trying again."
		);
	}

	if (kind === "conflict") {
		return (
			"The files are attached, but somebody else edited the description first, so the links " +
			"were not added. Open the issue to put them where you want them."
		);
	}

	return "The files are attached, but the links could not be added to the description.";
}

export async function attachPending(
	workspaceId: string,
	issueId: string,
	files: PendingFile[],
	onprogress: (tasks: UploadTask[]) => void = () => {},
	register: (key: string, abort: () => void) => void = () => {}
): Promise<AttachOutcome> {
	const attached: Attachment[] = [];
	const failed: PendingFile[] = [];
	const cancelled: PendingFile[] = [];
	const tasks = files.map((held) => newTask(held.key, held.file));

	onprogress([...tasks]);

	for (const [index, held] of files.entries()) {
		const already = held.sentAlready
			? await alreadyAttached(workspaceId, issueId, held.sentAlready)
			: undefined;

		if (already) {
			tasks[index] = {
				...tasks[index],
				state: "done",
				sent: held.size,
				attachmentId: already.id,
				attachment: already,
			};
			onprogress([...tasks]);
			attached.push(already);

			continue;
		}

		try {
			await upload(
				{ workspaceId, issueId },
				held.file,
				tasks[index],
				(task) => {
					tasks[index] = task;
					onprogress([...tasks]);
				},
				(abort) => register(held.key, abort),
				held.sentAlready
			);
		} catch {
			tasks[index] = { ...tasks[index], state: "failed", failure: { kind: "unavailable" } };
			onprogress([...tasks]);
		}

		const settled = tasks[index];

		if (settled.state === "done" && settled.attachment) {
			attached.push(settled.attachment);
		} else if (settled.state === "cancelled") {
			cancelled.push({ ...held, sentAlready: undefined });
		} else {
			failed.push({ ...held, sentAlready: settled.attachmentId });
		}
	}

	return { attached, failed, cancelled };
}

async function alreadyAttached(
	workspaceId: string,
	issueId: string,
	attachmentId: string
): Promise<Attachment | undefined> {
	try {
		const listed = await api.GET("/workspaces/{workspaceId}/issues/{issueId}/attachments", {
			params: { path: { workspaceId, issueId } },
		});

		if (listed.error || !listed.data) return undefined;

		return listed.data.attachments.find((held) => held.id === attachmentId);
	} catch {
		return undefined;
	}
}
