import * as Sentry from "@sentry/sveltekit";
import * as env from "$app/env/public";
import { sequence, type Handle, type HandleServerError } from "@sveltejs/kit/hooks";
import { actingSlot, signedInAccounts } from "#lib/api/acting.server.js";
import { apiForEvent, correlationHeader } from "#lib/api/server.js";
import { isPublicRoadmapRoute, onRoadmapHost } from "#lib/roadmap/host.js";
import { reportingFrom } from "#lib/telemetry.js";

const reporting = reportingFrom(env);

const publicRoadmapHeaders = { "cache-control": "no-store", "x-robots-tag": "noindex" };

const scope: Handle = async ({ event, resolve }) => {
	event.locals.correlationId =
		event.request.headers.get(correlationHeader) ?? crypto.randomUUID();

	if (
		onRoadmapHost(event.url.hostname, env.PUBLIC_ROADMAPS_DOMAIN) ||
		isPublicRoadmapRoute(event.route.id)
	) {
		event.locals.signedIn = Promise.resolve([]);
		event.locals.acting = Promise.resolve(null);
		event.locals.api = apiForEvent(event, event.locals.acting, "omit");
		event.locals.apiAs = () => event.locals.api;

		const response = await resolve(event);

		for (const [name, value] of Object.entries(publicRoadmapHeaders)) {
			response.headers.set(name, value);
		}

		return response;
	}

	event.locals.signedIn = signedInAccounts(event);
	event.locals.acting = actingSlot(event);
	event.locals.api = apiForEvent(event, event.locals.acting);
	event.locals.apiAs = (slot: string) => apiForEvent(event, Promise.resolve(slot));

	return resolve(event);
};

export const handle: Handle = reporting ? sequence(Sentry.sentryHandle(), scope) : scope;

const report: HandleServerError = (caught) => {
	if (caught.kind === "framework" && caught.error.status === 404) return { code: "not_found" };
	if (caught.kind !== "unknown") return;

	const { error, event } = caught;

	console.error({
		correlationId: event.locals.correlationId,
		route: event.route.id,
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
