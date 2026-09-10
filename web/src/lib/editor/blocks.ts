import type { Editor } from "@tiptap/core";

export type BlockRange = { from: number; to: number };

/**
 * Running a slash command replaces the `/query` the writer typed and nothing else. Leaving it
 * behind, or clearing the whole block, are the two ways this goes wrong and both lose text.
 */
export function runBlock(editor: Editor, range: BlockRange, key: string): boolean {
	const chain = editor.chain().focus().deleteRange(range);

	switch (key) {
		case "h1":
			return chain.setNode("heading", { level: 1 }).run();
		case "h2":
			return chain.setNode("heading", { level: 2 }).run();
		case "h3":
			return chain.setNode("heading", { level: 3 }).run();
		case "text":
			return chain.setParagraph().run();
		case "bullet":
			return chain.toggleBulletList().run();
		case "numbered":
			return chain.toggleOrderedList().run();
		case "todo":
			return chain.toggleTaskList().run();
		case "code":
			return chain.setCodeBlock().run();
		case "quote":
			return chain.setBlockquote().run();
		case "divider":
			return chain.setHorizontalRule().run();
		case "table":
			return chain.insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
		case "toggle":
			return chain.setDetails().run();
		default:
			return chain.run();
	}
}

export function insertMention(
	editor: Editor,
	range: BlockRange,
	attrs: { id: string; label: string; kind: string }
): boolean {
	return editor
		.chain()
		.focus()
		.insertContentAt(range, [
			{ type: "mention", attrs },
			{ type: "text", text: " " },
		])
		.run();
}

export function insertIssueRef(
	editor: Editor,
	range: BlockRange,
	attrs: { issueId: string; reference: string; title: string; href: string }
): boolean {
	return editor
		.chain()
		.focus()
		.insertContentAt(range, [
			{ type: "issueRef", attrs },
			{ type: "text", text: " " },
		])
		.run();
}

export function insertLink(
	editor: Editor,
	range: BlockRange,
	label: string,
	href: string
): boolean {
	return editor
		.chain()
		.focus()
		.insertContentAt(range, [
			{ type: "text", text: label, marks: [{ type: "link", attrs: { href } }] },
			{ type: "text", text: " " },
		])
		.run();
}
