import { defineEnvVars } from "@sveltejs/kit/env";

const optional = (value: string | undefined) => value;

export const variables = defineEnvVars({
	PUBLIC_ROADMAPS_DOMAIN: { public: true, schema: optional },
	PUBLIC_SENTRY_DSN: { public: true, schema: optional },
	PUBLIC_SENTRY_TRACES_SAMPLE_RATE: { public: true, schema: optional },
	PUBLIC_SENTRY_REPLAY_SAMPLE_RATE: { public: true, schema: optional },
	PUBLIC_SENTRY_REPLAY_ON_ERROR_SAMPLE_RATE: { public: true, schema: optional },
	INTERNAL_API_ORIGIN: { schema: optional },
});
