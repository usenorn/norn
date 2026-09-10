import type { components } from "$lib/api/dashboard.gen";

export type Document = components["schemas"]["Document"];
export type DocumentNode = components["schemas"]["DocumentNode"];
export type DocumentMark = components["schemas"]["DocumentMark"];
export type NodeKind = DocumentNode["type"];
export type MarkKind = DocumentMark["type"];

const nodeKinds = new Set<string>([
	"paragraph",
	"heading",
	"bulletList",
	"orderedList",
	"listItem",
	"taskList",
	"taskItem",
	"blockquote",
	"horizontalRule",
	"codeBlock",
	"table",
	"tableRow",
	"tableHeader",
	"tableCell",
	"details",
	"detailsSummary",
	"detailsContent",
	"image",
	"attachment",
	"mention",
	"issueRef",
	"hardBreak",
	"text",
]);

const markKinds = new Set<string>(["bold", "italic", "strike", "code", "link"]);

export const emptyDocument: Document = { type: "doc", content: [] };

export function asDocument(value: unknown): Document {
	if (!value || typeof value !== "object") return emptyDocument;

	const held = value as { type?: unknown; content?: unknown };

	return { type: "doc", content: asNodes(held.content) };
}

function asNodes(value: unknown): DocumentNode[] {
	if (!Array.isArray(value)) return [];

	const kept: DocumentNode[] = [];

	for (const held of value) {
		const node = asNode(held);

		if (node) kept.push(node);
	}

	return kept;
}

function asNode(value: unknown): DocumentNode | null {
	if (!value || typeof value !== "object") return null;

	const held = value as Record<string, unknown>;
	const kind = held.type;

	if (typeof kind !== "string" || !nodeKinds.has(kind)) return null;

	const node = { type: kind as NodeKind } as DocumentNode;

	if (held.attrs && typeof held.attrs === "object") {
		node.attrs = held.attrs as Record<string, unknown>;
	}

	if (typeof held.text === "string" && held.text !== "") node.text = held.text;

	const content = asNodes(held.content);

	if (content.length > 0) node.content = content;

	const marks = asMarks(held.marks);

	if (marks.length > 0) node.marks = marks;

	return node;
}

function asMarks(value: unknown): DocumentMark[] {
	if (!Array.isArray(value)) return [];

	const kept: DocumentMark[] = [];

	for (const held of value) {
		if (!held || typeof held !== "object") continue;

		const mark = held as Record<string, unknown>;
		const kind = mark.type;

		if (typeof kind !== "string" || !markKinds.has(kind)) continue;

		const converted = { type: kind as MarkKind } as DocumentMark;

		if (mark.attrs && typeof mark.attrs === "object") {
			converted.attrs = mark.attrs as Record<string, unknown>;
		}

		kept.push(converted);
	}

	return kept;
}

export function documentText(document: Document | undefined): string {
	if (!document) return "";

	let written = "";

	walk(document.content ?? [], (node) => {
		if (node.type === "text") written += node.text ?? "";
		if (node.type === "mention") written += `@${attrText(node, "label")}`;
		if (node.type === "issueRef") written += attrText(node, "reference");
	});

	return written;
}

export function documentEmpty(document: Document | undefined): boolean {
	if (!document || (document.content ?? []).length === 0) return true;

	let carries = false;

	walk(document.content ?? [], (node) => {
		if (node.type === "text" && (node.text ?? "").trim() !== "") carries = true;
		if (["image", "attachment", "mention", "issueRef", "horizontalRule", "table"].includes(node.type)) {
			carries = true;
		}
	});

	return !carries;
}

export function documentMentions(document: Document | undefined): string[] {
	const found = new Set<string>();

	walk(document?.content ?? [], (node) => {
		if (node.type !== "mention") return;

		const id = attrText(node, "id");

		if (id !== "") found.add(id);
	});

	return [...found];
}

export function documentAttachments(document: Document | undefined): string[] {
	const found = new Set<string>();

	walk(document?.content ?? [], (node) => {
		if (node.type !== "attachment" && node.type !== "image") return;

		const id = attrText(node, "attachmentId");

		if (id !== "") found.add(id);
	});

	return [...found];
}

export function sameDocument(one: Document | undefined, other: Document | undefined): boolean {
	return JSON.stringify(one ?? emptyDocument) === JSON.stringify(other ?? emptyDocument);
}

function attrText(node: DocumentNode, name: string): string {
	const value = (node.attrs as Record<string, unknown> | undefined)?.[name];

	return typeof value === "string" ? value : "";
}

function walk(nodes: DocumentNode[], visit: (node: DocumentNode) => void) {
	for (const node of nodes) {
		visit(node);
		walk(node.content ?? [], visit);
	}
}
