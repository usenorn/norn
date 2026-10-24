import type { DocEditing } from "$lib/docs/docs";
import type { SaveState } from "$lib/editor/autosave.svelte";
import type { Document } from "$lib/editor/document";

export type DocEditPreview = {
	editing: DocEditing;
	save?: SaveState;
	conflict?: { mine: Document; theirs: Document };
};

export const docEditPreviewStates: Record<string, DocEditPreview> = import.meta.env.DEV
	? {
			loading: { editing: { kind: "loading" } },
			unavailable: { editing: { kind: "unavailable" } },
			not_found: { editing: { kind: "not_found" } },
			...Object.fromEntries(
				(["ready", "waiting", "saving", "saved", "failed", "unknown", "conflict"] as const).map((state) => [
					state,
					{
						editing: {
							kind: "ready",
							doc: {
								id: "00000000-0000-4000-8000-000000000a01",
								title: "On-call runbook",
								audience: { visibility: "workspace" },
								authorAccountId: "00000000-0000-4000-8000-000000000201",
								authorName: "Rae Okafor",
								status: "active",
								version: 4,
								createdAt: "2026-09-01T09:00:00Z",
								updatedAt: "2026-10-20T09:00:00Z",
								markdown: "Acknowledge within five minutes, then open the incident channel.",
								content: {
									type: "doc",
									content: [
										{
											type: "paragraph",
											content: [
												{
													type: "text",
													text: "Acknowledge within five minutes, then open the incident channel.",
												},
											],
										},
									],
								},
								contributorIds: [],
								viewer: { canEdit: true, canShare: true },
							},
						},
						...(state === "ready" ? {} : { save: { kind: state } }),
						...(state === "conflict"
							? {
									conflict: {
										mine: {
											type: "doc",
											content: [
												{
													type: "paragraph",
													content: [
														{ type: "text", text: "Acknowledge within two minutes, then page the payments lead." },
													],
												},
											],
										},
										theirs: {
											type: "doc",
											content: [
												{
													type: "paragraph",
													content: [
														{ type: "text", text: "Acknowledge within five minutes, then open the incident channel." },
													],
												},
											],
										},
									},
								}
							: {}),
					} satisfies DocEditPreview,
				])
			),
		}
	: {};
