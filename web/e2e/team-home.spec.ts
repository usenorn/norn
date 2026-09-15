import { expect, test } from "@playwright/test";
import { at, fixture, ordinaryStatePath } from "./fixture";

test("a team's home carries an overview whose description can be written on the spot", async ({
	page,
}) => {
	const said = `Looks after billing ${Date.now()}`;

	await page.goto(at(`/teams/${fixture().teamKey}`));

	await expect(page.getByRole("heading", { name: "Billing", exact: true }).first()).toBeVisible();

	await page.getByRole("button", { name: "Edit" }).click();
	await page.getByLabel("Description").fill(said);
	await page.getByRole("button", { name: "Save" }).click();

	await expect(page.getByText(said)).toBeVisible();
	await expect(page.getByRole("button", { name: "Edit" })).toBeVisible();

	await page.reload();

	await expect(page.getByText(said)).toBeVisible();
});

test("the members tab has its own address and lists who is on the team", async ({ page }) => {
	await page.goto(at(`/teams/${fixture().teamKey}`));

	await page.getByRole("link", { name: "Members" }).click();

	await expect(page).toHaveURL(new RegExp(`/teams/${fixture().teamKey}\\?tab=members$`));
	await expect(page.getByText(fixture().email)).toBeVisible();
	await expect(
		page.getByRole("button", { name: `Remove ${fixture().displayName} from Billing` })
	).toBeVisible();

	await page.getByRole("link", { name: "Overview" }).click();

	await expect(page).toHaveURL(new RegExp(`/teams/${fixture().teamKey}$`));
});

test.describe("somebody who is on the team but does not run the workspace", () => {
	test.use({ storageState: ordinaryStatePath });

	test("is not offered what the server would refuse", async ({ page }) => {
		await page.goto(at(`/teams/${fixture().teamKey}`));

		await expect(page.getByRole("heading", { name: "Billing", exact: true }).first()).toBeVisible();
		await expect(page.getByRole("button", { name: "Edit" })).toHaveCount(0);

		await page.getByRole("link", { name: "Members" }).click();

		await expect(page.getByText(fixture().email)).toBeVisible();
		await expect(page.getByRole("button", { name: /^Remove / })).toHaveCount(0);
		await expect(page.getByRole("button", { name: "Add someone" })).toHaveCount(0);
	});
});
