import { describe, expect, it } from "vitest";
import { managesTeams, roleOf, type Membership } from "./members";

function member(accountId: string, role: Membership["role"]): Membership {
	return {
		workspaceId: "00000000-0000-4000-8000-000000000000",
		accountId,
		role,
		source: "manual",
	} as Membership;
}

const roster = [member("admin", "admin"), member("ordinary", "member"), member("guest", "viewer")];

describe("who may change a team", () => {
	it("lets an admin, because the server lets nobody else", () => {
		expect(managesTeams(roster, "admin")).toBe(true);
	});

	it("refuses an ordinary member, so the page does not offer what the server refuses", () => {
		expect(managesTeams(roster, "ordinary")).toBe(false);
		expect(managesTeams(roster, "guest")).toBe(false);
	});

	it("refuses somebody the workspace does not know", () => {
		expect(managesTeams(roster, "stranger")).toBe(false);
		expect(roleOf(roster, "stranger")).toBeNull();
	});

	it("reads the role back for the person asked about", () => {
		expect(roleOf(roster, "ordinary")).toBe("member");
	});
});
