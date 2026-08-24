import {
	appNoticeFrom,
	type SourceControlAppNotice,
	type SourceControlAppState,
	type SourceControlView,
} from "$lib/source-control/source-control";
import { keys } from "$lib/api/keys";
import type { PageServerLoad } from "./$types";

export type SourceControlPageData = {
	view: SourceControlView;
	application: SourceControlAppState;
	notice?: SourceControlAppNotice;
	teams: { id: string; key: string; name: string }[];
};

export const load: PageServerLoad = async ({
	depends,
	route,
	url,
	locals,
	parent,
}): Promise<SourceControlPageData> => {
	depends(keys.page(route.id));

	const { workspace } = await parent();

	const handle = url.searchParams.get("installations") ?? "";

	const [listing, repositories, teams, application, installations] = await Promise.all([
		locals.api.GET("/workspaces/{workspaceId}/source-control/connections", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/source-control/repositories", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/teams", {
			params: { path: { workspaceId: workspace.id } },
		}),
		locals.api.GET("/workspaces/{workspaceId}/source-control/application", {
			params: { path: { workspaceId: workspace.id } },
		}),
		handle
			? locals.api.GET("/workspaces/{workspaceId}/source-control/application/installations", {
					params: { path: { workspaceId: workspace.id }, query: { handle } },
				})
			: Promise.resolve(undefined),
	]);

	const reachable = (teams.data ?? []).map((team) => ({
		id: team.id,
		key: team.key,
		name: team.name,
	}));

	const held = application.data;

	let app: SourceControlAppState = { kind: "unsupported" };

	if (held?.registered) {
		app = { kind: "registered", slug: held.slug ?? "", installUrl: held.installUrl ?? "" };
	} else if (held) {
		app = { kind: "unregistered", canRegister: held.canRegister };
	}

	// An answered handle with nothing in it is its own state: the person is signed in and the
	// application reaches no account they administer. Folding it back into "registered" sent
	// them round the same sign-in with no word on why it came back empty.
	if (installations?.data) {
		app = {
			kind: "choosing",
			handle,
			installUrl: held?.installUrl ?? "",
			installations: installations.data,
		};
	}

	const notice: SourceControlAppNotice | undefined = url.searchParams.has("registered")
		? { kind: "registered" }
		: appNoticeFrom(url.searchParams.get("failed"));

	const common = { application: app, notice, teams: reachable };

	if (listing.error) {
		if (listing.response.status === 403) return { view: { kind: "forbidden" }, ...common };
		if (listing.response.status === 503) {
			return { view: { kind: "sealing_unavailable" }, ...common };
		}

		return { view: { kind: "unavailable" }, ...common };
	}

	if (!listing.data || listing.data.length === 0) {
		return { view: { kind: "empty" }, ...common };
	}

	// A repository listing that failed is not the same fact as a workspace that has connected
	// none, and reporting them identically is what let "nothing is connected" pass for normal.
	return {
		view: {
			kind: "list",
			connections: listing.data,
			repositories: repositories.data ?? [],
			repositoriesUnavailable: Boolean(repositories.error),
		},
		...common,
	};
};
