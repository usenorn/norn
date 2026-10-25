import type { ReadonlyURL } from "$app/state";

const listPath = /^\/(issues|my-tasks|teams\/[^/]+\/issues)$/;

export function withReturn(href: string, from: string): string {
	const [base, query] = href.split("?");
	const params = new URLSearchParams(query);

	params.set("from", from);

	return `${base}?${params}`;
}

export function listReturn(url: ReadonlyURL, workspace: string, fallback: string): string {
	const asked = url.searchParams.get("from");

	if (!asked) return fallback;

	let target: URL;

	try {
		target = new URL(asked, url.href);
	} catch {
		return fallback;
	}

	if (target.origin !== url.origin) return fallback;

	const prefix = `/${workspace}`;

	if (!target.pathname.startsWith(`${prefix}/`)) return fallback;
	if (!listPath.test(target.pathname.slice(prefix.length))) return fallback;

	return `${target.pathname}${target.search}`;
}
