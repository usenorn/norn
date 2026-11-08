import { devices, expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

async function watching(page: Page): Promise<void> {
	await page.goto(at(`/issues/${fixture().issues[0].reference}`));

	const collapsed = page.getByRole("button", { name: /^Details/ });

	if (await collapsed.isVisible()) await collapsed.click();

	await expect(page.getByText("Watching", { exact: true })).toBeVisible();

	const follow = page.getByRole("button", { name: "Follow this issue" });

	if (await follow.isVisible()) {
		await follow.click();
		await expect(page.getByRole("button", { name: "Stop following this issue" })).toBeVisible();
	}

	await expect(page.getByRole("button", { name: fixture().displayName, exact: true })).toBeVisible();
}

function details(page: Page) {
	return page.locator("[data-slot='tooltip-content']");
}

test("a watcher's details open when the avatar takes keyboard focus", async ({ page }) => {
	await watching(page);

	const avatar = page.getByRole("button", { name: fixture().displayName, exact: true });

	await page.getByRole("button", { name: "Stop following this issue" }).focus();

	for (let pressed = 0; pressed < 60; pressed += 1) {
		if (await avatar.evaluate((node) => node === document.activeElement)) break;

		await page.keyboard.press("Tab");
	}

	await expect(avatar).toBeFocused();
	await expect(details(page)).toContainText(fixture().displayName);
	await expect(details(page)).toContainText(fixture().email);

	await page.keyboard.press("Escape");
	await expect(details(page)).toBeHidden();
});

test.describe("on a touch screen", () => {
	const phone = devices["Pixel 7"];

	test.use({
		viewport: phone.viewport,
		userAgent: phone.userAgent,
		deviceScaleFactor: phone.deviceScaleFactor,
		hasTouch: true,
	});

	test("a tap opens a watcher's details and the next tap closes them", async ({ page }) => {
		await watching(page);

		const avatar = page.getByRole("button", { name: fixture().displayName, exact: true });

		await avatar.tap();
		await expect(details(page)).toContainText(fixture().email);

		await avatar.tap();
		await expect(details(page)).toBeHidden();
	});
});
