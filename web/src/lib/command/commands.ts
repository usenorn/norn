import CircleDot from "@lucide/svelte/icons/circle-dot";
import Layers from "@lucide/svelte/icons/layers";
import Link from "@lucide/svelte/icons/link";
import Plus from "@lucide/svelte/icons/plus";
import Table from "@lucide/svelte/icons/table";
import Tag from "@lucide/svelte/icons/tag";
import User from "@lucide/svelte/icons/user";
import Zap from "@lucide/svelte/icons/zap";
import { displayKeys, shortcutOf, type ShortcutId } from "$lib/shortcuts/shortcuts";
import { scopeSubject, sharedTeam, type CommandScope, type PaletteCommand } from "./model";

export function paletteCommands(
	scope: CommandScope,
	apple: boolean,
	cycling: ReadonlySet<string>
): PaletteCommand[] {
	const keys = (id: ShortcutId) => displayKeys(shortcutOf(id).keys[0], apple);
	const scoped = scope.kind === "issues";
	const subject = scopeSubject(scope);
	const team = sharedTeam(scope);
	const oneTeam = team !== null;

	const commands: (PaletteCommand | false)[] = [
		{ id: "issue-new", label: "New issue", icon: Plus, keys: keys("issue-new") },
		scoped && { id: "assign", label: `Assign ${subject} to…`, icon: User, keys: keys("bulk-assignee"), step: "assign" },
		oneTeam && { id: "status", label: `Change status of ${subject}…`, icon: CircleDot, keys: keys("bulk-status"), step: "status" },
		oneTeam && cycling.has(team) && { id: "cycle", label: `Move ${subject} to cycle…`, icon: Layers, keys: keys("bulk-cycle"), step: "cycle" },
		scoped && { id: "label", label: `Add label to ${subject}…`, icon: Tag, step: "label" },
		scoped && { id: "copy-link", label: "Copy issue link", icon: Link },
		{ id: "open-triage", label: "Open triage queue", icon: Zap, keys: keys("go-triage") },
		{ id: "toggle-density", label: "Toggle compact density", icon: Table },
	];

	return commands.filter((command) => command !== false);
}

export function stepLabel(command: PaletteCommand): string {
	return command.label.replace(/…$/, "");
}
