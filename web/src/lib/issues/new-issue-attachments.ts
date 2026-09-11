import { api } from "$lib/api";
import { attachmentNode, type Attachment } from "$lib/attachments/attachments";
import { documentAttachments, type Document } from "$lib/editor/document";
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

	const held = new Set(documentAttachments(description));
	const arriving = attached
		.filter((attachment) => !held.has(attachment.id))
		.map(attachmentNode);

	if (arriving.length === 0) return description;

	return { type: "doc", content: [...(description.content ?? []), ...arriving] };
}

export function describedAll(description: Document, attached: Attachment[]): boolean {
	const held = new Set(documentAttachments(description));

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

export const uploadsAtOnce = 4;

export async function attachPending(
	workspaceId: string,
	issueId: string,
	files: PendingFile[],
	onprogress: (tasks: UploadTask[]) => void = () => {},
	register: (key: string, abort: () => void) => void = () => {}
): Promise<AttachOutcome> {
	const tasks = files.map((held) => newTask(held.key, held.file));

	onprogress([...tasks]);

	const resumed = files.some((held) => held.sentAlready)
		? await attachmentsOn(workspaceId, issueId)
		: [];

	let next = 0;

	async function work() {
		for (let index = next++; index < files.length; index = next++) {
			const held = files[index];
			const already = held.sentAlready
				? resumed.find((candidate) => candidate.id === held.sentAlready)
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
		}
	}

	await Promise.all(
		Array.from({ length: Math.min(uploadsAtOnce, files.length) }, () => work())
	);

	const attached: Attachment[] = [];
	const failed: PendingFile[] = [];
	const cancelled: PendingFile[] = [];

	files.forEach((held, index) => {
		const settled = tasks[index];

		if (settled.state === "done" && settled.attachment) {
			attached.push(settled.attachment);
		} else if (settled.state === "cancelled") {
			cancelled.push({ ...held, sentAlready: undefined });
		} else {
			failed.push({ ...held, sentAlready: settled.attachmentId });
		}
	});

	return { attached, failed, cancelled };
}

async function attachmentsOn(workspaceId: string, issueId: string): Promise<Attachment[]> {
	try {
		const listed = await api.GET("/workspaces/{workspaceId}/issues/{issueId}/attachments", {
			params: { path: { workspaceId, issueId } },
		});

		if (listed.error || !listed.data) return [];

		return listed.data.attachments;
	} catch {
		return [];
	}
}
