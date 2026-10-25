import * as Sentry from '@sentry/sveltekit';
import { dataCollection, sampleRate } from '#lib/telemetry.js';

const dsn = process.env.PUBLIC_SENTRY_DSN?.trim();

if (dsn) {
	Sentry.init({
		dsn,
		tracesSampleRate: sampleRate(process.env.PUBLIC_SENTRY_TRACES_SAMPLE_RATE),
		enableLogs: true,
		dataCollection: dataCollection()
	});
}
