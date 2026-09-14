import { describe, expect, it } from "vitest";
import { documentAttachments, documentExcerpt, type Document } from "./document";

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

describe("the opening words of a document", () => {
	it("keeps paragraphs, list items and line breaks from running into each other", () => {
		const written: Document = {
			type: "doc",
			content: [
				{ type: "heading", content: [{ type: "text", text: "Refunds" }] },
				{
					type: "paragraph",
					content: [
						{ type: "text", text: "stall" },
						{ type: "hardBreak" },
						{ type: "text", text: "for" },
					],
				},
				{
					type: "bulletList",
					content: [
						{
							type: "listItem",
							content: [{ type: "paragraph", content: [{ type: "text", text: "invoices" }] }],
						},
					],
				},
			],
		};

		expect(documentExcerpt(written, 100)).toBe("Refunds stall for invoices");
	});

	it("names the people and issues it points at", () => {
		const pointing: Document = {
			type: "doc",
			content: [
				{
					type: "paragraph",
					content: [
						{ type: "mention", attrs: { id: "a", label: "Rae" } },
						{ type: "text", text: " look at " },
						{ type: "issueRef", attrs: { reference: "NORN-4" } },
					],
				},
			],
		};

		expect(documentExcerpt(pointing, 100)).toBe("@Rae look at NORN-4");
	});

	it("stops at a whole word and says it stopped", () => {
		const long: Document = {
			type: "doc",
			content: [{ type: "paragraph", content: [{ type: "text", text: "one two three four" }] }],
		};

		expect(documentExcerpt(long, 9)).toBe("one two…");
		expect(documentExcerpt(long, 7)).toBe("one two…");
		expect(documentExcerpt(long, 18)).toBe("one two three four");
	});

	it("gives nothing for an empty or missing document", () => {
		expect(documentExcerpt({ type: "doc", content: [] }, 100)).toBe("");
		expect(documentExcerpt(undefined, 100)).toBe("");
	});
});
