import type { Invite } from "$lib/workspace/invites";

export type InvitePreview = {
	text?: string;
	rows?: Invite[];
	emailConfigured?: boolean;
	sending?: boolean;
};

const entered = "jun@northwind.co, milo@northwind.co\nada@northwind.co, sana@northwind.co";

export const invitePreviewStates: Record<string, InvitePreview> = import.meta.env.DEV
	? {
			empty: { text: "" },
			entered: {
				text: entered,
				rows: [
					{ email: "jun@northwind.co", role: "Member", status: "pending" },
					{ email: "milo@northwind.co", role: "Member", status: "pending" },
					{ email: "ada@northwind.co", role: "Admin", status: "pending" },
					{ email: "sana@northwind.co", role: "Member", status: "pending" },
				],
			},
			invalid: {
				text: "jun@northwind.co, milo@northwind, ada@northwind.co\nsana@northwind.co, theo@northwind.co",
				rows: [
					{ email: "jun@northwind.co", role: "Member", status: "existing_member" },
					{ email: "milo@northwind", role: "Member", status: "invalid" },
					{ email: "ada@northwind.co", role: "Admin", status: "pending" },
					{ email: "sana@northwind.co", role: "Member", status: "pending" },
					{ email: "theo@northwind.co", role: "Member", status: "pending" },
				],
			},
			sending: {
				sending: true,
				rows: [
					{ email: "jun@northwind.co", role: "Member", status: "sent" },
					{ email: "milo@northwind.co", role: "Member", status: "sent" },
					{ email: "ada@northwind.co", role: "Admin", status: "sent" },
					{ email: "sana@northwind.co", role: "Member", status: "pending" },
				],
			},
			sent: {
				rows: [
					{ email: "jun@northwind.co", role: "Member", status: "sent" },
					{ email: "milo@northwind.co", role: "Member", status: "sent" },
					{ email: "ada@northwind.co", role: "Admin", status: "sent" },
					{ email: "sana@northwind.co", role: "Member", status: "sent" },
				],
			},
			failed: {
				rows: [
					{ email: "jun@northwind.co", role: "Member", status: "sent" },
					{ email: "milo@northwind.co", role: "Member", status: "failed" },
					{ email: "ada@northwind.co", role: "Admin", status: "sent" },
					{ email: "sana@northwind.co", role: "Member", status: "failed" },
				],
			},
			nomail: {
				text: entered,
				emailConfigured: false,
				rows: [
					{ email: "jun@northwind.co", role: "Member", status: "link_only" },
					{ email: "milo@northwind.co", role: "Member", status: "link_only" },
					{ email: "ada@northwind.co", role: "Admin", status: "link_only" },
					{ email: "sana@northwind.co", role: "Member", status: "link_only" },
				],
			},
		}
	: {};
