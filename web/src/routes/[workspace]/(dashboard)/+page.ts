import { redirect } from "@sveltejs/kit";
import { workspacePath } from "#lib/workspace/navigation.js";
import type { PageLoad } from "./$types";

export const load: PageLoad = ({ params }) => {
	redirect(307, workspacePath(params.workspace, "/my-tasks"));
};
