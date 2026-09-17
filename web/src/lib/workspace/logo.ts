import { api } from "$lib/api";
import { attempt } from "$lib/api/attempt";
import { withSlot } from "$lib/account/accounts";
import type { Workspace } from "$lib/workspace/settings";

export type LogoFailure = "too_large" | "unsupported" | "too_small" | "forbidden" | "unavailable";

export type LogoActivity =
	| { kind: "idle" }
	| { kind: "uploading" }
	| { kind: "removing" }
	| { kind: "failed"; failure: LogoFailure };

export type LogoOutcome = { kind: "saved"; workspace: Workspace } | { kind: "failed"; failure: LogoFailure };

export const logoAccept = "image/png,image/jpeg,image/webp";

export function logoSource(logoUrl: string | undefined, slot: string | null | undefined): string | undefined {
	return logoUrl ? withSlot(logoUrl, slot ?? null) : undefined;
}

export function logoFailureMessage(failure: LogoFailure): string {
	switch (failure) {
		case "too_large":
			return "That image is larger than 2 MB.";
		case "unsupported":
			return "Use a PNG, JPEG or WebP image.";
		case "too_small":
			return "The image needs to be at least 128px on each side.";
		case "forbidden":
			return "Only administrators can change the logo.";
		case "unavailable":
			return "The logo did not change. Wait a moment and try again.";
	}
}

function failureOf(status: number): LogoFailure {
	switch (status) {
		case 413:
			return "too_large";
		case 415:
			return "unsupported";
		case 422:
			return "too_small";
		case 403:
			return "forbidden";
		default:
			return "unavailable";
	}
}

export async function uploadLogo(workspaceId: string, file: File): Promise<LogoOutcome> {
	const outcome = await attempt({
		run: () =>
			api.PUT("/workspaces/{workspaceId}/logo", {
				params: { path: { workspaceId } },
				body: { file: file.name },
				bodySerializer: () => {
					const form = new FormData();
					form.append("file", file);

					return form;
				},
			}),
	});

	if (outcome.kind === "done") return { kind: "saved", workspace: outcome.value };
	if (outcome.kind === "refused") return { kind: "failed", failure: failureOf(outcome.status) };

	return { kind: "failed", failure: "unavailable" };
}

export async function removeLogo(workspaceId: string): Promise<LogoOutcome> {
	const outcome = await attempt({
		run: () =>
			api.DELETE("/workspaces/{workspaceId}/logo", {
				params: { path: { workspaceId } },
			}),
	});

	if (outcome.kind === "done") return { kind: "saved", workspace: outcome.value };
	if (outcome.kind === "refused") return { kind: "failed", failure: failureOf(outcome.status) };

	return { kind: "failed", failure: "unavailable" };
}
