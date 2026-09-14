import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const popup = '[data-slot="popover-content"]';

const screenshot = `async () => {
	const canvas = document.createElement("canvas");

	canvas.width = 40;
	canvas.height = 30;
	canvas.getContext("2d").fillRect(0, 0, 40, 30);

	return await new Promise((done) => canvas.toBlob(done, "image/png"));
}`;

async function openIssue(page: Page) {
	await page.goto(at(`/issues/${fixture().issues[0].reference}`));

	const writing = page.getByRole("textbox", { name: "Write a comment" });

	await expect(writing).toBeVisible();

	return writing;
}

async function dragOverComment(
	page: Page,
	carrying: "file" | "text" | "away",
	over: "card" | "writing" = "card"
) {
	await page.evaluate(
		([what, where]) => {
			const data = new DataTransfer();
			const writing = document.querySelector('[aria-label="Write a comment"]');
			const landing = where === "writing" ? writing : writing?.closest('[role="group"]');

			if (what === "text") data.setData("text/plain", "just words");
			else data.items.add(new File([new Uint8Array(4)], "held.png", { type: "image/png" }));

			landing?.dispatchEvent(
				new DragEvent(what === "away" ? "dragleave" : "dragenter", {
					dataTransfer: data,
					bubbles: true,
					cancelable: true,
				})
			);
		},
		[carrying, over] as const
	);
}

async function dropOnComment(page: Page, name: string) {
	await page.evaluate(
		async ([make, filed]) => {
			const blob = await eval(`(${make})`)();
			const data = new DataTransfer();
			const writing = document.querySelector('[aria-label="Write a comment"]');
			const box = writing?.getBoundingClientRect();

			data.items.add(new File([blob], filed, { type: "image/png" }));

			writing?.dispatchEvent(
				new DragEvent("drop", {
					dataTransfer: data,
					bubbles: true,
					cancelable: true,
					clientX: (box?.left ?? 0) + 8,
					clientY: (box?.top ?? 0) + 8,
				})
			);
		},
		[screenshot, name] as const
	);
}

test("a file dropped on a comment is attached once", async ({ page }) => {
	const reserved: string[] = [];

	page.on("request", (request) => {
		if (request.method() === "POST" && request.url().endsWith("/attachments")) {
			reserved.push(request.url());
		}
	});

	await openIssue(page);
	await dropOnComment(page, "dropped-once.png");

	await expect(page.getByText("dropped-once.png")).toHaveCount(1);
	await expect(page.locator('[aria-label="Write a comment"] img')).toHaveCount(1, {
		timeout: 20_000,
	});

	expect(reserved).toHaveLength(1);
});

test("throwing an attached file away takes the picture and the file with it", async ({ page }) => {
	const removed: string[] = [];

	page.on("request", (request) => {
		if (request.method() === "DELETE" && request.url().includes("/attachments/")) {
			removed.push(request.url());
		}
	});

	await openIssue(page);
	await dropOnComment(page, "thrown-away.png");

	const written = page.locator('[aria-label="Write a comment"] img');

	await expect(written).toHaveCount(1, { timeout: 20_000 });

	const throwAway = page.getByRole("button", { name: "Dismiss thrown-away.png" });

	await expect(throwAway).toBeVisible();
	await throwAway.click();

	await expect(page.getByText("thrown-away.png")).toHaveCount(0);
	await expect(written).toHaveCount(0);
	await expect.poll(() => removed.length).toBe(1);

	await expect
		.poll(() => page.locator('[aria-label="Write a comment"] p').count())
		.toBe(1);
});

test("the picture lands where the cursor points, so the editor keeps its own mark", async ({
	page,
}) => {
	await openIssue(page);

	const overlay = page.getByText("Drop files to attach them");

	await dragOverComment(page, "file");

	await expect(overlay).toBeVisible();

	await dragOverComment(page, "file", "writing");

	await expect(overlay).toHaveCount(0);
});

test("dragging a file over a comment says what will happen, and dragging text says nothing", async ({
	page,
}) => {
	await openIssue(page);

	const overlay = page.getByText("Drop files to attach them");

	await dragOverComment(page, "text");

	await expect(overlay).toHaveCount(0);

	await dragOverComment(page, "file");

	await expect(overlay).toBeVisible();

	await dragOverComment(page, "away");

	await expect(overlay).toHaveCount(0);
});

test("an emoji can be typed into a comment by name", async ({ page }) => {
	const writing = await openIssue(page);

	await writing.click();
	await page.keyboard.type(":tada");

	await expect(page.getByRole("option", { name: /tada/ }).first()).toBeVisible();

	await page.keyboard.press("Enter");

	await expect(writing).toContainText("🎉");
	await expect(page.locator(popup)).toHaveCount(0);
});

test("an emoji can be picked from the list beside the paperclip", async ({ page }) => {
	const writing = await openIssue(page);

	await writing.click();
	await page.getByRole("button", { name: "Emoji" }).click();
	await page.getByPlaceholder("Search emoji").fill("rocket");
	await page.getByRole("button", { name: "rocket", exact: true }).click();

	await expect(writing).toContainText("🚀");
});

test("the description keeps its colon, because emoji belong to comments", async ({ page }) => {
	await page.goto(at(`/issues/${fixture().issues[0].reference}`));
	await page.getByRole("button", { name: "Edit" }).first().click();

	const writing = page.getByRole("textbox", { name: "Description" });

	await writing.click();
	await page.keyboard.press("End");
	await page.keyboard.type(" :tada");

	await expect(page.locator(popup)).toHaveCount(0);
	await expect(writing).toContainText(":tada");
});
