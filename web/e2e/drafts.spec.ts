import { expect, test, type Page } from "@playwright/test";
import { at } from "./fixture";

const screenshot = `async () => {
	const canvas = document.createElement("canvas");

	canvas.width = 40;
	canvas.height = 30;
	canvas.getContext("2d").fillRect(0, 0, 40, 30);

	return await new Promise((done) => canvas.toBlob(done, "image/png"));
}`;

function offered(page: Page, title: string) {
	return page.getByRole("dialog").getByRole("button", { name: title, exact: true });
}

async function raise(page: Page, title: string) {
	await page.getByRole("button", { name: "New task" }).first().click();
	await page.getByRole("textbox", { name: "Issue title" }).fill(title);
}

async function leaveUnfinished(page: Page, title: string) {
	await raise(page, title);
	await page.keyboard.press("Escape");
	await page.getByRole("button", { name: "New task" }).first().click();
	await expect(offered(page, title)).toBeVisible();
}

test("creating an issue leaves nothing to resume", async ({ page }) => {
	const anchor = `Anchor ${Date.now()}`;
	const title = `Filed with a picture ${Date.now()}`;

	await page.goto(at("/my-tasks"));
	await leaveUnfinished(page, anchor);

	await page.getByRole("textbox", { name: "Issue title" }).fill(title);
	await page.getByRole("textbox", { name: "Description" }).click();
	await page.keyboard.type("a picture holds the form open until the upload is done");

	await page.evaluate(async (make) => {
		const blob = await eval(`(${make})`)();
		const data = new DataTransfer();

		data.items.add(new File([blob], "filed.png", { type: "image/png" }));

		document
			.querySelector('[aria-label="Description"]')
			?.dispatchEvent(
				new ClipboardEvent("paste", { clipboardData: data, bubbles: true, cancelable: true })
			);
	}, screenshot);

	await expect(page.locator('[aria-label="Description"] .animate-pulse')).toHaveCount(1);

	await page.getByRole("button", { name: /Create issue/ }).click();

	await expect(page.getByRole("link", { name: new RegExp(title) }).first()).toBeVisible();

	await page.getByRole("button", { name: "New task" }).first().click();

	await expect(offered(page, anchor)).toBeVisible();
	await expect(offered(page, title)).toHaveCount(0);
});

test("closing the form with something typed keeps it, and it can be thrown away", async ({
	page,
}) => {
	const title = `Left unfinished ${Date.now()}`;

	await page.goto(at("/my-tasks"));
	await leaveUnfinished(page, title);

	await page.getByRole("button", { name: `Throw away ${title}` }).click();

	await expect(offered(page, title)).toHaveCount(0);
});

test("creating from a resumed draft removes that draft", async ({ page }) => {
	const anchor = `Anchor ${Date.now()}`;
	const title = `Resumed then filed ${Date.now()}`;

	await page.goto(at("/my-tasks"));
	await leaveUnfinished(page, anchor);
	await page.keyboard.press("Escape");

	await leaveUnfinished(page, title);
	await offered(page, title).click();

	await expect(page.getByRole("textbox", { name: "Issue title" })).toHaveValue(title);

	await page.getByRole("button", { name: /Create issue/ }).click();

	await expect(page.getByRole("link", { name: new RegExp(title) }).first()).toBeVisible();

	await page.getByRole("button", { name: "New task" }).first().click();

	await expect(offered(page, anchor)).toBeVisible();
	await expect(offered(page, title)).toHaveCount(0);
});
