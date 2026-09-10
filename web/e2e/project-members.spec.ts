import { expect, test } from "@playwright/test";
import { at } from "./fixture";

test("searching for somebody to add says which of the three things happened", async ({ page }) => {
	await page.goto(at("/projects/billing"));

	const search = page.getByRole("search").getByRole("searchbox");

	await search.fill("nobodyatallhere");

	await expect(page.getByText(/matches that, or they are already here/)).toBeVisible();

	await page.route("**/v1/workspaces/*/members*", async (route) => {
		await route.fulfill({
			status: 503,
			contentType: "application/problem+json",
			body: JSON.stringify({ title: "Unavailable", status: 503 }),
		});
	});

	await search.fill("rae");

	await expect(page.getByText(/could not search just now/)).toBeVisible();
});
