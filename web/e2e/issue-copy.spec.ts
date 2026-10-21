import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

test.use({ permissions: ["clipboard-read", "clipboard-write"] });

function clipboard(page: Page) {
	return page.evaluate(() => navigator.clipboard.readText());
}

test("an issue's identifier, title and link each copy in one click", async ({ page }) => {
	const { reference } = fixture().issues[0];

	await page.goto(at(`/issues/${reference}`));

	await expect(async () => {
		await page.getByRole("button", { name: "Copy the identifier" }).click();
		await expect(page.getByText(`Copied ${reference}`)).toBeVisible({ timeout: 1_000 });
	}).toPass();
	await expect.poll(() => clipboard(page)).toBe(reference);

	const title = (await page.getByRole("heading", { level: 1 }).innerText()).trim();

	await page.getByRole("button", { name: "Copy the title" }).click();

	await expect(page.getByText("Copied the title")).toBeVisible();
	await expect.poll(() => clipboard(page)).toBe(title);

	await page.getByRole("button", { name: `Copy the link to ${reference}` }).click();

	await expect(page.getByText(`Copied a link to ${reference}`)).toBeVisible();
	await expect
		.poll(() => clipboard(page))
		.toBe(new URL(at(`/issues/${reference}`), page.url()).href);
});

test("ctrl f opens search, even from inside a comment", async ({ page }) => {
	const { reference } = fixture().issues[0];

	await page.goto(at(`/issues/${reference}`));

	const comment = page.getByRole("textbox", { name: "Write a comment" });
	const search = page.getByRole("dialog").getByRole("combobox");

	await expect(async () => {
		await comment.click();
		await page.keyboard.press("ControlOrMeta+f");
		await expect(search).toBeVisible({ timeout: 1_000 });
	}).toPass();
});
