import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

test("a bulk change that cannot be read back says so and can be asked again", async ({ page }) => {
	const held = fixture();

	await page.goto(at(`/teams/${held.teamKey}/issues`));

	await page.route("**/v1/workspaces/*/issues/bulk", async (route) => {
		await route.fulfill({
			status: 202,
			contentType: "application/json",
			body: JSON.stringify({ id: "bulk-e2e", status: "running", processed: 0, outcomes: [] }),
		});
	});

	await page.route("**/v1/workspaces/*/bulk-actions/*", async (route) => {
		await route.fulfill({
			status: 503,
			contentType: "application/problem+json",
			body: JSON.stringify({ title: "Unavailable", status: 503 }),
		});
	});

	await page.keyboard.press("j");
	await page.keyboard.press("x");
	await page.keyboard.press("j");
	await page.keyboard.press("x");

	await expect(page.getByText("2 selected")).toBeVisible();

	await page.getByRole("button", { name: "Set status" }).click();
	await page.getByRole("option").first().click();

	await expect(page.getByText("Working through the selection")).toBeVisible();
	await expect(page.getByText("We lost sight of this change")).toBeVisible({ timeout: 30_000 });

	await page.unroute("**/v1/workspaces/*/bulk-actions/*");
	await page.route("**/v1/workspaces/*/bulk-actions/*", async (route) => {
		await route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				id: "bulk-e2e",
				status: "complete",
				processed: 2,
				expected: 2,
				outcomes: [
					{ issueId: held.issues[0].id, reference: held.issues[0].reference, outcome: "applied" },
					{ issueId: held.issues[1].id, reference: held.issues[1].reference, outcome: "forbidden" },
				],
			}),
		});
	});

	await page.getByRole("button", { name: "Check again" }).click();

	const panel = page.getByRole("alert").filter({ hasText: "Some issues did not change" });

	await expect(panel).toBeVisible();
	await expect(panel.getByText(held.issues[1].reference, { exact: true })).toBeVisible();

	const asked: string[][] = [];

	await page.unroute("**/v1/workspaces/*/issues/bulk");
	await page.route("**/v1/workspaces/*/issues/bulk", async (route) => {
		asked.push(route.request().postDataJSON().issueIds);

		await route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				id: "bulk-e2e-again",
				status: "complete",
				processed: 1,
				expected: 1,
				outcomes: [
					{ issueId: held.issues[1].id, reference: held.issues[1].reference, outcome: "applied" },
				],
			}),
		});
	});

	await page.getByRole("button", { name: /that did not change/ }).click();

	await expect.poll(() => asked.length).toBe(1);
	expect(asked[0]).toEqual([held.issues[1].id]);
});
