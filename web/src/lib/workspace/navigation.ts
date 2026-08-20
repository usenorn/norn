import CircleDot from "@lucide/svelte/icons/circle-dot";
import Inbox from "@lucide/svelte/icons/inbox";
import List from "@lucide/svelte/icons/list";
import Menu from "@lucide/svelte/icons/menu";
import Zap from "@lucide/svelte/icons/zap";
import {
	CircleDot as CircleDotGlyph,
	Eye as EyeGlyph,
	Filter as FilterGlyph,
	Inbox as InboxGlyph,
	MailOpen as MailOpenGlyph,
	Target as TargetGlyph,
	Zap as ZapGlyph,
} from "lucide";
import type { IconNode } from "morphicons/svelte";
import type { IconComponent } from "$lib/utils.js";

export type NavEntry = {
	label: string;
	href: string;
	icon?: IconComponent;
	iconClass?: string;
	glyph?: IconNode;
	glyphEngaged?: IconNode;
	dot?: string;
	count?: number;
};

export function workspacePath(workspace: string, path = ""): string {
	return `/${workspace}${path}`;
}

export function primaryNav(workspace: string, waiting = 0, unread = 0): NavEntry[] {
	const at = (path: string) => workspacePath(workspace, path);
	return [
		{
			label: "Inbox",
			href: at("/inbox"),
			glyph: InboxGlyph,
			glyphEngaged: MailOpenGlyph,
			count: unread || undefined,
		},
		{
			label: "My tasks",
			href: at("/my-tasks"),
			glyph: CircleDotGlyph,
			glyphEngaged: TargetGlyph,
		},
		{
			label: "Reviews",
			href: at("/reviews"),
			glyph: CircleDotGlyph,
			glyphEngaged: EyeGlyph,
			iconClass: "text-status-active",
		},
		{
			label: "Triage",
			href: at("/triage"),
			glyph: ZapGlyph,
			glyphEngaged: FilterGlyph,
			count: waiting || undefined,
		},
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

	if (!query) return pathname === path || pathname.startsWith(`${path}/`);
	if (pathname !== path) return false;

	const wanted = new URLSearchParams(query);
	for (const [key, value] of wanted) {
		if (search.get(key) !== value) return false;
	}

	return true;
}

export function isExactly(pathname: string, href: string): boolean {
	return pathname === href.split("?")[0];
}
