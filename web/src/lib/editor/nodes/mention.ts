import Mention from "@tiptap/extension-mention";

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
