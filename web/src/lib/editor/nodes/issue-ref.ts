import { Node, mergeAttributes } from "@tiptap/core";

export type IssueRefAttributes = {
	issueId: string;
	reference: string;
	title: string;
	href: string;
};

export const IssueRef = Node.create({
	name: "issueRef",
	group: "inline",
	inline: true,
	atom: true,
	selectable: true,

	addAttributes() {
		return {
			issueId: { default: "" },
			reference: { default: "" },
			title: { default: "" },
			href: { default: "" },
		};
	},

	parseHTML() {
		return [{ tag: 'a[data-issue-ref]' }];
	},

	renderHTML({ HTMLAttributes, node }) {
		return [
			"a",
			mergeAttributes(HTMLAttributes, {
				"data-issue-ref": node.attrs.issueId,
				href: node.attrs.href,
				class: "font-medium text-ink-900 underline decoration-line-strong underline-offset-2",
			}),
			node.attrs.reference,
		];
	},

	renderText({ node }) {
		return node.attrs.reference;
	},
});
