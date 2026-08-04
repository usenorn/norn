import type { Invite } from "$lib/workspace/invites";

export type InvitePreview = {
	text?: string;
	rows?: Invite[];
	emailConfigured?: boolean;
	sending?: boolean;
};

export const invitePreviewStates: Record<string, InvitePreview> = import.meta.env.DEV
	? {
			empty: { text: "" },
			entered: {
				text: "jun@northwind.co, milo@northwind.co\nada@northwind.co, sana@northwind.co",
				rows: [
					{ email: "jun@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
					{ email: "milo@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
					{ email: "ada@northwind.co", role: "admin", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
					{ email: "sana@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
				],
			},
			invalid: {
				text: "jun@northwind.co, milo-northwind, ada@northwind.co\nsana@northwind.co, theo@northwind.co",
				rows: [
					{ email: "jun@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "existing_member" },
					{ email: "milo-northwind", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "invalid" },
					{ email: "ada@northwind.co", role: "admin", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
					{ email: "sana@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
					{ email: "theo@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
				],
			},
			sending: {
				sending: true,
				rows: [
					{ email: "jun@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "milo@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "ada@northwind.co", role: "admin", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "sana@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "pending" },
				],
			},
			sent: {
				rows: [
					{ email: "jun@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "milo@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "ada@northwind.co", role: "admin", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "sana@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
				],
			},
			failed: {
				rows: [
					{ email: "jun@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "milo@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "failed" },
					{ email: "ada@northwind.co", role: "admin", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "sent" },
					{ email: "sana@northwind.co", role: "member", teamIds: ["00000000-0000-4000-8000-000000000101"], status: "failed" },
				],
			},
			nomail: {
				text: "jun@northwind.co, milo@northwind.co\nada@northwind.co, sana@northwind.co",
				emailConfigured: false,
				rows: [
					{
						email: "jun@northwind.co",
						role: "member",
						teamIds: ["00000000-0000-4000-8000-000000000101"],
						status: "link_only",
						url: "https://norn.northwind.internal/accept-invitation?token=preview-jun",
					},
					{
						email: "milo@northwind.co",
						role: "member",
						teamIds: ["00000000-0000-4000-8000-000000000101"],
						status: "link_only",
						url: "https://norn.northwind.internal/accept-invitation?token=preview-milo",
					},
					{
						email: "ada@northwind.co",
						role: "admin",
						teamIds: ["00000000-0000-4000-8000-000000000101"],
						status: "link_only",
						url: "https://norn.northwind.internal/accept-invitation?token=preview-ada",
					},
					{
						email: "sana@northwind.co",
						role: "member",
						teamIds: ["00000000-0000-4000-8000-000000000101"],
						status: "link_only",
						url: "https://norn.northwind.internal/accept-invitation?token=preview-sana",
					},
				],
			},
		}
	: {};
