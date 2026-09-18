import { describe, expect, it } from "vitest";
import { burndownSeries, type Cycle, type CycleBurndown, type CycleBurndownPoint } from "./cycles";

const cycle = (startsOn: string, endsOn: string) =>
	({ startsOn, endsOn, name: "Cycle 7" }) as Cycle;

const day = (on: string, scope: number, remaining: number): CycleBurndownPoint => ({
	on,
	scope,
	remaining,
	unknown: 0,
	unrecorded: false,
});

const burndown = (points: CycleBurndownPoint[]): CycleBurndown => ({ points, whole: true });

describe("the burndown a cycle chart draws", () => {
	it("leaves a day out of the line when its standing is not in the record", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-13"),
			burndown([
				day("2026-08-10", 8, 8),
				{ ...day("2026-08-11", 8, 6), unknown: 2 },
				day("2026-08-12", 8, 4),
				day("2026-08-13", 8, 1),
			])
		);

		expect(series?.days.map((one) => one.remaining)).toEqual([8, null, 4, 1]);
		expect(series?.days.map((one) => one.scope)).toEqual([8, null, 8, 8]);
		expect(series?.uncertain).toBe(1);
	});

	it("marks a day standing alone between gaps so it is drawn as a point", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-13"),
			burndown([
				{ ...day("2026-08-10", 5, 5), unrecorded: true },
				day("2026-08-11", 5, 4),
				{ ...day("2026-08-12", 5, 3), unrecorded: true },
				day("2026-08-13", 5, 2),
			])
		);

		expect(series?.isolated.map((one) => one.at)).toEqual([1, 3]);
		expect(series?.missing).toBe(2);
	});

	it("keeps a run of neighbouring days out of the isolated points", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-12"),
			burndown([day("2026-08-10", 4, 4), day("2026-08-11", 4, 2), day("2026-08-12", 4, 0)])
		);

		expect(series?.isolated).toEqual([]);
	});

	it("draws no ideal line when the day the cycle started with is not in the record", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-12"),
			burndown([
				{ ...day("2026-08-10", 4, 4), unrecorded: true },
				day("2026-08-11", 4, 2),
				day("2026-08-12", 4, 0),
			])
		);

		expect(series?.hasIdeal).toBe(false);
		expect(series?.days.every((one) => one.ideal === null)).toBe(true);
	});

	it("runs the ideal line from the scope it started with down to nothing", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-12"),
			burndown([day("2026-08-10", 4, 4), day("2026-08-11", 4, 2), day("2026-08-12", 4, 0)])
		);

		expect(series?.days.map((one) => one.ideal)).toEqual([4, 2, 0]);
	});

	it("spans a single day cycle without collapsing the scale", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-10"),
			burndown([day("2026-08-10", 3, 3)])
		);

		expect(series?.span).toBe(1);
		expect(series?.ceiling).toBe(3);
		expect(series?.isolated.map((one) => one.at)).toEqual([0]);
	});

	it("keeps the scale above zero when nothing was ever in the cycle", () => {
		const series = burndownSeries(
			cycle("2026-08-10", "2026-08-11"),
			burndown([day("2026-08-10", 0, 0), day("2026-08-11", 0, 0)])
		);

		expect(series?.ceiling).toBe(1);
	});

	it("draws nothing at all when the record is empty", () => {
		expect(burndownSeries(cycle("2026-08-10", "2026-08-11"), burndown([]))).toBeNull();
	});
});
