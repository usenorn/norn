import type { ImportSession } from "$lib/import/types";
import type { PageLoad } from "./$types";

export const load: PageLoad = (): { session: ImportSession } => ({
	session: { source: "jira", stage: { kind: "choose_source" } },
});
