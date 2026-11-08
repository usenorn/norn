import { describe, expect, it } from "vitest";
import {
	kindsFor,
	managesTeams,
	personDetail,
	roleOf,
	segmentFrom,
	type Membership,
} from "./members";

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

describe("which segment the page is on", () => {
	it("falls back to people when the parameter is absent or unknown", () => {
		expect(segmentFrom(null)).toBe("people");
		expect(segmentFrom("")).toBe("people");
		expect(segmentFrom("robots")).toBe("people");
	});

	it("reads the agents segment from the parameter the tabs write", () => {
		expect(segmentFrom("agents")).toBe("agents");
	});

	it("asks the server for both machine kinds under agents, and people alone otherwise", () => {
		expect(kindsFor("agents")).toEqual(["agent", "integration"]);
		expect(kindsFor("people")).toEqual(["person"]);
	});
});

describe("what somebody's details say about them", () => {
	it("names a person with their role and email", () => {
		expect(
			personDetail({ ...member("rae", "admin"), displayName: "Rae Okafor", email: "rae@northwind.co" })
		).toEqual({ accountId: "rae", name: "Rae Okafor", standing: "Admin", email: "rae@northwind.co" });
	});

	it("calls an agent an agent, whatever role its membership carries", () => {
		const detail = personDetail({ ...member("bot", "member"), kind: "agent", displayName: "Rae's agent" });

		expect(detail?.standing).toBe("Agent");
	});

	it("leaves the email out when the member has none", () => {
		expect(personDetail({ ...member("jun", "viewer"), displayName: "Jun Park" })?.email).toBeUndefined();
	});

	it("does not repeat an email that already stands in for the name", () => {
		const detail = personDetail({ ...member("ana", "member"), email: "ana@northwind.co" });

		expect(detail?.name).toBe("ana@northwind.co");
		expect(detail?.email).toBeUndefined();
	});

	it("has nothing to show for a member with neither a name nor an email", () => {
		expect(personDetail(member("ghost", "member"))).toBeNull();
	});
});
