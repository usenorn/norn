import { keys } from "$lib/api/keys";
import {
	configured,
	memberPageSize,
	type ImportRun,
	type ImportRunView,
	type ImportTargets,
} from "$lib/imports/imports";
import type { Team } from "$lib/team/teams";
import type { PageServerLoad } from "./$types";

export type ImportRunPageData = {
	view: ImportRunView;
};

type Api = App.Locals["api"];

function resumesAt(response: Response): string | undefined {
	const seconds = Number(response.headers.get("retry-after"));

	if (!Number.isFinite(seconds) || seconds <= 0) return undefined;

	return new Date(Date.now() + seconds * 1000).toISOString();
}

async function targetsFor(
	api: Api,
	workspaceId: string,
	teams: Team[]
): Promise<ImportTargets> {
	const path = { workspaceId };
	const active = teams.filter((team) => team.status === "active");

	const [members, projects, labels, states] = await Promise.all([
		api.GET("/workspaces/{workspaceId}/members", {
			params: { path, query: { limit: memberPageSize } },
		}),
		api.GET("/workspaces/{workspaceId}/projects", { params: { path } }),
		api.GET("/workspaces/{workspaceId}/labels", { params: { path } }),
		Promise.all(
			active.map((team) =>
				api.GET("/workspaces/{workspaceId}/teams/{teamId}/states", {
					params: { path: { workspaceId, teamId: team.id } },
				})
			)
		),
	]);

	return {
		members: (members.data?.members ?? [])
			.filter((member) => !member.deactivatedAt)
			.map((member) => ({
				id: member.accountId,
				name: member.displayName ?? member.email ?? member.accountId,
				detail: member.email,
			})),
		teams: active.map((team) => ({ id: team.id, name: team.name, detail: team.key })),
		projects: (projects.data ?? []).map((project) => ({
			id: project.id,
			name: project.name,
		})),
		labels: (labels.data ?? []).map((label) => ({ id: label.id, name: label.name })),
		states: states.flatMap((listed, index) =>
			(listed.data ?? []).map((state) => ({
				id: state.id,
				name: state.name,
				detail: active[index]?.key,
			}))
		),
	};
}

async function draftView(api: Api, workspaceId: string, run: ImportRun): Promise<ImportRunView> {
	if (!configured(run)) return { kind: "connect", run };

	const catalogue = await api.GET("/workspaces/{workspaceId}/imports/{importRunId}/catalogue", {
		params: { path: { workspaceId, importRunId: run.id } },
	});

	if (catalogue.error) {
		switch (catalogue.response.status) {
			case 429:
				return { kind: "rate_limited", run, resumesAt: resumesAt(catalogue.response) };
			case 502:
				return {
					kind: "source_refused",
					run,
					reason: "reason" in catalogue.error ? catalogue.error.reason : undefined,
				};
			case 503:
				return { kind: "encryption_unavailable", run };
			case 403:
				return { kind: "forbidden" };
			default:
				return { kind: "unavailable" };
		}
	}

	return {
		kind: "catalogue",
		run,
		catalogue: catalogue.data ?? { scopes: [], columns: [], notes: [] },
	};
}

async function reportView(
	api: Api,
	workspaceId: string,
	run: ImportRun,
	kind: "imported" | "reverted" | "failed"
): Promise<ImportRunView> {
	const report = await api.GET("/workspaces/{workspaceId}/imports/{importRunId}/report", {
		params: { path: { workspaceId, importRunId: run.id } },
	});

	if (!report.data) return { kind: "unavailable" };

	return { kind, run, report: report.data };
}

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	params,
	parent,
}): Promise<ImportRunPageData> => {
	depends(keys.page(route.id));

	const { workspace, teams } = await parent();

	const path = { workspaceId: workspace.id, importRunId: params.importRunId };

	const found = await locals.api.GET("/workspaces/{workspaceId}/imports/{importRunId}", {
		params: { path },
	});

	if (found.error || !found.data) {
		if (found.response.status === 403) return { view: { kind: "forbidden" } };
		if (found.response.status === 404) return { view: { kind: "not_found" } };

		return { view: { kind: "unavailable" } };
	}

	const run = found.data;

	switch (run.status) {
		case "draft":
			return { view: await draftView(locals.api, workspace.id, run) };

		case "staging":
			return { view: { kind: "staging", run } };

		case "staged": {
			const [plan, targets] = await Promise.all([
				locals.api.GET("/workspaces/{workspaceId}/imports/{importRunId}/mappings", {
					params: { path },
				}),
				targetsFor(locals.api, workspace.id, teams ?? []),
			]);

			if (!plan.data) return { view: { kind: "unavailable" } };

			return { view: { kind: "staged", run, plan: plan.data, targets } };
		}

		case "mapped": {
			const preview = await locals.api.GET(
				"/workspaces/{workspaceId}/imports/{importRunId}/preview",
				{ params: { path } }
			);

			if (!preview.data) return { view: { kind: "unavailable" } };

			if (preview.data.triageTeams.length > 0 && !run.acknowledgeTriage) {
				return {
					view: {
						kind: "triage_ack",
						run,
						preview: preview.data,
						teams: preview.data.triageTeams,
					},
				};
			}

			return { view: { kind: "preview", run, preview: preview.data } };
		}

		case "executing":
			return { view: { kind: "executing", run } };

		case "reverting":
			return { view: { kind: "reverting", run } };

		case "imported":
			return { view: await reportView(locals.api, workspace.id, run, "imported") };

		case "reverted":
			return { view: await reportView(locals.api, workspace.id, run, "reverted") };

		case "failed":
			return { view: await reportView(locals.api, workspace.id, run, "failed") };
	}
};
