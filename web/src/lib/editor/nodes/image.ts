import Image from "@tiptap/extension-image";

export const WorkspaceImage = Image.extend({
	addAttributes() {
		return {
			...this.parent?.(),
			attachmentId: {
				default: null,
				parseHTML: (element) => element.getAttribute("data-attachment"),
				renderHTML: (attributes) =>
					attributes.attachmentId ? { "data-attachment": attributes.attachmentId } : {},
			},
		};
	},
}).configure({ inline: false });
