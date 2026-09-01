export type Reporting = {
	dsn: string;
	tracesSampleRate: number;
	replaysSessionSampleRate: number;
	replaysOnErrorSampleRate: number;
};

export function dataCollection() {
	return {
		userInfo: false,
		cookies: false,
		httpBodies: [],
		urlQueryParams: { deny: ["token", "code", "state"] },
		stackFrameVariables: false,
	};
}

export function sampleRate(value: string | undefined): number {
	const rate = Number(value);

	if (!Number.isFinite(rate)) return 0;

	return Math.min(Math.max(rate, 0), 1);
}

export function reportingFrom(source: Record<string, string | undefined>): Reporting | null {
	const dsn = source.PUBLIC_SENTRY_DSN?.trim();

	if (!dsn) return null;

	return {
		dsn,
		tracesSampleRate: sampleRate(source.PUBLIC_SENTRY_TRACES_SAMPLE_RATE),
		replaysSessionSampleRate: sampleRate(source.PUBLIC_SENTRY_REPLAY_SAMPLE_RATE),
		replaysOnErrorSampleRate: sampleRate(source.PUBLIC_SENTRY_REPLAY_ON_ERROR_SAMPLE_RATE),
	};
}

export function recordsSessions(reporting: Reporting): boolean {
	return reporting.replaysSessionSampleRate > 0 || reporting.replaysOnErrorSampleRate > 0;
}
