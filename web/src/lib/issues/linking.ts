export type Changes = Record<string, string | null>;

export type LinkWith = (changes: Changes) => string;

export function linkTo(basePath: string, params: URLSearchParams, changes: Changes): string {
	const q = new URLSearchParams(params);

	for (const [key, value] of Object.entries(changes)) {
		if (value === null) q.delete(key);
		else q.set(key, value);
	}

	const query = q.toString();

	return `${basePath}${query ? `?${query}` : ""}`;
}
