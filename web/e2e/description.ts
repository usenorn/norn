import { expect, type Page } from "@playwright/test";

export async function editDescription(page: Page) {
	const writing = page.getByRole("textbox", { name: "Description" });

	await expect(async () => {
		await page.getByTitle("Edit the description").click({ timeout: 1_000 });
		await expect(writing).toBeVisible({ timeout: 1_000 });
	}).toPass();

	return writing;
}
