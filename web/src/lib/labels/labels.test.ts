import { describe, expect, it } from "vitest";
import { randomLabelColor } from "./labels";

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
