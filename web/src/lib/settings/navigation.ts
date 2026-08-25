import Bell from "@lucide/svelte/icons/bell";
import Bot from "@lucide/svelte/icons/bot";
import GitBranch from "@lucide/svelte/icons/git-branch";
import Import from "@lucide/svelte/icons/import";
import KeyRound from "@lucide/svelte/icons/key-round";
import Network from "@lucide/svelte/icons/network";
import ScrollText from "@lucide/svelte/icons/scroll-text";
import Server from "@lucide/svelte/icons/server";
import Settings from "@lucide/svelte/icons/settings";
import Tags from "@lucide/svelte/icons/tags";
import Terminal from "@lucide/svelte/icons/terminal";
import Users from "@lucide/svelte/icons/users";
import UsersRound from "@lucide/svelte/icons/users-round";
import Webhook from "@lucide/svelte/icons/webhook";
import type { IconComponent } from "$lib/utils.js";
import { workspacePath } from "$lib/workspace/navigation";

export type SettingsNavigationEntry = {
	href: string;
	label: string;
	icon: IconComponent;
	exact?: boolean;
};

export type SettingsNavigationSection = {
	label: string;
	entries: SettingsNavigationEntry[];
};

export function workspaceSettingsNavigation(slug: string): SettingsNavigationSection[] {
	const settings = (path = "") => workspacePath(slug, `/settings${path}`);

	return [
		{
			label: "Workspace",
			entries: [
				{ href: settings(), label: "General", icon: Settings, exact: true },
				{ href: settings("/members"), label: "Members", icon: Users },
				{ href: settings("/teams"), label: "Teams", icon: UsersRound },
				{ href: settings("/labels"), label: "Labels", icon: Tags },
			],
		},
		{
			label: "Preferences",
			entries: [
				{ href: settings("/notifications"), label: "Notifications", icon: Bell },
			],
		},
		{
			label: "Automation",
			entries: [
				{ href: settings("/agents"), label: "Agents", icon: Bot },
				{ href: settings("/runners"), label: "Runners", icon: Server },
			],
		},
		{
			label: "Security",
			entries: [
				{ href: settings("/authentication"), label: "Authentication", icon: KeyRound },
				{ href: settings("/directory"), label: "Directory", icon: Network },
				{ href: settings("/audit"), label: "Audit log", icon: ScrollText },
			],
		},
		{
			label: "Integrations",
			entries: [
				{ href: settings("/source-control"), label: "Source control", icon: GitBranch },
				{ href: settings("/imports"), label: "Imports", icon: Import },
				{ href: settings("/webhooks"), label: "Webhooks", icon: Webhook },
			],
		},
	];
}

export const userSettingsNavigation: SettingsNavigationSection[] = [
	{
		label: "Account",
		entries: [{ href: "/settings/tokens", label: "API tokens", icon: Terminal }],
	},
];

export function settingsEntryFor(
	pathname: string,
	sections: SettingsNavigationSection[]
): SettingsNavigationEntry | undefined {
	return sections
		.flatMap((section) => section.entries)
		.filter((entry) =>
			entry.exact ? pathname === entry.href : pathname === entry.href || pathname.startsWith(`${entry.href}/`)
		)
		.sort((left, right) => right.href.length - left.href.length)[0];
}
