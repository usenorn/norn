import * as Sentry from '@sentry/sveltekit';
import { env } from '$env/dynamic/public';
import { dataCollection, sampleRate } from '$lib/telemetry';

const dsn = env.PUBLIC_SENTRY_DSN?.trim();

if (dsn) {
	Sentry.init({
		dsn,
		tracesSampleRate: sampleRate(env.PUBLIC_SENTRY_TRACES_SAMPLE_RATE),
		enableLogs: true,
		dataCollection: dataCollection()
	});
}
