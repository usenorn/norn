import createClient, { type Client } from "openapi-fetch";
import { parseSetCookie } from "set-cookie-parser";
import { env } from "$env/dynamic/private";
import type { Cookies, RequestEvent } from "@sveltejs/kit";
import { sessionHeader } from "$lib/account/accounts";
import type { paths } from "./dashboard.gen";

export const correlationHeader = "x-correlation-id";

const sessionCookiePrefix = "norn_session_";
const hostCookiePrefix = "__Host-";

const loopbackOrigin = "http://127.0.0.1:8080";

export function internalOrigin(): string {
	return env.INTERNAL_API_ORIGIN || loopbackOrigin;
}

const verbatim = { decode: (value: string) => value };

export function hasSession(cookies: Cookies): boolean {
	return cookies
		.getAll(verbatim)
		.some(
			({ name }) =>
				name.startsWith(sessionCookiePrefix) ||
				name.startsWith(hostCookiePrefix + sessionCookiePrefix)
		);
}

function cookieHeader(cookies: Cookies): string {
	return cookies
		.getAll(verbatim)
		.map(({ name, value }) => `${name}=${value}`)
		.join("; ");
}

function sameSite(value: string | undefined): "lax" | "strict" | "none" | undefined {
	switch (value?.toLowerCase()) {
		case "lax":
			return "lax";
		case "strict":
			return "strict";
		case "none":
			return "none";
		default:
			return undefined;
	}
}

function relaySetCookie(cookies: Cookies, response: Response) {
	for (const cookie of parseSetCookie(response, { decodeValues: false })) {
		cookies.set(cookie.name, cookie.value, {
			path: cookie.path ?? "/",
			domain: cookie.domain,
			expires: cookie.expires,
			maxAge: cookie.maxAge,
			httpOnly: cookie.httpOnly,
			secure: cookie.secure,
			sameSite: sameSite(cookie.sameSite),
			encode: (value) => value,
		});
	}
}

function unreachable(correlationId: string): Response {
	const problem = {
		type: "about:blank",
		title: "Service Unavailable",
		status: 503,
		detail: "the api could not be reached",
		instance: correlationId,
	};

	return new Response(JSON.stringify(problem), {
		status: 503,
		headers: { "content-type": "application/problem+json" },
	});
}

// adapter-node throws when ADDRESS_HEADER is configured but the header is missing, which is
// every request that did not come through the proxy — kubelet probes above all. Such a request
// has no client to attribute, so it forwards no chain and the Go side falls back to the peer.
function clientAddress(event: RequestEvent): string | undefined {
	try {
		return event.getClientAddress();
	} catch {
		return undefined;
	}
}

export function apiForEvent(event: RequestEvent, acting?: Promise<string | null>): Client<paths> {
	return createClient<paths>({
		baseUrl: `${internalOrigin()}/v1`,
		fetch: async (request) => {
			const address = clientAddress(event);
			const slot = acting ? await acting : null;

			if (slot) {
				request.headers.set(sessionHeader, slot);
			}

			request.headers.set("cookie", cookieHeader(event.cookies));
			request.headers.set("origin", event.url.origin);
			if (address) {
				request.headers.set("x-forwarded-for", address);
			}
			request.headers.set("x-forwarded-proto", event.url.protocol.slice(0, -1));
			request.headers.set("x-forwarded-host", event.url.host);
			request.headers.set("user-agent", event.request.headers.get("user-agent") ?? "");
			request.headers.set(correlationHeader, event.locals.correlationId);

			let response: Response;

			try {
				response = await fetch(request);
			} catch {
				return unreachable(event.locals.correlationId);
			}

			relaySetCookie(event.cookies, response);

			return response;
		},
	});
}
