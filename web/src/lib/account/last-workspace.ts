export function lastWorkspaceCookie(accountId: string): string {
	return `norn.workspace.${accountId}`;
}

export function rememberedWorkspace<T extends { workspace: { slug: string } }>(
	reached: T[],
	remembered: string | undefined
): T | undefined {
	if (!remembered) return undefined;

	return reached.find((candidate) => candidate.workspace.slug === remembered);
}
