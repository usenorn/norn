import { Node, mergeAttributes } from "@tiptap/core";

export type AttachmentAttributes = {
	attachmentId: string;
	fileName: string;
	href: string;
	contentType: string;
	byteSize: number;
};

export const Attachment = Node.create({
	name: "attachment",
	group: "block",
	atom: true,
	draggable: true,
	selectable: true,

	addAttributes() {
		return {
			attachmentId: { default: "" },
			fileName: { default: "" },
			href: { default: "" },
			contentType: { default: "" },
			byteSize: { default: 0 },
		};
	},

	parseHTML() {
		return [{ tag: "a[data-attachment]" }];
	},

	renderHTML({ HTMLAttributes, node }) {
		return [
			"a",
			mergeAttributes(HTMLAttributes, {
				"data-attachment": node.attrs.attachmentId,
				href: node.attrs.href,
				download: node.attrs.fileName,
				class:
					"inline-flex max-w-full items-center gap-1.5 rounded-md border border-line-subtle " +
					"bg-paper-2 px-2 py-1 text-sm text-ink-900 no-underline",
			}),
			node.attrs.fileName || "attachment",
		];
	},

	renderText({ node }) {
		return node.attrs.fileName || "attachment";
	},
});
