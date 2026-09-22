import { describe, expect, it } from "vitest";
import { dueLabel } from "./time";

const now = "2026-09-10T12:00:00Z";
const timezone = "UTC";

function open(due: string): string {
	return dueLabel(due, now, timezone, false);
}

function settled(due: string): string {
	return dueLabel(due, now, timezone, true);
}

describe("dueLabel", () => {
	it("calls a past due date overdue while the issue is still open", () => {
		expect(open("2026-09-09")).toBe("Overdue by a day");
		expect(open("2026-09-03")).toBe("Overdue · 3 Sept");
		expect(open("2025-12-24")).toBe("Overdue · 24 Dec 2025");
	});

	it("drops overdue once the issue is complete or abandoned", () => {
		expect(settled("2026-09-09")).toBe("Due yesterday");
		expect(settled("2026-09-03")).toBe("Due 3 Sept");
		expect(settled("2025-12-24")).toBe("Due 24 Dec 2025");
	});

	it("reads the same for today and later whatever the state", () => {
		for (const label of [open, settled]) {
			expect(label("2026-09-10")).toBe("Due today");
			expect(label("2026-09-11")).toBe("Due tomorrow");
			expect(label("2026-09-24")).toBe("Due 24 Sept");
		}
	});

	it("says there is no due date when none is set", () => {
		expect(dueLabel(undefined, now, timezone, false)).toBe("No due date");
		expect(dueLabel(undefined, now, timezone, true)).toBe("No due date");
	});
});
