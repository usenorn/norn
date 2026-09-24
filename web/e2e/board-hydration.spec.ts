import { expect, test } from "@playwright/test";
import { expectBoardLive, moveCard } from "./board";
import { at, fixture } from "./fixture";

const hydrationDelay = 3000;

test("a drop lands on a board whose route chunks arrive late", async ({ page }) => {
	const moved = fixture().issues[4];

	await page.route("**/_app/immutable/nodes/*.js", async (route) => {
		await new Promise((settle) => setTimeout(settle, hydrationDelay));
		await route.continue();
	});

	await page.goto(at(`/teams/${fixture().teamKey}/issues?layout=board&group=priority&empty=1`));

	await expect(page.locator(`[data-issue="${moved.id}"]`)).toBeVisible();
	await expectBoardLive(page);

	await moveCard(page, moved.id, "High");

	await expect(page.getByText(`Set ${moved.reference} to high priority`)).toBeVisible();

	await moveCard(page, moved.id, "Low");

	await expect(page.getByText(`Set ${moved.reference} to low priority`)).toBeVisible();
});
