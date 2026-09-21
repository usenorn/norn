import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

test("keys pressed while the list is still hydrating still select", async ({ page }) => {
	const held = fixture();

	await page.route("**/_app/immutable/nodes/*.js", async (route) => {
		await new Promise((wake) => setTimeout(wake, 600));
		await route.continue();
	});

	await page.goto(at(`/teams/${held.teamKey}/issues`), { waitUntil: "commit" });

	await expect(page.locator("[data-issue]").first()).toBeVisible();

	await page.keyboard.press("j");
	await page.keyboard.press("x");

	await expect(page.getByText("1 selected")).toBeVisible();
});
