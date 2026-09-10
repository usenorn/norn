import Mention from "@tiptap/extension-mention";

/**
 * A mention carries who it names rather than only what it reads. The label is what a reader
 * sees and the kind and identifier are what a notification follows, so renaming somebody
 * changes the text without breaking who it reaches.
 */
export const WorkspaceMention = Mention.extend({
	addAttributes() {
		return {
			id: { default: "" },
			label: { default: "" },
			kind: { default: "account" },
		};
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
