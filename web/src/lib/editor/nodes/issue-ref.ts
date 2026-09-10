import { Node, mergeAttributes } from "@tiptap/core";

export type IssueRefAttributes = {
	issueId: string;
	reference: string;
	title: string;
	href: string;
};

/**
 * IssueRef is a link to an issue that survives what a written-out link does not: renaming the
 * issue, or moving it to another team so its reference changes. The identifier is what is
 * stored; the reference and title are what is shown, and they are refreshed on read.
 */
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
