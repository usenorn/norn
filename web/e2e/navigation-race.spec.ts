import { expect, test } from "@playwright/test";
import { at, fixture } from "./fixture";

const origin = `http://localhost:${process.env.NORN_PREVIEW_PORT ?? 4173}`;

const refetchWindowMs = 400;

test("a realtime update arriving mid-navigation does not throw the reader back to the list", async ({
	page,
}) => {
	const issue = fixture().issues[0];

	let release = () => {};
	const held = new Promise<void>((settle) => (release = settle));
	let started = () => {};
	const navigationStarted = new Promise<void>((settle) => (started = settle));

	await page.route("**/issues/*/__data.json*", async (route) => {
		started();
		await held;
		await route.continue();
	});

	const read = await page.request.get(
		`/v1/workspaces/${fixture().workspaceId}/issues/${issue.id}`
	);

	expect(read.ok(), await read.text()).toBe(true);

	const { version } = (await read.json()) as { version: number };

	const streamOpened = page.waitForRequest((request) => request.url().includes("/events?"));

	await page.goto(at("/my-tasks"));
	await expect(page.getByRole("link", { name: new RegExp(issue.reference) }).first()).toBeVisible();

	const stream = (await streamOpened).url();

	const updated = page.evaluate(
		(url) =>
			new Promise<boolean>((settle) => {
				const source = new EventSource(url);

				source.addEventListener("issue.updated", () => {
					source.close();
					settle(true);
				});
				source.onerror = () => {
					source.close();
					settle(false);
				};
			}),
		stream
	);

	await page.getByRole("link", { name: new RegExp(issue.reference) }).first().click();
	await navigationStarted;

	const patched = await page.request.patch(
		`/v1/workspaces/${fixture().workspaceId}/issues/${issue.id}`,
		{
			headers: { origin, referer: `${origin}/` },
			data: { expectedVersion: version, title: `${issue.title} (touched ${Date.now()})` },
		}
	);

	expect(patched.ok(), await patched.text()).toBe(true);
	expect(await updated).toBe(true);

	const deferred = page.waitForRequest(
		(request) =>
			request.url().includes(`/issues/${issue.reference}/__data.json`) &&
			new URL(request.url()).searchParams.has("x-sveltekit-invalidated")
	);

	await page.waitForTimeout(refetchWindowMs);
	release();

	await expect(page).toHaveURL(new RegExp(`/issues/${issue.reference}(\\?|$)`));
	await expect(page.getByRole("main")).toContainText(issue.title);
	await deferred;
});
