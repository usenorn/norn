import * as Sentry from '@sentry/sveltekit';
import { PUBLIC_SENTRY_DSN, PUBLIC_SENTRY_TRACES_SAMPLE_RATE } from '$app/env/public';
import { dataCollection, sampleRate } from '#lib/telemetry.js';

const dsn = PUBLIC_SENTRY_DSN?.trim();

if (dsn) {
	Sentry.init({
		dsn,
		tracesSampleRate: sampleRate(PUBLIC_SENTRY_TRACES_SAMPLE_RATE),
		enableLogs: true,
		dataCollection: dataCollection()
	});
}
