import { expect, test, type Page } from "@playwright/test";
import { at, fixture } from "./fixture";

const priorityOf = (page: Page, reference: string) =>
	page.locator(`button[aria-label="Change priority on ${reference}"] svg`).first();

async function setPriority(page: Page, reference: string, name: string) {
	await page.locator(`button[aria-label="Change priority on ${reference}"]`).click();
	await page.getByRole("option", { name: `${name} ${name}` }).click();
}

test.describe("an optimistic change", () => {
	test("goes back when the server refuses it, and says why", async ({ page }) => {
		const target = fixture().issues[0].reference;

		await page.goto(at(`/teams/${fixture().teamKey}/issues`));
		await expect(priorityOf(page, target)).toHaveAttribute("aria-label", "Urgent");

		await page.route("**/v1/workspaces/*/issues/*", async (route) => {
			if (route.request().method() !== "PATCH") return route.continue();

			await route.fulfill({
				status: 409,
				contentType: "application/problem+json",
				body: JSON.stringify({ code: "issue_stale", title: "Stale", status: 409 }),
			});
		});

		await setPriority(page, target, "Low");

		await expect(page.getByText(/changed something while you were editing/)).toBeVisible();
		await expect(priorityOf(page, target)).toHaveAttribute("aria-label", "Urgent");
	});

	test("reports an outcome nobody can know rather than claiming nothing changed", async ({
		page,
	}) => {
		const target = fixture().issues[1].reference;

		await page.goto(at(`/teams/${fixture().teamKey}/issues`));

		await page.route("**/v1/workspaces/*/issues/*", async (route) => {
			if (route.request().method() !== "PATCH") return route.continue();

			await route.fulfill({
				status: 500,
				contentType: "application/problem+json",
				body: JSON.stringify({ title: "Internal", status: 500 }),
			});
		});

		await setPriority(page, target, "Low");

		await expect(page.getByText(/could not tell whether that went through/)).toBeVisible();
	});

	test("undo says so when it is refused, and leaves the change standing", async ({ page }) => {
		const target = fixture().issues[2].reference;

		await page.goto(at(`/teams/${fixture().teamKey}/issues`));
		await setPriority(page, target, "Low");

		await expect(priorityOf(page, target)).toHaveAttribute("aria-label", "Low");

		await page.route("**/v1/workspaces/*/issues/*", async (route) => {
			if (route.request().method() !== "PATCH") return route.continue();

			await route.fulfill({
				status: 403,
				contentType: "application/problem+json",
				body: JSON.stringify({ code: "issue_forbidden", title: "Forbidden", status: 403 }),
			});
		});

		await page.getByRole("button", { name: "Undo" }).click();

		await expect(page.getByText(/do not have permission/)).toBeVisible();
		await expect(priorityOf(page, target)).toHaveAttribute("aria-label", "Low");
	});
});
