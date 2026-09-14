type Versioned = { id: string; version: number };

const seen = new Map<string, number>();

export function expectedVersion(issue: Versioned): number {
	return Math.max(issue.version, seen.get(issue.id) ?? 0);
}

export function remember(answered: unknown): void {
	const issue = versioned(answered);

	if (!issue) return;

	seen.set(issue.id, Math.max(issue.version, seen.get(issue.id) ?? 0));
}

export function forget(issueId?: string): void {
	if (issueId === undefined) {
		seen.clear();

		return;
	}

	seen.delete(issueId);
}

function versioned(value: unknown): Versioned | null {
	if (!value || typeof value !== "object") return null;

	const held = value as { id?: unknown; version?: unknown };

	if (typeof held.id !== "string" || typeof held.version !== "number") return null;

	return { id: held.id, version: held.version };
}
