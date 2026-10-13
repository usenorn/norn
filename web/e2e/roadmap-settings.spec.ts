import {
	expect,
	request,
	test,
	type APIRequestContext,
	type APIResponse,
	type Browser,
	type Page,
} from "@playwright/test";
import type { components } from "../src/lib/api/dashboard.gen";
import { at, fixture, ordinaryStatePath, statePath } from "./fixture";

type Roadmap = components["schemas"]["Roadmap"];
type RoadmapVisibility = components["schemas"]["RoadmapVisibility"];
type Team = components["schemas"]["Team"];
type Project = components["schemas"]["Project"];
type Issue = components["schemas"]["Issue"];

const api = process.env.NORN_E2E_API ?? "http://localhost:8080";
const port = process.env.NORN_PREVIEW_PORT ?? "4173";
const appOrigin = `http://localhost:${port}`;
const roadmapsDomain = process.env.NORN_ROADMAPS_DOMAIN ?? "roadmap.localhost";

const roadmapOrigin = () => `http://${fixture().slug}.${roadmapsDomain}:${port}`;

const secrets = {
	projectName: "SECRET-SETTINGS-PROJECT-3c9e1a",
	projectDescription: "SECRET-SETTINGS-PROJECT-DESCRIPTION-8d02f7",
	hiddenProjectName: "SECRET-SETTINGS-UNSHARED-PROJECT-61b4ce",
	walletTitle: "SECRET-SETTINGS-WALLET-ISSUE-a73d09",
	walletDescription: "SECRET-SETTINGS-WALLET-DESCRIPTION-0e5b21",
	themeTitle: "SECRET-SETTINGS-THEME-ISSUE-f4c816",
	themeDescription: "SECRET-SETTINGS-THEME-DESCRIPTION-29a7d3",
};

const copy = {
	roadmapTitle: "Northwind product plan",
	roadmapDescription: "What Northwind customers can expect next.",
	projectTitle: "Faster checkout",
	projectRetitled: "Checkout in two taps",
	projectDescription: "Pay with a saved card in two taps.",
	walletTitle: "Wallet payments",
	walletRetitled: "Wallet payments everywhere",
	walletDescription: "Apple Pay and Google Pay on every device.",
	themeTitle: "Dark mode",
	themeDescription: "Follows the system setting.",
	typedDuringConflict: "Typed while someone else saved.",
};

const nothingShown: RoadmapVisibility = {
	issueLists: false,
	projectDescriptions: false,
	issueDescriptions: false,
	status: false,
	timeline: false,
	progress: false,
};

function send(
	client: APIRequestContext,
	method: "GET" | "POST" | "PATCH" | "DELETE",
	path: string,
	body?: unknown
): Promise<APIResponse> {
	return client.fetch(`${api}/v1${path}`, {
		method,
		headers: { origin: appOrigin, referer: `${appOrigin}/`, "content-type": "application/json" },
		...(body === undefined ? {} : { data: body }),
	});
}

async function read<T>(answer: APIResponse, status = 200): Promise<T> {
	expect(answer.status(), await answer.text()).toBe(status);

	return (await answer.json()) as T;
}

function expectNoSecrets(where: string, body: string) {
	for (const secret of Object.values(secrets)) {
		expect(body, `${where} leaked ${secret}`).not.toContain(secret);
	}
}

function captureResponses(page: Page): () => Promise<string> {
	const bodies: Promise<string>[] = [];

	page.on("response", (response) => {
		bodies.push(
			response
				.text()
				.catch(() => "")
				.then((text) => `${response.url()} ${text}`)
		);
	});

	return async () => (await Promise.all(bodies)).join("\n");
}

function teamKey(): string {
	return `S${Array.from(String(Date.now()).slice(-4), (digit) => "ABCDEFGHIJ"[Number(digit)]).join("")}`;
}

async function horizontalOverflow(page: Page): Promise<number> {
	return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
}

function publication(page: Page) {
	return page.getByRole("region", { name: "Publication" });
}

