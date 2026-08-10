import { workspacePath } from "$lib/workspace/navigation";
import type { ShortcutId } from "./shortcuts";

export type Destination = {
	id: ShortcutId;
	label: string;
	href: string;
};

export function destinations(workspace: string): Destination[] {
	const at = (path: string) => workspacePath(workspace, path);

	return [
		{ id: "go-inbox", label: "Inbox", href: at("/inbox") },
		{ id: "go-my-tasks", label: "My tasks", href: at("/my-tasks") },
		{ id: "go-reviews", label: "Reviews", href: at("/reviews") },
		{ id: "go-triage", label: "Triage", href: at("/triage") },
		{ id: "go-issues", label: "Issues", href: at("/issues") },
		{ id: "go-settings", label: "Settings", href: at("/settings") },
	];
}
