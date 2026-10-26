import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

test.describe("labels on the issue page", () => {
	test("a label created from the picker lands on the issue with a colour", async ({ page }) => {
		const name = `Created ${Date.now()}`;

		await page.goto(at(`/issues/${fixture().issues[4].reference}`));

		const field = page.getByRole("button", { name: /^Labels:/ });

		await field.click();
		await page.getByPlaceholder("Add or create a label…").fill(name);

		const created = page.waitForResponse(
			(response) =>
				response.request().method() === "POST" && /\/v1\/workspaces\/[^/]+\/labels$/.test(response.url())
		);

		await page.getByRole("option", { name: `Create “${name}”` }).click();

		const label = (await (await created).json()) as { color: string };

		expect(label.color).not.toBe("neutral");

		await page.keyboard.press("Escape");
		await expect(field).toContainText(name);

		await page.waitForLoadState("networkidle");
		await page.reload();

		await expect(page.getByRole("button", { name: /^Labels:/ })).toContainText(name);
	});
});
