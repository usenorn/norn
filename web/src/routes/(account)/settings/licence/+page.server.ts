import type { LicenceView } from "$lib/licence/licence";
import type { PageServerLoad } from "./$types";

export type LicencePageData = {
	view: LicenceView;
};

export const load: PageServerLoad = async ({ locals }): Promise<LicencePageData> => {
	const report = await locals.api.GET("/instance/licence");

	if (report.error || !report.data) {
		return { view: { kind: "unavailable" } };
	}

	return { view: { kind: "ready", report: report.data } };
};
