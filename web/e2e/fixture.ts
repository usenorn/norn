import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

export type Fixture = {
	workspaceId: string;
	slug: string;
	teamKey: string;
	accountId: string;
	displayName: string;
	email: string;
	issues: { id: string; reference: string; title: string }[];
};

export const statePath = fileURLToPath(new URL("./.state/session.json", import.meta.url));
export const fixturePath = fileURLToPath(new URL("./.state/fixture.json", import.meta.url));

export function fixture(): Fixture {
	return JSON.parse(readFileSync(fixturePath, "utf8")) as Fixture;
}

export function at(path: string): string {
	return `/${fixture().slug}${path}`;
}
