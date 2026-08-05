import { apiFor } from "$lib/api";
import { noAuditFilters, type AuditFilters, type AuditRecord } from "$lib/audit/audit";
import type { PageLoad } from "./$types";

export type AuditPageData = {
	record: AuditRecord;
	filters: AuditFilters;
};

export const load: PageLoad = async ({ fetch, parent, url }): Promise<AuditPageData> => {
	const api = apiFor(url);
	const { workspace } = await parent();

	const availability = await api.GET("/workspaces/{workspaceId}/audit/availability", {
		fetch,
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
			record: { kind: "unlicensed", retentionDays, holder: availability.data?.holder },
		};
	}

	const page = await api.GET("/workspaces/{workspaceId}/audit", {
		fetch,
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
