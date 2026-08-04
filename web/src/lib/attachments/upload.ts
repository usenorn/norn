import { api } from "$lib/api";
import {
	readAttachmentFailure,
	type Attachment,
	type AttachmentFailure,
	type AttachmentTransfer,
} from "$lib/attachments/attachments";

export type UploadState = "reserving" | "sending" | "finalizing" | "done" | "failed" | "cancelled";

export type UploadTask = {
	id: string;
	name: string;
	size: number;
	sent: number;
	state: UploadState;
	failure?: AttachmentFailure;
	attachment?: Attachment;
};

export type UploadTarget = { workspaceId: string; issueId: string };

export function newTask(id: string, file: File): UploadTask {
	return { id, name: file.name, size: file.size, sent: 0, state: "reserving" };
}

export function settled(task: UploadTask): boolean {
	return task.state === "done" || task.state === "failed" || task.state === "cancelled";
}

function send(
	transfer: AttachmentTransfer,
	file: File,
	onprogress: (sent: number) => void,
	register: (abort: () => void) => void
): Promise<number> {
	return new Promise((resolve, reject) => {
		const request = new XMLHttpRequest();

		register(() => request.abort());

		request.open(transfer.method, transfer.url, true);

		for (const [name, value] of Object.entries(transfer.headers)) {
			request.setRequestHeader(name, value);
		}

		request.upload.onprogress = (event) => onprogress(event.loaded);
		request.onload = () =>
			request.status >= 200 && request.status < 300
				? resolve(request.status)
				: reject(new Error(String(request.status)));
		request.onerror = () => reject(new Error("network"));
		request.onabort = () => reject(new Error("aborted"));
		request.send(file);
	});
}

export async function upload(
	target: UploadTarget,
	file: File,
	task: UploadTask,
	update: (task: UploadTask) => void,
	register: (abort: () => void) => void
): Promise<void> {
	const path = { workspaceId: target.workspaceId, issueId: target.issueId };

	const reserved = await api.POST("/workspaces/{workspaceId}/issues/{issueId}/attachments", {
		params: { path },
		body: { fileName: file.name, contentType: file.type || undefined, byteSize: file.size },
	});

	if (reserved.error || !reserved.data) {
		update({ ...task, state: "failed", failure: readAttachmentFailure(reserved.error) });

		return;
	}

	const attachmentId = reserved.data.attachment.id;
	update({ ...task, state: "sending" });

	try {
		await send(
			reserved.data.transfer,
			file,
			(sent) => update({ ...task, state: "sending", sent }),
			register
		);
	} catch (failure) {
		const cancelled = failure instanceof Error && failure.message === "aborted";

		update({
			...task,
			state: cancelled ? "cancelled" : "failed",
			failure: cancelled ? undefined : { kind: "unavailable" },
		});

		await api.DELETE("/workspaces/{workspaceId}/issues/{issueId}/attachments/{attachmentId}", {
			params: { path: { ...path, attachmentId } },
		});

		return;
	}

	update({ ...task, state: "finalizing", sent: file.size });

	const finalized = await api.POST(
		"/workspaces/{workspaceId}/issues/{issueId}/attachments/{attachmentId}/finalize",
		{ params: { path: { ...path, attachmentId } } }
	);

	if (finalized.error || !finalized.data) {
		update({ ...task, state: "failed", failure: readAttachmentFailure(finalized.error) });

		return;
	}

	update({ ...task, state: "done", sent: file.size, attachment: finalized.data });
}
