import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

async function moveCard(page: Page, issueId: string, column: string) {
	await page.evaluate(
		([id, name]) => {
			const card = document.querySelector(`[data-issue="${id}"]`);
			const landing = [...document.querySelectorAll('[role="group"]')].find(
				(one) => one.getAttribute("aria-label") === name
			);

			if (!card || !landing) throw new Error(`no card ${id} or column ${name}`);

			const data = new DataTransfer();

			data.setData("text/plain", id);

			card.dispatchEvent(new DragEvent("dragstart", { dataTransfer: data, bubbles: true }));

			const box = landing.getBoundingClientRect();

			landing.dispatchEvent(
				new DragEvent("drop", {
					dataTransfer: data,
					bubbles: true,
					cancelable: true,
					clientX: box.left + box.width / 2,
					clientY: box.bottom - 4,
				})
			);
		},
		[issueId, column] as const
	);
}

test("moving a card twice over is not reported as somebody else's change", async ({ page }) => {
	const moved = fixture().issues[4];

	await page.route("**/__data.json*", async (route) => {
		await new Promise((settle) => setTimeout(settle, 1500));
		await route.continue();
	});

	await page.goto(at(`/teams/${fixture().teamKey}/issues?layout=board&group=priority&empty=1`));

	await expect(page.locator(`[data-issue="${moved.id}"]`)).toBeVisible();

	await moveCard(page, moved.id, "High");

	await expect(page.getByText(`Set ${moved.reference} to high priority`)).toBeVisible();

	await moveCard(page, moved.id, "Low");

	await expect(page.getByText(`Set ${moved.reference} to low priority`)).toBeVisible();
	await expect(page.getByText(/while you were editing/)).toHaveCount(0);
});

type Columns = string[][];

async function columnsOf(page: Page): Promise<Columns> {
	return page.evaluate(() =>
		[...document.querySelectorAll('[role="group"][aria-label]')].map((column) =>
			[...column.querySelectorAll("[data-issue]")].map((card) => card.getAttribute("data-issue") ?? "")
		)
	);
}

async function cursorOn(page: Page): Promise<string | null> {
	return page.locator('[data-cursor="true"]').getAttribute("data-issue");
}

function placeOf(columns: Columns, id: string | null): { column: number; row: number } {
	const column = columns.findIndex((cards) => cards.includes(id ?? ""));

	return { column, row: columns[column]?.indexOf(id ?? "") ?? -1 };
}

function besideOf(columns: Columns, id: string | null, by: number): string | null {
	const { column, row } = placeOf(columns, id);

	for (let next = column + by; next >= 0 && next < columns.length; next += by) {
		const cards = columns[next];

		if (cards.length > 0) return cards[Math.min(row, cards.length - 1)];
	}

	return id;
}

async function openBoard(page: Page, layout = "board") {
	await page.goto(at(`/teams/${fixture().teamKey}/issues?layout=${layout}&group=priority&empty=1`));
	await expect(page.locator('[data-cursor="true"]')).toBeVisible();
}

async function prevented(page: Page, key: string): Promise<boolean> {
	return page.evaluate(
		(pressed) =>
			new Promise<boolean>((settle) => {
				window.addEventListener(
					"keydown",
					(event) => setTimeout(() => settle(event.defaultPrevented)),
					{ once: true }
				);
				document.activeElement?.dispatchEvent(
					new KeyboardEvent("keydown", { key: pressed, bubbles: true, cancelable: true })
				);
			}),
		key
	);
}

test.describe("board arrows", () => {
	test("right and left arrows step the cursor between non-empty columns", async ({ page }) => {
		await openBoard(page);

		const columns = await columnsOf(page);
		const start = await cursorOn(page);
		const right = besideOf(columns, start, 1);

		expect(right).not.toBe(start);

		await page.keyboard.press("ArrowRight");
		await expect.poll(() => cursorOn(page)).toBe(right);

		await page.keyboard.press("ArrowLeft");
		await expect.poll(() => cursorOn(page)).toBe(besideOf(columns, right, -1));
	});

	test("stepping sideways keeps the row, or lands on the last card of a shorter column", async ({
		page,
	}) => {
		await openBoard(page);

		const columns = await columnsOf(page);
		const deep = columns.findIndex((cards) => cards.length > 1);

		expect(deep).toBeGreaterThanOrEqual(0);

		const flat = columns.flat();
		const target = columns[deep][1];

		for (let moved = 0; moved < flat.indexOf(target); moved += 1) {
			await page.keyboard.press("ArrowDown");
		}

		await expect.poll(() => cursorOn(page)).toBe(target);

		const by = columns.slice(deep + 1).some((cards) => cards.length > 0) ? 1 : -1;
		const landing = besideOf(columns, target, by);
		const { column, row } = placeOf(columns, landing);

		expect(row).toBe(Math.min(1, columns[column].length - 1));

		await page.keyboard.press(by === 1 ? "ArrowRight" : "ArrowLeft");
		await expect.poll(() => cursorOn(page)).toBe(landing);
	});

	test("the cursor holds its card at either edge of the board", async ({ page }) => {
		await openBoard(page);

		const columns = await columnsOf(page);
		const first = columns.find((cards) => cards.length > 0)?.[0] ?? null;
		const last = columns.findLast((cards) => cards.length > 0)?.[0] ?? null;

		await expect.poll(() => cursorOn(page)).toBe(first);
		await page.keyboard.press("ArrowLeft");
		await expect.poll(() => cursorOn(page)).toBe(first);

		for (let stepped = 0; stepped < columns.length; stepped += 1) {
			await page.keyboard.press("ArrowRight");
		}

		await expect.poll(() => cursorOn(page)).toBe(last);
		await page.keyboard.press("ArrowRight");
		await expect.poll(() => cursorOn(page)).toBe(last);
	});

	test("sideways arrows leave the list layout alone", async ({ page }) => {
		await openBoard(page, "list");

		const start = await cursorOn(page);

		expect(await prevented(page, "ArrowRight")).toBe(false);
		expect(await prevented(page, "ArrowLeft")).toBe(false);
		await expect.poll(() => cursorOn(page)).toBe(start);
	});

	test("right arrow moves the current card as soon as the board opens", async ({ page }) => {
		await openBoard(page);

		const columns = await columnsOf(page);
		const start = await cursorOn(page);
		const right = besideOf(columns, start, 1);

		expect(right).not.toBe(start);

		await page.keyboard.press("ArrowRight");
		await expect.poll(() => cursorOn(page)).toBe(right);
	});

	test("sideways arrows stay with a text field and with an open dialog", async ({ page }) => {
		await openBoard(page);

		const start = await cursorOn(page);

		await page.getByRole("button", { name: "Save as view" }).click();

		const name = page.getByLabel("Save what you are looking at, for yourself");

		await name.fill("Urgent");
		await name.press("ArrowLeft");
		await name.press("ArrowLeft");

		expect(await name.evaluate((field: HTMLInputElement) => field.selectionStart)).toBe(4);
		await expect.poll(() => cursorOn(page)).toBe(start);

		await page.getByRole("button", { name: "Cancel" }).click();
		await page.keyboard.press("?");
		await expect(page.getByRole("dialog")).toBeVisible();
		await page.keyboard.press("ArrowRight");
		await expect.poll(() => cursorOn(page)).toBe(start);
	});
});
