import * as Sentry from "@sentry/sveltekit";
import { env } from "$env/dynamic/public";
import type { HandleClientError } from "@sveltejs/kit";
import { dataCollection, recordsSessions, reportingFrom } from "$lib/telemetry";

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

const report: HandleClientError = ({ error, status, message }) => {
	if (status === 404) return { message, code: "not_found" };

	console.error(error);

	return { message: "Something went wrong.", code: "unexpected" };
};

export const handleError: HandleClientError = reporting
	? Sentry.handleErrorWithSentry(report)
	: report;
