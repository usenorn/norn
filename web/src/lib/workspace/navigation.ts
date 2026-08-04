import Clock from "@lucide/svelte/icons/clock";
import CircleDot from "@lucide/svelte/icons/circle-dot";
import Inbox from "@lucide/svelte/icons/inbox";
import List from "@lucide/svelte/icons/list";
import Menu from "@lucide/svelte/icons/menu";
import Target from "@lucide/svelte/icons/target";
import Zap from "@lucide/svelte/icons/zap";
import type { IconComponent } from "$lib/utils.js";

export type NavEntry = {
	label: string;
	href: string;
	icon?: IconComponent;
	iconClass?: string;
	dot?: string;
	count?: number;
};

export function workspacePath(workspace: string, path = ""): string {
	return `/${workspace}${path}`;
}

export function primaryNav(workspace: string): NavEntry[] {
	const at = (path: string) => workspacePath(workspace, path);
	return [
		{ label: "Inbox", href: at("/inbox"), icon: Inbox, count: 4 },
		{ label: "My tasks", href: at("/my-tasks"), icon: CircleDot, count: 10 },
		{
			label: "Reviews",
			href: at("/reviews"),
			icon: CircleDot,
			iconClass: "text-status-active",
			count: 3,
		},
		{ label: "Triage", href: at("/triage"), icon: Zap, count: 12 },
		{ label: "Issues", href: at("/issues"), icon: List },
	];
}

export function savedViews(workspace: string): NavEntry[] {
	const at = (path: string) => workspacePath(workspace, path);
	return [
		{ label: "Urgent & unassigned", href: at("/issues?view=urgent-unassigned"), icon: Target },
		{ label: "Due this week", href: at("/issues?view=due-this-week"), icon: Clock },
	];
}

export function mobileNav(workspace: string): NavEntry[] {
	const at = (path: string) => workspacePath(workspace, path);
	return [
		{ label: "Inbox", href: at("/inbox"), icon: Inbox },
		{ label: "My tasks", href: at("/my-tasks"), icon: CircleDot },
		{ label: "Issues", href: at("/issues"), icon: List },
		{ label: "Menu", href: at("/menu"), icon: Menu },
	];
}

export function isCurrent(pathname: string, search: URLSearchParams, href: string): boolean {
	const [path, query] = href.split("?");
	if (pathname !== path) return false;
	if (!query) return search.size === 0;
	const wanted = new URLSearchParams(query);
	for (const [key, value] of wanted) {
		if (search.get(key) !== value) return false;
	}
	return true;
}
