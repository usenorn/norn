import DOMPurify from "isomorphic-dompurify";
import { Marked } from "marked";

const marked = new Marked({ gfm: true, breaks: true, async: false });

DOMPurify.addHook("uponSanitizeAttribute", (_node, event) => {
	if (event.attrName !== "src" && event.attrName !== "href") return;

	if (event.attrValue.trim().toLowerCase().startsWith("data:")) {
		event.keepAttr = false;
	}
});

DOMPurify.addHook("afterSanitizeAttributes", (node) => {
	if (node.nodeName !== "IMG") return;

	node.setAttribute("loading", "lazy");
	node.setAttribute("decoding", "async");
	node.setAttribute("referrerpolicy", "no-referrer");
});

export function renderMarkdown(source: string): string {
	if (source.trim() === "") return "";

	return DOMPurify.sanitize(marked.parse(source) as string, {
		USE_PROFILES: { html: true },
		ADD_ATTR: ["target", "rel"],
	});
}
