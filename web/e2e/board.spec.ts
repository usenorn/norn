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
