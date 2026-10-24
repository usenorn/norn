import { redirect } from "@sveltejs/kit";
import { keys } from "$lib/api/keys";
import { docPath, type DocEditing } from "$lib/docs/docs";
import { readDoc } from "$lib/docs/docs.server";
import { docEditPreviewStates } from "./preview";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ depends, route, locals, params, parent, url }) => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	depends(keys.doc(params.docId));

	if (import.meta.env.DEV && docEditPreviewStates[url.searchParams.get("state") ?? ""]) {
		return { editing: { kind: "loading" } as DocEditing };
	}

	const read = await readDoc(locals.api, workspace.id, params.docId);

	if (read.kind === "ready" && (!read.doc.viewer.canEdit || read.doc.status === "archived")) {
		redirect(307, docPath(workspace.slug, read.doc.id));
	}

	return { editing: read as DocEditing };
};
