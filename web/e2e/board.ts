import { expect, type Page } from "@playwright/test";

export async function expectBoardLive(page: Page): Promise<void> {
	await expect
		.poll(() =>
			page.evaluate(() => {
				const landing = document.querySelector('[role="group"]');

				if (!landing) return false;

				const probe = new DragEvent("dragover", {
					dataTransfer: new DataTransfer(),
					bubbles: true,
					cancelable: true,
				});

				landing.dispatchEvent(probe);

				return probe.defaultPrevented;
			})
		)
		.toBe(true);
}

export async function moveCard(page: Page, issueId: string, column: string) {
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
