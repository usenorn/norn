import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

test("coming back from an issue lands on the list it was opened from", async ({ page }) => {
	const held = fixture();

	await page.goto(at(`/teams/${held.teamKey}/issues`));

	await page.getByRole("button", { name: "Filter" }).click();
	await page.getByRole("option", { name: "Assignee" }).click();
	await page.getByRole("option", { name: held.displayName }).click();

	const chip = page.getByText(`Assignee: ${held.displayName}`);

	await expect(chip).toBeVisible();

	const rows = page.locator('[role="listitem"][data-issue]');
	const narrowed = await rows.count();
	const filtered = new URL(page.url());

	expect(narrowed).toBeGreaterThan(0);

	await rows.first().getByRole("link").first().click();

	await expect(page.getByRole("link", { name: "Back to the list" })).toBeVisible();

	await page.getByRole("link", { name: "Back to the list" }).click();

	await expect(chip).toBeVisible();

	const back = new URL(page.url());

	expect(back.pathname).toBe(filtered.pathname);
	expect(back.searchParams.get("assignee")).toBe(filtered.searchParams.get("assignee"));

	await expect(rows).toHaveCount(narrowed);
});

test("an issue opened on its own still offers a way back to the list", async ({ page }) => {
	const held = fixture();

	await page.goto(at(`/issues/${held.issues[0].reference}`));

	await page.getByRole("link", { name: "Back to the list" }).click();

	await expect(page).toHaveURL(new RegExp(`/${held.slug}/issues$`));
});
