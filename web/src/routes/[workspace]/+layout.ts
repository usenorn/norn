import type { NavProject } from "$lib/workspace/navigation";
import type { LayoutLoad } from "./$types";

export type WorkspaceSummary = { slug: string; name: string };

export type WorkspaceScope = {
	workspace: WorkspaceSummary;
	workspaces: WorkspaceSummary[];
	member: { name: string };
	projects: NavProject[];
};

export const load: LayoutLoad = ({ params }): WorkspaceScope => ({
	workspace: { slug: params.workspace, name: "Northwind" },
	workspaces: [
		{ slug: params.workspace, name: "Northwind" },
		{ slug: "northwind-labs", name: "Northwind Labs" },
	],
	member: { name: "Rae Okafor" },
	projects: [
		{ name: "Mobile", slug: "mobile", color: "var(--label-blue)" },
		{ name: "Billing", slug: "billing", color: "var(--label-cyan)" },
		{ name: "Growth", slug: "growth", color: "var(--label-violet)" },
	],
});
