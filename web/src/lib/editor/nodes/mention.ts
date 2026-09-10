import Mention from "@tiptap/extension-mention";

export const WorkspaceMention = Mention.extend({
	addAttributes() {
		return {
			...this.parent?.(),
			kind: {
				default: "account",
				parseHTML: (element) => element.getAttribute("data-kind") ?? "account",
				renderHTML: (attributes) => ({ "data-kind": attributes.kind }),
			},
		};
	},

	addProseMirrorPlugins() {
		return [];
	},

	renderText({ node }) {
		return `@${node.attrs.label}`;
	},
}).configure({
	HTMLAttributes: {
		class:
			"rounded-sm bg-accent px-1 py-0.5 font-medium text-ink-900",
	},
});
