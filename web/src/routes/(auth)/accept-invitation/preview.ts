import type { AcceptInvitation } from "$lib/workspace/accept-invitation";

export const acceptInvitationPreviewStates: Record<string, AcceptInvitation> = import.meta.env.DEV
	? {
			no_token: { kind: "no_token" },
			invalid: { kind: "invalid" },
			expired: { kind: "expired" },
			revoked: { kind: "revoked" },
			already_accepted: { kind: "already_accepted" },
			create_account: {
				kind: "create_account",
				workspace: { slug: "northwind", name: "Northwind" },
				email: "rae@northwind.co",
				role: "member",
			},
			create_account_admin: {
				kind: "create_account",
				workspace: { slug: "northwind", name: "Northwind" },
				email: "rae@northwind.co",
				role: "admin",
			},
			sign_in_required: {
				kind: "sign_in_required",
				workspace: { slug: "northwind", name: "Northwind" },
				email: "rae@northwind.co",
			},
			confirm: {
				kind: "confirm",
				workspace: { slug: "northwind", name: "Northwind" },
				email: "rae@northwind.co",
				role: "member",
			},
			address_mismatch: { kind: "address_mismatch", email: "rae@northwind.co" },
			sso_required: {
				kind: "sso_required",
				workspace: { slug: "northwind", name: "Northwind" },
				email: "rae@northwind.co",
			},
			joined: { kind: "joined", workspace: { slug: "northwind", name: "Northwind" } },
			unavailable: { kind: "unavailable" },
		}
	: {};
