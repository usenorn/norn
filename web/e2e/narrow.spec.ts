import { devices, expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

const screens = ["/my-tasks", "/inbox", "/projects", "/projects/billing", "/reviews"];

const widths = [
	{ name: "360px", width: 360, height: 760 },
	{ name: "200% zoom", width: 640, height: 430 },
];

for (const size of widths) {
	test(`nothing runs off the side at ${size.name}`, async ({ page }) => {
		await page.setViewportSize({ width: size.width, height: size.height });

		for (const screen of [...screens, `/teams/${fixture().teamKey}/issues`]) {
			await page.goto(at(screen));
			await expect(page.getByRole("heading").first()).toBeVisible();

			const sideways = await page.evaluate(
				() => document.documentElement.scrollWidth > document.documentElement.clientWidth
			);

			expect(sideways, `${screen} at ${size.name}`).toBe(false);
		}
	});
}

test.describe("on a touch screen", () => {
	const phone = devices["Pixel 7"];

	test.use({
		viewport: phone.viewport,
		userAgent: phone.userAgent,
		deviceScaleFactor: phone.deviceScaleFactor,
		hasTouch: true,
	});

	test("the filter and the composer open with a tap", async ({ page }) => {
		await page.goto(at("/my-tasks"));

		await page.getByRole("button", { name: "Filter" }).tap();
		await expect(page.getByRole("option", { name: "Status" })).toBeVisible();

		await page.keyboard.press("Escape");

		await page.getByRole("button", { name: "New task" }).first().tap();
		await expect(page.getByRole("dialog").getByRole("textbox", { name: "Issue title" })).toBeVisible();
	});
});
