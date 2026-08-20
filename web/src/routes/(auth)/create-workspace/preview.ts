import type { CreateWorkspaceInput } from "$lib/workspace/create-workspace-schema";
import type { WorkspaceContext, WorkspaceCreationFailure } from "$lib/workspace/types";

export type CreateWorkspacePreview = {
	workspace?: Partial<WorkspaceContext>;
	form?: Partial<CreateWorkspaceInput>;
	failure?: WorkspaceCreationFailure;
	busy?: boolean;
};

export const createWorkspacePreviewStates: Record<string, CreateWorkspacePreview> = import.meta.env
	.DEV
	? {
			default: {
				form: { name: "Northwind", slug: "northwind", teamName: "Mobile", teamKey: "MOB" },
			},
			taken: {
				form: { name: "Northwind", slug: "northwind", teamName: "Mobile", teamKey: "MOB" },
				failure: {
					kind: "slug_taken",
					slug: "northwind",
					suggestions: ["northwind-co", "northwind-hq", "northwind-eng"],
				},
			},
			team_key_taken: {
				form: { name: "Northwind", slug: "northwind", teamName: "Mobile", teamKey: "MOB" },
				failure: {
					kind: "team_key_taken",
					key: "MOB",
					suggestions: ["MOBI", "MOBIL", "MBL"],
				},
			},
			signed_out: {
				form: { name: "Northwind", slug: "northwind", teamName: "Mobile", teamKey: "MOB" },
				failure: { kind: "signed_out" },
			},
			unavailable: {
				form: { name: "Northwind", slug: "northwind", teamName: "Mobile", teamKey: "MOB" },
				failure: { kind: "unavailable" },
			},
			creating: {
				form: { name: "Northwind", slug: "northwind", teamName: "Mobile", teamKey: "MOB" },
				busy: true,
			},
			additional: {
				workspace: { existingWorkspace: "Northwind" },
				form: {
					name: "Northwind Labs",
					slug: "northwind-labs",
					teamName: "Mobile",
					teamKey: "MOB",
				},
			},
		}
	: {};
