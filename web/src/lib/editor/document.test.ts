import { describe, expect, it } from "vitest";
import { documentAttachments, type Document } from "./document";

const nested: Document = {
	type: "doc",
	content: [
		{ type: "paragraph", content: [{ type: "text", text: "before" }] },
		{
			type: "bulletList",
			content: [
				{
					type: "listItem",
					content: [
						{
							type: "paragraph",
							content: [{ type: "image", attrs: { src: "/a", alt: "a", attachmentId: "one" } }],
						},
					],
				},
			],
		},
		{
			type: "blockquote",
			content: [
				{
					type: "paragraph",
					content: [{ type: "attachment", attrs: { attachmentId: "two", fileName: "b.pdf" } }],
				},
			],
		},
	],
};

describe("finding the attachments a description already carries", () => {
	it("sees a picture that sits inside a list or a quote, not only at the top", () => {
		expect(documentAttachments(nested).sort()).toEqual(["one", "two"]);
	});

	it("names each attachment once, however many times it appears", () => {
		const twice: Document = {
			type: "doc",
			content: [
				{ type: "image", attrs: { src: "/a", attachmentId: "one" } },
				{ type: "image", attrs: { src: "/a", attachmentId: "one" } },
			],
		};

		expect(documentAttachments(twice)).toEqual(["one"]);
	});

	it("ignores a picture that carries no attachment, which is every older document", () => {
		const older: Document = {
			type: "doc",
			content: [
				{ type: "image", attrs: { src: "/a", alt: "a" } },
				{ type: "image", attrs: { src: "/b", attachmentId: "" } },
				{ type: "paragraph", content: [{ type: "text", text: "words" }] },
			],
		};

		expect(documentAttachments(older)).toEqual([]);
		expect(documentAttachments(undefined)).toEqual([]);
	});
});
