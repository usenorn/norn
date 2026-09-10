import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const popup = '[data-slot="popover-content"]';

async function onScreen(page: Page) {
	return await page.locator(popup).evaluate((node) => {
		const box = node.getBoundingClientRect();

		return (
			box.left >= 0 &&
			box.top >= 0 &&
			box.right <= window.innerWidth &&
			box.bottom <= window.innerHeight &&
			box.height > 0
		);
	});
}

async function openComposer(page: Page) {
	await page.goto(at("/my-tasks"));
	await page.getByRole("button", { name: "New task" }).click();
	await expect(page.getByRole("dialog")).toBeVisible();

	const writing = page.getByRole("textbox", { name: "Description" });

	await writing.click();

	return writing;
}

test.describe("the suggestions in an editor", () => {
	test("open where the caret is, inside the create dialog", async ({ page }) => {
		await openComposer(page);

		for (const [typed, expected] of [
			["@rae", fixture().displayName],
			["/head", "Heading 1"],
			["#prora", "Proration is off by one day on annual plans"],
		]) {
			await page.keyboard.type(typed);
			await expect(page.getByRole("option", { name: new RegExp(expected) }).first()).toBeVisible();
			expect(await onScreen(page), `${typed} is on screen`).toBe(true);

			for (let at = 0; at < typed.length; at += 1) await page.keyboard.press("Backspace");
			await expect(page.locator(popup)).toHaveCount(0);
		}
	});

	test("close on escape without taking the dialog with them", async ({ page }) => {
		await openComposer(page);

		await page.keyboard.type("/");
		await expect(page.locator(popup)).toBeVisible();

		await page.keyboard.press("Escape");
		await expect(page.locator(popup)).toHaveCount(0);
		await expect(page.getByRole("dialog")).toBeVisible();

		await page.keyboard.press("Escape");
		await expect(page.getByRole("dialog")).toHaveCount(0);
	});

	test("keep the issue list open when the slash menu asks for one", async ({ page }) => {
		await openComposer(page);

		await page.keyboard.type("/issue");
		await page.getByRole("option", { name: /Issue/ }).first().click();

		await expect(page.locator(popup)).toBeVisible();

		await page.keyboard.type("prora");

		await expect(
			page.getByRole("option", { name: /Proration is off by one day/ }).first()
		).toBeVisible();
	});

	test("work the same on the issue page, where there is no dialog", async ({ page }) => {
		await page.goto(at(`/issues/${fixture().issues[0].reference}`));
		await page.getByRole("button", { name: "Edit" }).first().click();

		const writing = page.getByRole("textbox", { name: "Description" });

		await writing.click();
		await page.keyboard.type("@rae");

		await expect(
			page.getByRole("option", { name: new RegExp(fixture().displayName) }).first()
		).toBeVisible();
		expect(await onScreen(page)).toBe(true);
	});
});

test("the placeholder is there before anybody clicks, and follows the caret", async ({ page }) => {
	await page.goto(at("/my-tasks"));
	await page.getByRole("button", { name: "New task" }).click();
	await expect(page.getByRole("dialog")).toBeVisible();

	const shown = () =>
		page.evaluate(() => {
			const editor = document.querySelector('[aria-label="Description"]');
			const blocks = [...(editor?.querySelectorAll("p") ?? [])];

			return blocks.map((block) => getComputedStyle(block, "::before").content !== "none");
		});

	expect(await shown()).toEqual([true]);

	await page.getByRole("textbox", { name: "Description" }).click();
	await page.keyboard.press("Enter");

	expect(await shown()).toEqual([false, true]);
});

test("no editor carries a formatting toolbar any more", async ({ page }) => {
	await page.goto(at(`/issues/${fixture().issues[0].reference}`));
	await page.getByRole("button", { name: "Edit" }).first().click();

	await expect(page.getByRole("textbox", { name: "Description" })).toBeVisible();
	await expect(page.getByRole("toolbar", { name: "Formatting" })).toHaveCount(0);
});