function entryRow(page: Page, title: string) {
	return page
		.getByRole("listitem")
		.filter({ has: page.getByRole("button", { name: `More for ${title}`, exact: true }) })
		.last();
}

type PublicReader = { page: Page; close: () => Promise<void>; bodies: () => Promise<string> };

async function readPublicPage(browser: Browser): Promise<PublicReader> {
	const context = await browser.newContext({ storageState: { cookies: [], origins: [] } });
	const page = await context.newPage();
	const bodies = captureResponses(page);

	await page.goto(`${roadmapOrigin()}/`);

	return { page, bodies, close: () => context.close() };
}

test.describe.configure({ mode: "serial" });

test.describe("the public roadmap settings", () => {
	test.use({ permissions: ["clipboard-read", "clipboard-write"] });

	const workspace = () => `/workspaces/${fixture().workspaceId}`;

	let admin: APIRequestContext;
	let member: APIRequestContext;
	let wallet: Issue;
	let theme: Issue;

	async function roadmap(): Promise<Roadmap> {
		return read<Roadmap>(await send(admin, "GET", `${workspace()}/roadmap`));
	}

	async function changeElsewhere(change: Partial<Pick<Roadmap, "title" | "description">>) {
		const current = await roadmap();

		await read(
			await send(admin, "PATCH", `${workspace()}/roadmap`, {
				expectedVersion: current.version,
				title: current.title,
				description: current.description,
				visibility: current.visibility,
				...change,
			})
		);
	}

	async function clear() {
		for (const entry of (await roadmap()).entries) {
			await read(
				await send(
					admin,
					"DELETE",
					`${workspace()}/roadmap/entries/${entry.id}?expectedVersion=${(await roadmap()).version}`
				)
			);
		}

		await read(await send(admin, "POST", `${workspace()}/roadmap/unpublish`));

		const current = await roadmap();

		await read(
			await send(admin, "PATCH", `${workspace()}/roadmap`, {
				expectedVersion: current.version,
				title: "",
				description: "",
				visibility: nothingShown,
			})
		);
	}

	async function addSource(
		page: Page,
		pick: { button: string; search: string; query: string; option: string | RegExp },
		write: { title: string; description: string; descriptions: string; showDescription: boolean }
	) {
		await page.getByRole("button", { name: pick.button }).click();
		await page.getByPlaceholder(pick.search).fill(pick.query);
		await page.getByRole("option", { name: pick.option }).click();

		const editor = page.getByRole("dialog");
		const copyTitle = editor.getByRole("button", { name: "Copy into public title" });
		const title = editor.getByRole("textbox", { name: "Public title" });
		const descriptions = editor.getByRole("switch", { name: new RegExp(`^${write.descriptions}`) });

		await expect(title).toHaveValue("");
		await expect(editor.getByRole("textbox", { name: "Public description" })).toHaveValue("");
		await expect(descriptions).not.toBeChecked();
		await expect(copyTitle).toBeEnabled();
		await copyTitle.click();

		await expect(title).toHaveValue(typeof pick.option === "string" ? pick.option : /SECRET-/);
		await title.fill(write.title);
		await editor.getByRole("textbox", { name: "Public description" }).fill(write.description);

		if (write.showDescription) {
			await descriptions.click();
			await expect(descriptions).toBeChecked();
		}

		await editor.getByRole("button", { name: "Add to roadmap" }).click();
		await expect(editor).toBeHidden();
		await expect(entryRow(page, write.title)).toContainText("Not yet published");
	}

	async function reviewAndPublish(page: Page, publishLabel: "Publish" | "Publish changes") {
		await publication(page).getByRole("button", { name: "Review and publish" }).click();

		const review = page.getByRole("dialog", { name: "Review and publish" });

		await expect(review.getByText("Anyone can read and copy what you publish")).toBeVisible();

		return {
			review,
			publish: async () => {
				await review.getByRole("button", { name: publishLabel, exact: true }).click();
				await expect(review).toBeHidden();
				await expect(publication(page)).toContainText("Published");
				await expect(publication(page)).not.toContainText("Unpublished changes");
			},
		};
	}

	test.beforeAll(async () => {
		admin = await request.newContext({ storageState: statePath });
		member = await request.newContext({ storageState: ordinaryStatePath });

		await clear();

		const key = teamKey();
		const team = await read<Team>(
			await send(admin, "POST", `${workspace()}/teams`, {
				key,
				name: `Settings ${key}`,
				visibility: "public",
			}),
			201
		);

		await read<Project>(
			await send(admin, "POST", `${workspace()}/projects`, {
				slug: `settings-${key.toLowerCase()}`,
				name: secrets.projectName,
				description: secrets.projectDescription,
				teamIds: [team.id],
			}),
			201
		);

		const unshared = await read<Project>(
			await send(admin, "POST", `${workspace()}/projects`, {
				slug: `unshared-${key.toLowerCase()}`,
				name: secrets.hiddenProjectName,
				teamIds: [team.id],
			}),
			201
		);

		wallet = await read<Issue>(
			await send(admin, "POST", `${workspace()}/issues`, {
				teamId: team.id,
				projectId: unshared.id,
				title: secrets.walletTitle,
				description: secrets.walletDescription,
			}),
			201
		);

		theme = await read<Issue>(
			await send(admin, "POST", `${workspace()}/issues`, {
				teamId: team.id,
				projectId: unshared.id,
				title: secrets.themeTitle,
				description: secrets.themeDescription,
			}),
			201
		);
	});

	test.afterAll(async () => {
		await clear();
		await admin.dispose();
		await member.dispose();
	});

	test("an administrator reaches the roadmap from the settings navigation", async ({ page }) => {
		await page.goto(at("/my-tasks"));
		await page.getByRole("link", { name: "Settings", exact: true }).click();
		await page.getByRole("navigation", { name: "Settings" }).getByRole("link", { name: "Public roadmap" }).click();

		await expect(page).toHaveURL(new RegExp(`/${fixture().slug}/settings/roadmap$`));
		await expect(page.getByRole("heading", { level: 1, name: "Public roadmap" })).toBeVisible();
		await expect(publication(page)).toContainText("Not published");
		await expect(publication(page)).toContainText("Nothing is public.");
		await expect(page.getByText("Nothing selected yet")).toBeVisible();
	});

	test("a projects-only roadmap publishes the edited public copy and nothing internal", async ({
		page,
		browser,
	}) => {
		await page.goto(at("/settings/roadmap"));

		await page.getByRole("textbox", { name: "Public title" }).fill(copy.roadmapTitle);
		await page.getByRole("textbox", { name: "Public description" }).fill(copy.roadmapDescription);

		for (const flag of ["Project descriptions", "Issue descriptions", "Status"]) {
			await page.getByRole("switch", { name: flag, exact: true }).click();
		}

		await page.getByRole("button", { name: "Save roadmap page" }).click();
		await expect(page.getByText("Saved", { exact: true })).toBeVisible();

		const saved = await roadmap();

		expect(saved.title).toBe(copy.roadmapTitle);
		expect(saved.visibility).toEqual({
			...nothingShown,
			projectDescriptions: true,
			issueDescriptions: true,
			status: true,
		});

		await page.getByRole("radio", { name: /^Projects only/ }).click();
		await expect(page.getByRole("button", { name: "Add issue" })).toBeHidden();

		await addSource(
			page,
			{ button: "Add project", search: "Find a project", query: secrets.projectName, option: secrets.projectName },
			{
				title: copy.projectTitle,
				description: copy.projectDescription,
				descriptions: "Project descriptions",
				showDescription: false,
			}
		);

		await expect(publication(page)).toContainText("Not published");

		const { review, publish } = await reviewAndPublish(page, "Publish");

		await expect(review.getByRole("heading", { name: /^Added/ })).toBeVisible();
		await expect(review.getByRole("listitem").filter({ hasText: copy.projectTitle })).toBeVisible();
		await expect(review).not.toContainText(copy.projectDescription);
		expectNoSecrets("the review", (await review.textContent()) ?? "");

		await publish();

		const published = await roadmap();

		expect(published.publicUrl).toBe(roadmapOrigin());
		await expect(publication(page)).toContainText(published.publicUrl ?? "");

		await publication(page).getByRole("button", { name: "Copy public link" }).click();
		await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(published.publicUrl);

		const reader = await readPublicPage(browser);

		await expect(reader.page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();
		await expect(reader.page.getByText(copy.roadmapDescription)).toBeVisible();
		await expect(reader.page.getByRole("link", { name: new RegExp(copy.projectTitle) })).toBeVisible();

		const shown = await reader.page.content();

		expect(shown).not.toContain(copy.projectDescription);
		expectNoSecrets("the public page", shown);
		expectNoSecrets("a public response", await reader.bodies());

		await reader.close();
	});

	test("issues are shared without their project, each with its own description setting", async ({
		page,
		browser,
	}) => {
		await page.goto(at("/settings/roadmap"));

		await page.getByRole("radio", { name: /^Selected issues/ }).click();
		await expect(page.getByText("1 entry does not fit Selected issues")).toBeVisible();
		await expect(page.getByRole("button", { name: "Add project" })).toBeHidden();

		await addSource(
			page,
			{
				button: "Add issue",
				search: "Search issues",
				query: wallet.reference,
				option: new RegExp(secrets.walletTitle),
			},
			{
				title: copy.walletTitle,
				description: copy.walletDescription,
				descriptions: "Issue descriptions",
				showDescription: true,
			}
		);
		await addSource(
			page,
			{
				button: "Add issue",
				search: "Search issues",
				query: theme.reference,
				option: new RegExp(secrets.themeTitle),
			},
			{
				title: copy.themeTitle,
				description: copy.themeDescription,
				descriptions: "Issue descriptions",
				showDescription: false,
			}
		);

		await expect(entryRow(page, copy.projectTitle)).toBeVisible();
		await expect(publication(page)).toContainText("Unpublished changes");

		const { review, publish } = await reviewAndPublish(page, "Publish changes");
		const added = review.getByRole("listitem");

		await expect(added.filter({ hasText: copy.walletTitle })).toContainText(copy.walletDescription);
		await expect(added.filter({ hasText: copy.themeTitle })).toBeVisible();
		await expect(review).not.toContainText(copy.themeDescription);
		expectNoSecrets("the review", (await review.textContent()) ?? "");

		await publish();

		const reader = await readPublicPage(browser);
		const otherWork = reader.page.getByRole("region", { name: "Other planned work" });

		await expect(otherWork.getByRole("link", { name: new RegExp(copy.walletTitle) })).toBeVisible();
		await expect(otherWork.getByRole("link", { name: new RegExp(copy.themeTitle) })).toBeVisible();
		await expect(reader.page.getByText(copy.walletDescription)).toBeVisible();

		const shown = await reader.page.content();

		expect(shown).not.toContain(copy.themeDescription);
		expectNoSecrets("the public page", shown);
		expectNoSecrets("a public response", await reader.bodies());

		await reader.close();
	});

	test("a draft edit stays off the public page until published, and the preview shows it", async ({
		page,
		browser,
	}) => {
		await page.goto(at("/settings/roadmap"));

		await page.getByRole("button", { name: `Edit ${copy.projectTitle}`, exact: true }).click();

		const editor = page.getByRole("dialog", { name: `Edit ${copy.projectTitle}` });

		await editor.getByRole("textbox", { name: "Public title" }).fill(copy.projectRetitled);
		await editor.getByRole("button", { name: "Save changes" }).click();
		await expect(editor).toBeHidden();

		await expect(publication(page)).toContainText("Unpublished changes");

		const before = await readPublicPage(browser);

		await expect(before.page.getByRole("link", { name: new RegExp(copy.projectTitle) })).toBeVisible();
		expect(await before.page.content()).not.toContain(copy.projectRetitled);
		await before.close();

		const responses = captureResponses(page);

		await publication(page).getByRole("link", { name: "Preview draft" }).click();

		await expect(page).toHaveURL(new RegExp(`/${fixture().slug}/settings/roadmap/preview$`));
		await expect(page.getByRole("navigation", { name: "Draft preview" })).toContainText(
			"Draft preview · not public"
		);
		await expect(page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();
		await expect(page.getByText("Draft · not published yet")).toBeVisible();

		const retitled = page.getByRole("link", { name: new RegExp(copy.projectRetitled) });

		await expect(retitled).toBeVisible();
		expect(await page.locator("body").innerText()).not.toContain(copy.projectTitle);
		expectNoSecrets("the draft preview", await page.locator("body").innerText());

		await retitled.click();
		await expect(page.getByRole("heading", { level: 1, name: copy.projectRetitled })).toBeVisible();
		expectNoSecrets("a draft preview entry", await page.locator("body").innerText());

		const previewed = await responses();

		expect(previewed).not.toContain(copy.projectTitle);
		expectNoSecrets("a draft preview response", previewed);

		await page.getByRole("navigation", { name: "Draft preview" }).getByRole("link", { name: /Review/ }).click();

		await expect(page).toHaveURL(new RegExp(`/${fixture().slug}/settings/roadmap\\?review=1$`));

		const review = page.getByRole("dialog", { name: "Review and publish" });
		const changed = review.getByRole("listitem").filter({ hasText: copy.projectRetitled });

		await expect(review.getByRole("heading", { name: /^Changed public text/ })).toBeVisible();
		await expect(changed).toContainText(copy.projectTitle);

		await review.getByRole("button", { name: "Publish changes", exact: true }).click();
		await expect(review).toBeHidden();
		await expect(publication(page)).not.toContainText("Unpublished changes");

		const after = await readPublicPage(browser);

		await expect(after.page.getByRole("link", { name: new RegExp(copy.projectRetitled) })).toBeVisible();
		await after.close();
	});

	test("a conflicting change keeps what was typed until it is saved anyway", async ({ page }) => {
		await page.goto(at("/settings/roadmap"));

		await page.getByRole("button", { name: `Edit ${copy.walletTitle}`, exact: true }).click();

		const editor = page.getByRole("dialog", { name: `Edit ${copy.walletTitle}` });
		const entryTitle = editor.getByRole("textbox", { name: "Public title" });

		await entryTitle.fill(copy.walletRetitled);
		await changeElsewhere({ description: "Changed in another tab." });
		await editor.getByRole("button", { name: "Save changes" }).click();

		await expect(editor.getByText("Someone else changed the roadmap")).toBeVisible();
		await expect(entryTitle).toHaveValue(copy.walletRetitled);

		await editor.getByRole("button", { name: "Save mine anyway" }).click();
		await expect(editor).toBeHidden();
		await expect(entryRow(page, copy.walletRetitled)).toBeVisible();
		expect((await roadmap()).entries.map((entry) => entry.publicTitle)).toContain(copy.walletRetitled);

		const description = page.getByRole("textbox", { name: "Public description" });

		await expect(description).toHaveValue("Changed in another tab.");
		await description.fill(copy.typedDuringConflict);
		await changeElsewhere({ title: `${copy.roadmapTitle}, revised` });
		await page.getByRole("button", { name: "Save roadmap page" }).click();

		const refused = page.getByRole("form", { name: "Roadmap page" });

		await expect(refused.getByText("The roadmap page was not saved")).toBeVisible();
		await expect(description).toHaveValue(copy.typedDuringConflict);

		await refused.getByRole("button", { name: "Save mine anyway" }).click();
		await expect(page.getByText("Saved", { exact: true })).toBeVisible();

		await expect
			.poll(async () => {
				const { title, description: saved } = await roadmap();

				return { title, description: saved };
			})
			.toEqual({ title: `${copy.roadmapTitle}, revised`, description: copy.typedDuringConflict });
	});

	test("the settings and the draft preview fit a 360px phone", async ({ browser }) => {
		const context = await browser.newContext({ storageState: statePath, viewport: { width: 360, height: 740 } });
		const page = await context.newPage();

		await page.goto(at("/settings/roadmap"));
		await expect(publication(page)).toBeVisible();
		await expect(entryRow(page, copy.walletRetitled)).toBeVisible();
		expect(await horizontalOverflow(page), "settings").toBeLessThanOrEqual(0);

		await publication(page).getByRole("button", { name: "Review and publish" }).click();
		await expect(page.getByRole("dialog", { name: "Review and publish" })).toBeVisible();
		expect(await horizontalOverflow(page), "review").toBeLessThanOrEqual(0);
		await page.keyboard.press("Escape");

		await page.goto(at("/settings/roadmap/preview"));
		await expect(page.getByRole("link", { name: new RegExp(copy.projectRetitled) })).toBeVisible();
		expect(await horizontalOverflow(page), "preview").toBeLessThanOrEqual(0);

		await page.getByRole("link", { name: new RegExp(copy.projectRetitled) }).click();
		await expect(page.getByRole("heading", { level: 1, name: copy.projectRetitled })).toBeVisible();
		expect(await horizontalOverflow(page), "preview entry").toBeLessThanOrEqual(0);

		await context.close();
	});

	test("removing an entry and unpublishing take the public page down at once", async ({ page, browser }) => {
		await page.goto(at("/settings/roadmap"));

		await page.getByRole("button", { name: `More for ${copy.themeTitle}`, exact: true }).click();
		await page.getByRole("menuitem", { name: "Remove from roadmap" }).click();

		const removal = page.getByRole("alertdialog", { name: `Remove ${copy.themeTitle}` });

		await expect(removal).toContainText("Leaves the public page now");
		await removal.getByRole("button", { name: "Remove", exact: true }).click();
		await expect(removal).toBeHidden();
		await expect(entryRow(page, copy.themeTitle)).toBeHidden();

		const reader = await readPublicPage(browser);

		await expect(reader.page.getByRole("heading", { level: 1, name: copy.roadmapTitle })).toBeVisible();
		await expect(reader.page.getByRole("link", { name: new RegExp(copy.walletTitle) })).toBeVisible();
		expect(await reader.page.content()).not.toContain(copy.themeTitle);

		await publication(page).getByRole("button", { name: "Unpublish" }).click();

		const unpublish = page.getByRole("alertdialog", { name: "Take the public roadmap down" });

		await unpublish.getByRole("button", { name: "Unpublish", exact: true }).click();
		await expect(unpublish).toBeHidden();
		await expect(publication(page)).toContainText("Not published");

		await reader.page.reload();
		await expect(
			reader.page.getByRole("heading", { level: 1, name: "This roadmap is not available" })
		).toBeVisible();
		expect(await reader.page.content()).not.toContain(copy.walletTitle);

		await reader.close();
	});

	test("an ordinary member is refused the settings, the preview and publishing", async ({ browser }) => {
		const context = await browser.newContext({ storageState: ordinaryStatePath });
		const page = await context.newPage();

		await page.goto(at("/settings/roadmap"));
		await expect(page.getByText("Only administrators manage the public roadmap")).toBeVisible();
		await expect(page.getByRole("button", { name: "Review and publish" })).toHaveCount(0);

		await page.goto(at("/settings/roadmap/preview"));
		await expect(
			page.getByRole("heading", { level: 1, name: "Only administrators preview the public roadmap" })
		).toBeVisible();
		expect(await page.content()).not.toContain(copy.walletRetitled);

		await context.close();

		const { version, review } = await roadmap();

		expect((await send(member, "GET", `${workspace()}/roadmap/preview`)).status()).toBe(403);
		expect(
			(
				await send(member, "POST", `${workspace()}/roadmap/publish`, {
					expectedVersion: version,
					expectedDigest: review.digest,
				})
			).status()
		).toBe(403);
		expect((await send(member, "POST", `${workspace()}/roadmap/unpublish`)).status()).toBe(403);
	});
});
