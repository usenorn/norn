import { roamShortcuts } from "./roam/shortcuts";

export type ShortcutGroup =
	| "global"
	| "navigation"
	| "list"
	| "issues"
	| "selection"
	| "triage"
	| "roam";

export type Shortcut = {
	id: string;
	keys: string[];
	label: string;
	group: ShortcutGroup;
	keysLabel?: string;
	whileTyping?: boolean;
	yieldsToFocus?: boolean;
	mode?: string;
};

export const shortcutGroupLabels: Record<ShortcutGroup, string> = {
	global: "Anywhere",
	navigation: "Going places",
	list: "Moving through a list",
	issues: "Issues",
	selection: "What you have selected",
	triage: "Triage",
	roam: "Moving like a game",
};

export const shortcutGroupOrder: ShortcutGroup[] = [
	"global",
	"navigation",
	"list",
	"issues",
	"selection",
	"triage",
	"roam",
];

const declared = [
	{ id: "search", keys: ["mod+k"], label: "Go anywhere", group: "global" },
	{ id: "help", keys: ["?"], label: "Show these shortcuts", group: "global" },

	{ id: "go-inbox", keys: ["g n"], label: "Inbox", group: "navigation" },
	{ id: "go-my-tasks", keys: ["g m"], label: "My tasks", group: "navigation" },
	{ id: "go-reviews", keys: ["g r"], label: "Reviews", group: "navigation" },
	{ id: "go-triage", keys: ["g t"], label: "Triage", group: "navigation" },
	{ id: "go-issues", keys: ["g i"], label: "Issues", group: "navigation" },
	{ id: "go-settings", keys: ["g s"], label: "Settings", group: "navigation" },

	{ id: "cursor-down", keys: ["j", "arrowdown"], label: "Move down", group: "list" },
	{ id: "cursor-up", keys: ["k", "arrowup"], label: "Move up", group: "list" },
	{ id: "cursor-open", keys: ["enter", "o"], label: "Open", group: "list", yieldsToFocus: true },
	{ id: "issue-new", keys: ["c"], label: "New issue", group: "issues" },
	{ id: "issue-filter", keys: ["f"], label: "Filter", group: "issues" },
	{ id: "issue-list", keys: ["shift+l"], label: "List", group: "issues" },
	{ id: "issue-board", keys: ["shift+b"], label: "Board", group: "issues" },
	{
		id: "status-set",
		keys: ["1", "2", "3", "4", "5", "6"],
		keysLabel: "1 – 6",
		label: "Set status",
		group: "issues",
	},
	{ id: "issue-edit", keys: ["e"], label: "Edit the description", group: "issues" },

	{ id: "select-toggle", keys: ["x", " "], label: "Select", group: "selection" },
	{ id: "select-clear", keys: ["escape"], label: "Clear the selection", group: "selection" },
	{ id: "bulk-status", keys: ["s"], label: "Set status", group: "selection" },
	{ id: "bulk-assignee", keys: ["a"], label: "Set assignee", group: "selection" },
	{ id: "bulk-priority", keys: ["p"], label: "Set priority", group: "selection" },
	{ id: "bulk-cycle", keys: ["shift+c"], label: "Set cycle", group: "selection" },

	{ id: "triage-accept", keys: ["a"], label: "Accept", group: "triage" },
	{ id: "triage-decline", keys: ["d"], label: "Decline", group: "triage" },
	{ id: "triage-move", keys: ["m"], label: "Move to a team", group: "triage" },
	{
		id: "triage-close",
		keys: ["escape"],
		label: "Close",
		group: "triage",
		whileTyping: true,
	},
	...roamShortcuts,
] as const satisfies readonly Shortcut[];

export type ShortcutId = (typeof declared)[number]["id"];

export const shortcuts: readonly Shortcut[] = declared;

const byId = new Map<string, Shortcut>(shortcuts.map((shortcut) => [shortcut.id, shortcut]));

export function shortcutOf(id: ShortcutId): Shortcut {
	const found = byId.get(id);

	if (!found) throw new Error(`no shortcut is declared as ${id}`);

	return found;
}

const namedKeys: Record<string, string> = {
	arrowdown: "↓",
	arrowup: "↑",
	enter: "↵",
	escape: "Esc",
	" ": "Space",
};

export function displayKeys(binding: string, apple: boolean): string {
	return binding
		.split(" ")
		.map((chord) =>
			chord
				.split("+")
				.map((part) => {
					if (part === "mod") return apple ? "⌘" : "Ctrl";
					if (part === "shift") return "⇧";

					return namedKeys[part] ?? (part.length === 1 ? part.toUpperCase() : part);
				})
				.join(" ")
		)
		.join(" ");
}

export function keycap(shortcut: Shortcut, apple: boolean): string {
	return shortcut.keysLabel ?? displayKeys(shortcut.keys[0], apple);
}

export function isApplePlatform(): boolean {
	if (typeof navigator === "undefined") return true;

	return /mac|iphone|ipad/i.test(navigator.platform || navigator.userAgent);
}

export function isTypingTarget(target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement)) return false;

	return (
		target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName)
	);
}

const activatable = [
	"button",
	"summary",
	"a[href]",
	'[role="button"]',
	'[role="menuitem"]',
	'[role="option"]',
	'[role="tab"]',
	'[role="link"]',
].join(", ");

export function isActivatableTarget(target: EventTarget | null): boolean {
	if (!(target instanceof HTMLElement)) return false;

	return Boolean(target.closest(activatable));
}
