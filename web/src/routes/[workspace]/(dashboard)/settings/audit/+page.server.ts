import { noAuditFilters, type AuditFilters, type AuditRecord } from "$lib/audit/audit";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type AuditPageData = {
	record: AuditRecord;
	filters: AuditFilters;
};

export const load: PageServerLoad = async ({ depends, route, locals, parent }): Promise<AuditPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const availability = await locals.api.GET("/workspaces/{workspaceId}/audit/availability", {
		params: { path: { workspaceId: workspace.id } },
	});

	const retentionDays = availability.data?.retentionDays ?? 0;

	if (availability.error) {
		return {
			filters: noAuditFilters,
			record: { kind: availability.error.status === 403 ? "forbidden" : "unavailable" },
		};
	}

	if (!availability.data?.available) {
		return {
			filters: noAuditFilters,
			record: { kind: "unlicensed", retentionDays },
		};
	}

	const page = await locals.api.GET("/workspaces/{workspaceId}/audit", {
		params: { path: { workspaceId: workspace.id } },
	});

	if (page.error) {
		if (page.response.status === 403) {
			return { filters: noAuditFilters, record: { kind: "forbidden" } };
		}

		return { filters: noAuditFilters, record: { kind: "unavailable" } };
	}

	if (!page.data || page.data.events.length === 0) {
		return { filters: noAuditFilters, record: { kind: "empty", retentionDays } };
	}

	return {
		filters: noAuditFilters,
		record: {
			kind: "ready",
			events: page.data.events,
			nextCursor: page.data.nextCursor,
			retentionDays: page.data.retentionDays,
		},
	};
};
