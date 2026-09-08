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

export const markdownProse =
	"prose prose-sm max-w-none text-ink-900 [--tw-prose-bold:var(--ink-900)] [--tw-prose-bullets:var(--line-strong)] [--tw-prose-code:var(--ink-900)] [--tw-prose-counters:var(--text-muted)] [--tw-prose-headings:var(--ink-900)] [--tw-prose-quote-borders:var(--line-strong)] [--tw-prose-quotes:var(--ink-600)] prose-headings:font-medium prose-headings:tracking-snug prose-a:text-link prose-pre:bg-paper-2 prose-code:font-mono prose-img:my-3 prose-img:max-h-[32rem] prose-img:w-auto prose-img:rounded-md prose-img:border prose-img:border-line-subtle prose-img:bg-paper-2 prose-img:object-contain";

export function renderMarkdown(source: string): string {
	if (source.trim() === "") return "";

	return DOMPurify.sanitize(marked.parse(source) as string, {
		USE_PROFILES: { html: true },
		ADD_ATTR: ["target", "rel"],
	});
}
