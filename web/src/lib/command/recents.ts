import { readStored, store } from "$lib/storage";
import type { Destination, RecentEntry } from "./model";

export const recentLimit = 5;

const storageKey = (workspaceId: string) => `norn.palette.recent.${workspaceId}`;

export function readRecents(workspaceId: string): RecentEntry[] {
	let stored: unknown;

	try {
		stored = JSON.parse(readStored(storageKey(workspaceId)) ?? "[]");
	} catch {
		return [];
	}

	return Array.isArray(stored) ? stored.filter(isRecent).slice(0, recentLimit) : [];
}

export function rememberRecent(workspaceId: string, entry: RecentEntry): RecentEntry[] {
	const next = [
		{ id: entry.id, href: entry.href },
		...readRecents(workspaceId).filter((recent) => recent.id !== entry.id),
	].slice(0, recentLimit);

	store(storageKey(workspaceId), JSON.stringify(next));

	return next;
}

export function referenceOf(href: string): string | null {
	return /\/issues\/([A-Za-z][A-Za-z0-9]*-\d+)(?:[?#]|$)/.exec(href)?.[1] ?? null;
}

export function resolveRecents(
	recents: RecentEntry[],
	known: Destination[],
	issues: Map<string, Destination>
): Destination[] {
	const byId = new Map(known.map((destination) => [destination.id, destination]));

	return recents.flatMap((recent) => {
		const found = byId.get(recent.id) ?? issues.get(recent.id);

		return found ? [found] : [];
	});
}

function isRecent(value: unknown): value is RecentEntry {
	return (
		typeof value === "object" &&
		value !== null &&
		"id" in value &&
		"href" in value &&
		typeof value.id === "string" &&
		typeof value.href === "string" &&
		isLocalPath(value.href)
	);
}

function isLocalPath(href: string): boolean {
	return href.startsWith("/") && !href.startsWith("//") && !href.startsWith("/\\");
}
