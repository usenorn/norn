import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.NORN_PREVIEW_PORT ?? 4173);

export default defineConfig({
	testDir: "e2e",
	globalSetup: "./e2e/setup.ts",
	fullyParallel: false,
	forbidOnly: Boolean(process.env.CI),
	retries: process.env.CI ? 1 : 0,
	workers: 1,
	reporter: process.env.CI ? [["line"], ["html", { open: "never" }]] : "list",
	timeout: 45_000,
	expect: { timeout: 10_000 },
	use: {
		baseURL: `http://localhost:${port}`,
		storageState: "e2e/.state/session.json",
		trace: "retain-on-failure",
	},
	projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
	webServer: {
		command: "pnpm preview",
		url: `http://localhost:${port}/sign-in`,
		reuseExistingServer: !process.env.CI,
		timeout: 120_000,
	},
});
