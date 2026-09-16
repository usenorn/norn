import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;

async function seedLabel(page: Page, name: string): Promise<void> {
	const answer = await page.request.post(`/v1/workspaces/${fixture().workspaceId}/labels`, {
		headers: { origin, referer: `${origin}/` },
		data: { name, color: "cyan" },
	});

	expect(answer.ok()).toBe(true);
}

test.describe("labels on the issue page", () => {
	test("two labels picked one after another are both still there after a reload", async ({
		page,
	}) => {
		const stamp = Date.now();
		const first = `First ${stamp}`;
		const second = `Second ${stamp}`;

		await seedLabel(page, first);
		await seedLabel(page, second);

		let held = true;

		await page.route("**/v1/workspaces/*/issues/*/labels", async (route) => {
			if (held) {
				held = false;
				await new Promise((settle) => setTimeout(settle, 1_000));
			}

			await route.continue();
		});

		await page.goto(at(`/issues/${fixture().issues[3].reference}`));

		const field = page.getByRole("button", { name: "Labels: change" });

		await field.click();
		await page.getByRole("option", { name: first }).click();
		await page.getByRole("option", { name: second }).click();
		await page.keyboard.press("Escape");

		await expect(field).toContainText(first);
		await expect(field).toContainText(second);

		await page.waitForLoadState("networkidle");
		await page.reload();

		await expect(page.getByRole("button", { name: "Labels: change" })).toContainText(first);
		await expect(page.getByRole("button", { name: "Labels: change" })).toContainText(second);
	});

	test("a label created from the picker lands on the issue with a colour", async ({ page }) => {
		const name = `Created ${Date.now()}`;

		await page.goto(at(`/issues/${fixture().issues[4].reference}`));

		const field = page.getByRole("button", { name: "Labels: change" });

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

		await expect(page.getByRole("button", { name: "Labels: change" })).toContainText(name);
	});
});
