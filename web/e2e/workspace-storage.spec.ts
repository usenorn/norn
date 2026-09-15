import { expect, test } from "@playwright/test";
import { at } from "./fixture";

test("a workspace's settings show how much of its 1 GB it is storing", async ({ page }) => {
	await page.goto(at("/settings"));

	const storage = page
		.locator("section")
		.filter({ has: page.getByRole("heading", { name: "Storage", exact: true }) });

	await expect(storage.getByText(/ of 1 GB$/)).toBeVisible();
	await expect(storage.getByText(/% used$/)).toBeVisible();
	await expect(
		storage.getByRole("progressbar", { name: "Storage used in this workspace" })
	).toBeVisible();
	await expect(storage.getByText(/files brought in by an import count/)).toBeVisible();
});
