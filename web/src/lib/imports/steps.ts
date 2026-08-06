import { api } from "$lib/api";
import {
	importFailure,
	type ImportFailure,
	type ImportFile,
	type ImportRun,
	type ImportRunView,
	type ImportTargets,
} from "./imports";

export type StepResult = { ok: true; view: ImportRunView } | { ok: false; failure: ImportFailure };

export function resumesAt(response: Response): string | undefined {
	const seconds = Number(response.headers.get("retry-after"));

	if (!Number.isFinite(seconds) || seconds <= 0) return undefined;

	return new Date(Date.now() + seconds * 1000).toISOString();
}

export async function readCatalogue(workspaceId: string, run: ImportRun): Promise<StepResult> {
	const { data, error, response } = await api.GET(
		"/workspaces/{workspaceId}/imports/{importRunId}/catalogue",
		{ params: { path: { workspaceId, importRunId: run.id } } }
	);

	if (error) {
		const failure = importFailure(error, resumesAt(response));

		switch (failure.kind) {
			case "rate_limited":
				return { ok: true, view: { kind: "rate_limited", run, resumesAt: failure.resumesAt } };
			case "source_refused":
				return { ok: true, view: { kind: "source_refused", run, reason: failure.reason } };
			case "encryption_unavailable":
				return { ok: true, view: { kind: "encryption_unavailable", run } };
			default:
				return { ok: false, failure };
		}
	}

	return {
		ok: true,
		view: { kind: "catalogue", run, catalogue: data ?? { scopes: [], columns: [], notes: [] } },
	};
}

export async function readPlan(
	workspaceId: string,
	run: ImportRun,
	targets: ImportTargets
): Promise<StepResult> {
	const { data, error } = await api.GET(
		"/workspaces/{workspaceId}/imports/{importRunId}/mappings",
		{ params: { path: { workspaceId, importRunId: run.id } } }
	);

	if (error) return { ok: false, failure: importFailure(error) };
	if (!data) return { ok: false, failure: { kind: "unavailable" } };

	return { ok: true, view: { kind: "mapping", run, plan: data, targets } };
}

export async function readStalePreview(workspaceId: string, run: ImportRun): Promise<StepResult> {
	const { data, error } = await api.GET("/workspaces/{workspaceId}/imports/{importRunId}/preview", {
		params: { path: { workspaceId, importRunId: run.id } },
	});

	if (error) return { ok: false, failure: importFailure(error) };
	if (!data) return { ok: false, failure: { kind: "unavailable" } };

	return { ok: true, view: { kind: "preview_stale", run, preview: data } };
}

export async function stageRun(workspaceId: string, runId: string): Promise<ImportFailure | null> {
	const { data, error } = await api.POST("/workspaces/{workspaceId}/imports/{importRunId}/stage", {
		params: { path: { workspaceId, importRunId: runId } },
	});

	if (error) return importFailure(error);
	if (!data) return { kind: "unavailable" };

	return null;
}

export type UploadResult =
	| { ok: true; file: ImportFile }
	| { ok: false; failure: ImportFailure };

export async function uploadFile(
	workspaceId: string,
	runId: string,
	file: File
): Promise<UploadResult> {
	const { data, error } = await api.POST("/workspaces/{workspaceId}/imports/{importRunId}/file", {
		params: { path: { workspaceId, importRunId: runId } },
		body: { file: file.name },
		bodySerializer: () => {
			const form = new FormData();
			form.append("file", file);

			return form;
		},
	});

	if (error) return { ok: false, failure: importFailure(error) };
	if (!data) return { ok: false, failure: { kind: "unavailable" } };

	return { ok: true, file: data };
}
