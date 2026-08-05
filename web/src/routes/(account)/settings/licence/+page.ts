import { apiFor } from "$lib/api";
import type { LicenceView } from "$lib/licence/licence";
import type { PageLoad } from "./$types";

export type LicencePageData = {
	view: LicenceView;
};

export const load: PageLoad = async ({ fetch, url }): Promise<LicencePageData> => {
	const api = apiFor(url);

	const report = await api.GET("/instance/licence", { fetch });

	if (report.error || !report.data) {
		return { view: { kind: "unavailable" } };
	}

	return { view: { kind: "ready", report: report.data } };
};
