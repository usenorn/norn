import type { SCMIdentity } from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SCMIdentityView =
	| { kind: "loading" }
	| { kind: "ready"; identities: SCMIdentity[] }
	| { kind: "forbidden" }
	| { kind: "unavailable" };

export type SCMIdentityPageData = {
	view: SCMIdentityView;
	members: { id: string; name: string }[];
};

export const load: PageServerLoad = async ({
	depends,
	route,
	locals,
	parent,
}): Promise<SCMIdentityPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const [identities, members] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/identities", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/members", {
			params: { path: { workspaceId: workspace.id } },
		}),
	]);

	// Only a person can be somebody on a forge. An integration account is one Norn created
	// for a connection, and mapping it would let the integration assign work to itself.
	const roster = (members.data?.members ?? [])
		.filter((member) => member.kind !== "integration")
		.map((member) => ({
			id: member.accountId,
			name: member.displayName || member.email || member.accountId,
		}));

	if (identities.error) {
		if (identities.response.status === 403) {
			return { view: { kind: "forbidden" }, members: roster };
		}

		return { view: { kind: "unavailable" }, members: roster };
	}

	return { view: { kind: "ready", identities: identities.data ?? [] }, members: roster };
};
