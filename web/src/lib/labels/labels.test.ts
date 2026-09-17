import { describe, expect, it } from "vitest";
import {
	randomLabelColor,
	uncountedLabel,
	usageFor,
	usageLabel,
	usageOf,
	type Label,
} from "./labels";

describe("the colour a new label is given", () => {
	it.each([
		[0, "cyan"],
		[0.2, "blue"],
		[0.4, "violet"],
		[0.6, "orchid"],
		[0.8, "magenta"],
		[0.999999, "magenta"],
	] as const)("reaches every colour but neutral: %s gives %s", (drawn, expected) => {
		expect(randomLabelColor(() => drawn)).toBe(expected);
	});

	it("is never neutral, whatever the draw", () => {
		const draws = [0, 0.0001, 0.19999, 0.5, 0.79999, 0.99999, 1];

		expect(draws.map((drawn) => randomLabelColor(() => drawn))).not.toContain("neutral");
	});
});

describe("issue counts when the tally request does not come back", () => {
	const bug: Label = {
		id: "l1",
		workspaceId: "w",
		name: "Bug",
		description: "",
		color: "magenta",
	};

	it("reports a failed read as uncounted rather than as zero", () => {
		const usage = usageFor({ data: undefined });

		expect(usage.kind).toBe("uncounted");
		expect(usageOf(usage, bug)).toBeNull();
		expect(usageLabel(usage, bug)).toBe(uncountedLabel);
		expect(usageLabel(usage, bug)).not.toMatch(/\d/);
	});

	it("keeps a real zero distinguishable from an unread count", () => {
		const counted = usageFor({ data: { groups: [] } });

		expect(counted.kind).toBe("counted");
		expect(usageOf(counted, bug)).toBe(0);
		expect(usageLabel(counted, bug)).toBe("0 issues");
	});

	it("counts the labels the tally names and zeroes only the ones it omits", () => {
		const usage = usageFor({
			data: { groups: [{ key: "l1", issues: 86 }, { key: "", issues: 4 }] },
		});

		expect(usageLabel(usage, bug)).toBe("86 issues");
		expect(usageOf(usage, { ...bug, id: "l2" })).toBe(0);
	});
});
