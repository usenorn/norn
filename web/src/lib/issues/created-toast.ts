import { copyText } from "$lib/clipboard";
import { documentExcerpt } from "$lib/editor/document";
import { showToast } from "$lib/toast/toasts";
import type { Issue } from "./issues";

const excerptLength = 140;
const lingerFor = 8000;

export function announceCreated(issue: Issue, href: string, origin: string) {
	return showToast(`Created ${issue.reference}`, {
		href,
		detail: documentExcerpt(issue.descriptionDoc, excerptLength) || undefined,
		action: "Copy link",
		duration: lingerFor,
		onaction: () => void copyText(`${origin}${href}`, `Copied a link to ${issue.reference}`),
	});
}
