import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

function numberOf(reference: string): string {
	return reference.split("-")[1];
}

test("a bare issue number finds that issue and opens it", async ({ page }) => {
	const reference = fixture().issues[0].reference;

	await page.goto(at("/my-tasks"));
	await page.getByRole("button", { name: /Search/ }).first().click();

	await page.getByRole("combobox").fill(numberOf(reference));

	const found = page.getByRole("option", { name: new RegExp(reference) });

	await expect(found).toBeVisible();

	await found.click();

	await expect(page).toHaveURL(new RegExp(`/issues/${reference}$`));
});

test("the number with its team key in front finds the same issue", async ({ page }) => {
	const reference = fixture().issues[0].reference;
	const [key, number] = reference.split("-");

	await page.goto(at("/my-tasks"));
	await page.getByRole("button", { name: /Search/ }).first().click();

	await page.getByRole("combobox").fill(`${key.toLowerCase()}${number}`);

	await expect(page.getByRole("option", { name: new RegExp(reference) })).toBeVisible();
});
