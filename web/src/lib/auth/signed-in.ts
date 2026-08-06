import { redirect } from "@sveltejs/kit";

export async function leaveIfSignedIn(api: App.Locals["api"]): Promise<void> {
	const { data, error } = await api.GET("/workspaces");

	if (!error && data) redirect(307, "/");
}
