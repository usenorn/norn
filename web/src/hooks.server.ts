import * as Sentry from "@sentry/sveltekit";
import { env } from "$env/dynamic/public";
import type { Handle, HandleServerError } from "@sveltejs/kit";
import { sequence } from "@sveltejs/kit/hooks";
import { actingSlot, signedInAccounts } from "$lib/api/acting.server";
import { apiForEvent, correlationHeader } from "$lib/api/server";
import { reportingFrom } from "$lib/telemetry";

const reporting = reportingFrom(env);

const scope: Handle = ({ event, resolve }) => {
	event.locals.correlationId =
		event.request.headers.get(correlationHeader) ?? crypto.randomUUID();

	event.locals.signedIn = signedInAccounts(event);
	event.locals.acting = actingSlot(event);
	event.locals.api = apiForEvent(event, event.locals.acting);
	event.locals.apiAs = (slot: string) => apiForEvent(event, Promise.resolve(slot));

	return resolve(event);
};

export const handle: Handle = reporting ? sequence(Sentry.sentryHandle(), scope) : scope;

const report: HandleServerError = ({ error, event, status, message }) => {
	if (status === 404) return { message, code: "not_found" };

	console.error({
		correlationId: event.locals.correlationId,
		route: event.route.id,
		status,
		error,
	});

	return {
		message: "Something went wrong on our side.",
		code: "unexpected",
		reference: event.locals.correlationId,
	};
};

export const handleError: HandleServerError = reporting
	? Sentry.handleErrorWithSentry(report)
	: report;
