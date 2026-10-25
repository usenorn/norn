import type { DocFailure } from "#lib/docs/docs.js";

export type NewDocPreview = { failure?: DocFailure; submitting?: boolean };

export const newDocPreviewStates: Record<string, NewDocPreview> = import.meta.env.DEV
	? {
			submitting: { submitting: true },
			not_shareable: { failure: { kind: "not_shareable" } },
			unavailable: { failure: { kind: "unavailable" } },
		}
	: {};
