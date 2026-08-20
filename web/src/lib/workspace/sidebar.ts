export function expansionCookie(accountId: string, workspaceId: string): string {
	return `norn.sidebar.${accountId}.${workspaceId}`;
}

export function readExpanded(stored: string | undefined, teamKeys: string[]): string[] {
	if (stored === undefined) return teamKeys;

	const remembered = new Set(stored.split(",").filter(Boolean));

	return teamKeys.filter((key) => remembered.has(key));
}

export function writeExpanded(keys: string[]): string {
	return keys.join(",");
}

export function toggledExpansion(expanded: string[], key: string): string[] {
	return expanded.includes(key)
		? expanded.filter((each) => each !== key)
		: [...expanded, key];
}
