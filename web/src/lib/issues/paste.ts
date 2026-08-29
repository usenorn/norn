import TurndownService from "turndown";
import { gfm } from "turndown-plugin-gfm";

let service: TurndownService | null = null;

function converter(): TurndownService {
	if (service) return service;

	service = new TurndownService({
		headingStyle: "atx",
		hr: "---",
		bulletListMarker: "-",
		codeBlockStyle: "fenced",
		emDelimiter: "_",
	});

	service.use(gfm);

	return service;
}

export function pastedMarkdown(event: ClipboardEvent): string | null {
	const clipboard = event.clipboardData;

	if (!clipboard || clipboard.files.length > 0) return null;

	const html = clipboard.getData("text/html");

	if (html.trim() === "") return null;

	const markdown = converter().turndown(html).trim();
	const plain = clipboard.getData("text/plain").trim();

	return markdown === "" || markdown === plain ? null : markdown;
}

export function withPasted(field: HTMLTextAreaElement, markdown: string): string {
	const start = field.selectionStart;
	const end = field.selectionEnd;

	return field.value.slice(0, start) + markdown + field.value.slice(end);
}

export function caretAfterPaste(field: HTMLTextAreaElement, markdown: string): number {
	return field.selectionStart + markdown.length;
}
