import DOMPurify from "isomorphic-dompurify";
import { Marked } from "marked";

const marked = new Marked({ gfm: true, breaks: true, async: false });

export function renderMarkdown(source: string): string {
	if (source.trim() === "") return "";

	return DOMPurify.sanitize(marked.parse(source) as string, {
		USE_PROFILES: { html: true },
		ADD_ATTR: ["target", "rel"],
	});
}
