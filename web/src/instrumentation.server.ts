import * as Sentry from '@sentry/sveltekit';

Sentry.init({
	dsn: 'https://bcd796095f6fab40204c63f2931ef8c9@events.hexmere.com/5',
	tracesSampleRate: 1.0,
	enableLogs: true,
	dataCollection: {
		userInfo: false,
		cookies: false,
		httpBodies: [],
		urlQueryParams: { deny: ['token', 'code', 'state'] },
		stackFrameVariables: false
	}
});
