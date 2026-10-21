import type { Attachment } from "svelte/attachments";

export function rovingControls(active: boolean): Attachment<HTMLElement> {
	return (node) => {
		for (const control of node.querySelectorAll<HTMLElement>("button, a[href]")) {
			if (control.hasAttribute("data-row-anchor")) continue;

			control.tabIndex = active ? 0 : -1;
		}
	};
}
