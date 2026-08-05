import type { ConsentState } from "$lib/account/connections";

export type AuthorizePreview = {
	consent?: ConsentState;
	busy?: "approve" | "deny";
};

export const authorizePreviewStates: Record<string, AuthorizePreview> = import.meta.env.DEV
	? {
			loading: { consent: { kind: "loading" } },
			ready: {
				consent: {
					kind: "ready",
					request: {
						clientName: "Claude",
						capability: "write",
						workspaces: [
							{ id: "00000000-0000-4000-8000-0000000009a1", slug: "acme", name: "Acme" },
							{
								id: "00000000-0000-4000-8000-0000000009a2",
								slug: "northwind",
								name: "Northwind",
							},
						],
					},
				},
			},
			"read-only": {
				consent: {
					kind: "ready",
					request: {
						clientName: "Issue digest bot",
						capability: "read",
						workspaces: [
							{ id: "00000000-0000-4000-8000-0000000009a1", slug: "acme", name: "Acme" },
						],
					},
				},
			},
			approving: {
				consent: {
					kind: "ready",
					request: {
						clientName: "Claude",
						capability: "write",
						workspaces: [
							{ id: "00000000-0000-4000-8000-0000000009a1", slug: "acme", name: "Acme" },
						],
					},
				},
				busy: "approve",
			},
			expired: { consent: { kind: "expired" } },
			failed: { consent: { kind: "failed" } },
		}
	: {};
