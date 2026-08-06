import type { Handle, HandleServerError } from "@sveltejs/kit";
import { apiForEvent, correlationHeader } from "$lib/api/server";

export const handle: Handle = ({ event, resolve }) => {
	event.locals.correlationId =
		event.request.headers.get(correlationHeader) ?? crypto.randomUUID();
	event.locals.api = apiForEvent(event);

	return resolve(event);
};

export const handleError: HandleServerError = ({ error, event, status, message }) => {
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
