import { expect, test, type Page } from "@playwright/test";
import { at, fixture, ordinaryStatePath } from "./fixture";

type Created = { id: string; version: number };

async function raiseDoc(page: Page, title: string): Promise<Created> {
	return page.evaluate(
		async ([workspaceId, called]) => {
			const answer = await fetch(`/v1/workspaces/${workspaceId}/docs`, {
				method: "POST",
				headers: { "content-type": "application/json" },
				body: JSON.stringify({ title: called, audience: { visibility: "workspace" }, markdown: "First words." }),
			});

			if (!answer.ok) throw new Error(`the doc was refused: ${answer.status} ${await answer.text()}`);

			return (await answer.json()) as Created;
		},
		[fixture().workspaceId, title] as const
	);
}

async function sideways(page: Page): Promise<boolean> {
	return page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
}

async function more(page: Page, item: string) {
	await page.getByRole("button", { name: "More for this doc" }).click();
	await page.getByRole("menuitem", { name: item }).click();
}

test("a person creates, writes, nests, shares, restores and archives docs on a phone", async ({ page }) => {
	const stamp = Date.now();
	const parent = `Runbook ${stamp}`;
	const child = `Restarts ${stamp}`;

	await page.setViewportSize({ width: 360, height: 760 });

	await page.goto(at("/menu"));
	await page.getByRole("link", { name: "Docs" }).click();
	await expect(page).toHaveURL(new RegExp(`/${fixture().slug}/docs$`));

	await page.getByRole("link", { name: "New doc" }).first().click();
	await page.getByRole("textbox", { name: "Title" }).fill(parent);
	await page.getByRole("button", { name: "Create doc" }).click();
	await expect(page).toHaveURL(/\/docs\/[0-9a-f-]+\/edit$/);
	expect(await sideways(page), "the editor at 360px").toBe(false);

	await page.getByRole("textbox", { name: "Doc text" }).click();
	await page.keyboard.type("Drain the worker before restarting it.");
	await expect(page.getByText("Saved", { exact: true })).toBeVisible();

	await page.getByRole("link", { name: "Done" }).click();
	await expect(page).toHaveURL(/\/docs\/[0-9a-f-]+$/);
	await expect(page.getByText("Drain the worker before restarting it.")).toBeVisible();
	expect(await sideways(page), "the reader at 360px").toBe(false);

	const parentPath = new URL(page.url()).pathname;

	await more(page, "New doc inside");
	await expect(page.getByText(`It is filed under “${parent}”.`)).toBeVisible();
	await page.getByRole("textbox", { name: "Title" }).fill(child);
	await page.getByRole("button", { name: "Create doc" }).click();
	await expect(page).toHaveURL(/\/edit$/);
	await page.getByRole("link", { name: "Done" }).click();
	await expect(page).toHaveURL(/\/docs\/[0-9a-f-]+$/);

	const childPath = new URL(page.url()).pathname;

	await page.goto(at("/docs"));
	await page.getByRole("button", { name: `Expand ${parent}` }).last().click();
	await expect(page.getByRole("link", { name: child }).last()).toBeVisible();
	expect(await sideways(page), "the docs list at 360px").toBe(false);

	await page.goto(parentPath);
	await more(page, "Share");
	const dialog = page.getByRole("dialog");
	await expect(dialog.getByText("Everybody in Northwind can read it.")).toBeVisible();
	await dialog.getByRole("button", { name: "Who reads it" }).click();
	await page.getByRole("option", { name: "Only me" }).click();
	await dialog.getByRole("button", { name: "Share" }).click();
	await expect(dialog).toBeHidden();
	await expect(page.getByText("Only me", { exact: true })).toBeVisible();

	await more(page, "History");
	await expect(page.getByRole("link", { name: /Changed who reads it/ })).toBeVisible();
	await page.getByRole("link", { name: /Started the doc/ }).click();
	await page.getByRole("button", { name: "Put this version back" }).click();
	await expect(page).toHaveURL(new RegExp(`${parentPath}$`));
	await expect(page.getByText("Nothing is written here yet.")).toBeVisible();

	await page.goto(parentPath);
	await more(page, "Archive");
	await page.getByRole("alertdialog").getByRole("button", { name: "Archive" }).click();
	await expect(page.getByText("Docs are still filed under this one. Move or archive them first.")).toBeVisible();
	await page.getByRole("button", { name: "Cancel" }).click();

	await page.goto(childPath);
	await more(page, "Archive");
	await page.getByRole("alertdialog").getByRole("button", { name: "Archive" }).click();
	await expect(page).toHaveURL(new RegExp(`${parentPath}$`));

	await more(page, "Archive");
	await page.getByRole("alertdialog").getByRole("button", { name: "Archive" }).click();
	await expect(page).toHaveURL(new RegExp(`/${fixture().slug}/docs$`));
	await expect(page.getByRole("link", { name: parent })).toHaveCount(0);
});

test("a save against a doc somebody else changed shows both versions", async ({ page }) => {
	await page.goto(at("/docs"));

	const doc = await raiseDoc(page, `Conflict ${Date.now()}`);

	await page.goto(at(`/docs/${doc.id}/edit`));
	await expect(page.getByRole("textbox", { name: "Doc text" })).toBeVisible();

	await page.evaluate(
		async ([workspaceId, docId, version]) => {
			await fetch(`/v1/workspaces/${workspaceId}/docs/${docId}`, {
				method: "PATCH",
				headers: { "content-type": "application/json" },
				body: JSON.stringify({ expectedVersion: version, markdown: "Somebody else's words." }),
			});
		},
		[fixture().workspaceId, doc.id, doc.version] as const
	);

	await page.getByRole("textbox", { name: "Doc text" }).click();
	await page.keyboard.type(" My words.");

	await expect(page.getByText("Somebody else saved first")).toBeVisible();
	await expect(page.getByRole("group", { name: "Conflicting saves" })).toBeVisible();

	await page.getByRole("button", { name: "Keep mine" }).click();
	await expect(page.getByText("Saved", { exact: true })).toBeVisible();
});

test.describe("a member who did not write a doc", () => {
	test("reads it without being offered to change it", async ({ page, browser }) => {
		await page.goto(at("/docs"));

		const doc = await raiseDoc(page, `Shared reading ${Date.now()}`);
		const ordinary = await browser.newContext({ storageState: ordinaryStatePath });
		const reader = await ordinary.newPage();

		await reader.goto(at(`/docs/${doc.id}`));

		await expect(reader.getByText("You can read this doc")).toBeVisible();
		await expect(reader.getByRole("link", { name: "Edit" })).toHaveCount(0);

		await reader.goto(at(`/docs/${doc.id}/edit`));
		await expect(reader).toHaveURL(new RegExp(`/docs/${doc.id}$`));

		await ordinary.close();
	});
});
