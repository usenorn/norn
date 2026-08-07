import type { SourceControlFailure } from "$lib/source-control/source-control";
import type { SCMIdentityView } from "./+page.server";

export type SCMIdentityPreview = {
	view?: SCMIdentityView;
	failure?: SourceControlFailure;
};

export const scmIdentityPreviewStates: Record<string, SCMIdentityPreview> = import.meta.env.DEV
	? {
			loading: { view: { kind: "loading" } },
			empty: { view: { kind: "ready", identities: [] } },
			ready: {
				view: {
					kind: "ready",
					identities: [
						{
							id: "00000000-0000-4000-8000-0000000000f1",
							accountId: "00000000-0000-4000-8000-0000000000e1",
							provider: "github",
							login: "raechen",
						},
						{
							id: "00000000-0000-4000-8000-0000000000f2",
							accountId: "00000000-0000-4000-8000-0000000000e2",
							provider: "gitlab",
							login: "sam.iyer",
						},
					],
				},
			},
			forbidden: { view: { kind: "forbidden" } },
			unavailable: { view: { kind: "unavailable" } },
			identity_mapped: {
				view: { kind: "ready", identities: [] },
				failure: { kind: "identity_mapped" },
			},
		}
	: {};
