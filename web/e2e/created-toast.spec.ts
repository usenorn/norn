import { expect, test } from "@playwright/test";
import { at } from "./fixture";

test.use({ permissions: ["clipboard-read", "clipboard-write"] });

test("the toast for a new issue shows how its description begins and copies its link", async ({
	page,
}) => {
	const title = `Announced with a summary ${Date.now()}`;

	await page.goto(at("/my-tasks"));

	await page.getByRole("button", { name: "New task" }).first().click();
	await page.getByRole("textbox", { name: "Issue title" }).fill(title);
	await page.getByRole("textbox", { name: "Description" }).click();
	await page.keyboard.type("Refunds stall after checkout");
	await page.keyboard.press("Enter");
	await page.keyboard.type("for customers paying by invoice");
	await page.getByRole("button", { name: /Create issue/ }).click();

	const toast = page.getByRole("status").filter({ hasText: "Copy link" });
	const opens = toast.getByRole("link", { name: /^Created / });

	await expect(opens).toBeVisible();
	await expect(toast).toContainText("Refunds stall after checkout for customers paying by invoice");

	const href = await opens.getAttribute("href");

	expect(href).toMatch(/\/issues\/[A-Z]+-\d+$/);

	await toast.getByRole("button", { name: "Copy link" }).click();

	await expect(page.getByText(/^Copied a link to [A-Z]+-\d+$/)).toBeVisible();
	await expect
		.poll(() => page.evaluate(() => navigator.clipboard.readText()))
		.toBe(new URL(href ?? "", page.url()).href);
});
