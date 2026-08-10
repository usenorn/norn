export function safeReturn(requested: string | null): string {
	return requested && requested.startsWith("/") && !requested.startsWith("//") ? requested : "/";
}

// Adding a second account can take several screens — sign in, create an account, single sign-on —
// and each hop has to keep saying that this is an addition and where the person came from, or the
// next screen bounces them back to the account they already had.
export function authPath(from: URL, path: string): string {
	const adding = from.searchParams.get("add");
	const back = from.searchParams.get("return");

	if (!adding && !back) return path;

	const [base, query] = path.split("?");
	const params = new URLSearchParams(query);

	if (adding) params.set("add", adding);
	if (back) params.set("return", back);

	return `${base}?${params}`;
}
