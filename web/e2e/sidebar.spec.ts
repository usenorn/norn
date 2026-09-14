import { expect, test } from "@playwright/test";
import { at } from "./fixture";

test("every row in the sidebar tree shares one band, whatever its depth", async ({ page }) => {
	await page.goto(at("/my-tasks"));

	const measured = await page
		.getByRole("complementary")
		.evaluate((side) => {
			const edge = side.getBoundingClientRect();
			const rows = [...side.querySelectorAll("a")].filter((row) =>
				/Inbox|My tasks|Billing|Issues|Projects/.test(row.textContent?.trim() ?? "")
			);

			return rows.map((row) => {
				const box = row.getBoundingClientRect();
				const icon = row.querySelector("svg")?.getBoundingClientRect();

				return {
					label: row.textContent?.replace(/\s+/g, " ").trim() ?? "",
					left: Math.round(box.left - edge.left),
					right: Math.round(edge.right - box.right),
					height: Math.round(box.height),
					icon: icon ? Math.round(icon.left - edge.left) : null,
				};
			});
		});

	expect(measured.length).toBeGreaterThan(3);
	expect(new Set(measured.map((row) => row.left)).size).toBe(1);
	expect(new Set(measured.map((row) => row.right)).size).toBe(1);
	expect(new Set(measured.map((row) => row.height)).size).toBe(1);

	const team = measured.find((row) => row.label === "Billing");
	const child = measured.find((row) => row.label === "Issues");
	const top = measured.find((row) => row.label === "Inbox");

	expect(team?.icon).toBeGreaterThan(top?.icon ?? 0);
	expect(child?.icon).toBeGreaterThan(team?.icon ?? 0);
	expect((child?.icon ?? 0) - (team?.icon ?? 0)).toBe((team?.icon ?? 0) - (top?.icon ?? 0));
});
