export function safeReturn(requested: string | null): string {
	return requested && requested.startsWith("/") && !requested.startsWith("//") ? requested : "/";
}
