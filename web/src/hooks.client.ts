import * as Sentry from "@sentry/sveltekit";
import type { HandleClientError } from "@sveltejs/kit";

Sentry.init({
	dsn: "https://bcd796095f6fab40204c63f2931ef8c9@events.hexmere.com/5",
	tracesSampleRate: 1,
	replaysSessionSampleRate: 0.1,
	replaysOnErrorSampleRate: 1,
	integrations: [Sentry.replayIntegration()],
	enableLogs: true,
	dataCollection: {
		userInfo: false,
		cookies: false,
		httpBodies: [],
		urlQueryParams: { deny: ["token", "code", "state"] },
		stackFrameVariables: false,
	},
});

export const handleError: HandleClientError = Sentry.handleErrorWithSentry(
	({ error, status, message }) => {
		if (status === 404) return { message, code: "not_found" };

		console.error(error);

		return { message: "Something went wrong.", code: "unexpected" };
	},
);
