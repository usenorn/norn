import { describe, expect, it } from "vitest";
import { teamSettingsEntryAt, teamSettingsGroups } from "./team-settings-sections";
import { teamSettingsPath, teamSettingsSections } from "./teams";

describe("the addresses of a team's settings", () => {
	it("keeps the team's settings at one address and gives each section its own below it", () => {
		expect(teamSettingsPath("northwind", "bil")).toBe("/northwind/settings/teams/BIL");
		expect(teamSettingsPath("northwind", "BIL", "cycles")).toBe(
			"/northwind/settings/teams/BIL/cycles"
		);
		expect(teamSettingsPath("northwind", "BIL", "source-control")).toBe(
			"/northwind/settings/teams/BIL/source-control"
		);
	});

	it("names the section an address points at, and nothing for the list of sections", () => {
		expect(teamSettingsEntryAt("/northwind/settings/teams/BIL/cycles")?.title).toBe("Cycles");
		expect(teamSettingsEntryAt("/northwind/settings/teams/BIL/email")?.title).toBe(
			"Issues by email"
		);
		expect(teamSettingsEntryAt("/northwind/settings/teams/BIL")).toBeNull();
	});

	it("lists every section exactly once across the groups", () => {
		const listed = teamSettingsGroups.flatMap((group) => group.entries.map((entry) => entry.section));

		expect([...listed].sort()).toEqual([...teamSettingsSections].sort());
		expect(teamSettingsGroups.map((group) => group.label)).toEqual([
			"Team",
			"Work",
			"Intake & automation",
		]);
	});
});
