import type { Attachment } from "svelte/attachments";

export function rovingControls(active: boolean): Attachment<HTMLElement> {
	return (node) => {
		const apply = () => {
			for (const control of node.querySelectorAll<HTMLElement>("button, a[href]")) {
				if (control.hasAttribute("data-row-anchor")) continue;

				control.tabIndex = active ? 0 : -1;
			}
		};

		apply();

		const observer = new MutationObserver(apply);

		observer.observe(node, { childList: true, subtree: true });

		return () => observer.disconnect();
	};
}
