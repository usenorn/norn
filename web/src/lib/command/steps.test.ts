import { describe, expect, it } from "vitest";
import { backlogOption, changeFor, stepOptions } from "./steps";

describe("steps", () => {
	it("moves an issue back to the backlog by clearing its cycle", () => {
		expect(changeFor("cycle", backlogOption)).toEqual({ clearCycle: true });
		expect(changeFor("cycle", "c1")).toEqual({ cycleId: "c1" });
	});

	it("offers only the team's own labels besides workspace labels", () => {
		const options = stepOptions("label", {
			teamId: "t1",
			members: [],
			states: [],
			cycles: [],
			labels: [
				{ id: "l1", workspaceId: "w", name: "Bug", description: "", color: "magenta" },
				{ id: "l2", workspaceId: "w", teamId: "t1", name: "Design", description: "", color: "violet" },
				{ id: "l3", workspaceId: "w", teamId: "t2", name: "Infra", description: "", color: "cyan" },
			],
		});

		expect(options.map((option) => option.label)).toEqual(["Bug", "Design"]);
	});

	it("offers only workspace labels when no team is in scope", () => {
		const options = stepOptions("label", {
			teamId: null,
			members: [],
			states: [],
			cycles: [],
			labels: [
				{ id: "l1", workspaceId: "w", name: "Bug", description: "", color: "magenta" },
				{ id: "l2", workspaceId: "w", teamId: "t1", name: "Design", description: "", color: "violet" },
				{ id: "l3", workspaceId: "w", teamId: "t2", name: "Infra", description: "", color: "cyan" },
			],
		});

		expect(options.map((option) => option.label)).toEqual(["Bug"]);
	});
});
