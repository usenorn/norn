import type { HandleClientError } from "@sveltejs/kit/hooks";
import * as Sentry from "@sentry/sveltekit";
import * as env from "$app/env/public";
import { startEarlyKeys } from "#lib/shortcuts/early-keys.js";
import { dataCollection, recordsSessions, reportingFrom } from "#lib/telemetry.js";

startEarlyKeys();

const reporting = reportingFrom(env);

if (reporting) {
	Sentry.init({
		dsn: reporting.dsn,
		tracesSampleRate: reporting.tracesSampleRate,
		replaysSessionSampleRate: reporting.replaysSessionSampleRate,
		replaysOnErrorSampleRate: reporting.replaysOnErrorSampleRate,
		integrations: recordsSessions(reporting) ? [Sentry.replayIntegration()] : [],
		enableLogs: true,
		dataCollection: dataCollection(),
	});
}

const report: HandleClientError = (caught) => {
	if (caught.kind === "framework" && caught.error.status === 404) return { code: "not_found" };
	if (caught.kind !== "unknown") return;

	console.error(caught.error);

	return { message: "Something went wrong.", code: "unexpected" };
};

export const handleError: HandleClientError = reporting
	? Sentry.handleErrorWithSentry(report)
	: report;
