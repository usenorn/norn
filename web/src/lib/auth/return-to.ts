import type { ReadonlyURL } from "$app/state";

export function safeReturn(requested: string | null): string {
	return requested && requested.startsWith("/") && !requested.startsWith("//") ? requested : "/";
}

// Adding a second account can take several screens — sign in, create an account, single sign-on —
// and each hop has to keep saying that this is an addition and where the person came from, or the
// next screen bounces them back to the account they already had.
export function authPath(from: ReadonlyURL, path: string): string {
	const adding = from.searchParams.get("add");
	const back = from.searchParams.get("return");

	if (!adding && !back) return path;

	const [base, query] = path.split("?");
	const params = new URLSearchParams(query);

	if (adding) params.set("add", adding);
	if (back) params.set("return", back);

	return `${base}?${params}`;
}

export function carriedURL(url: URL, body: FormData): URL {
	const carried = new URL(url.pathname, url);

	for (const key of ["add", "return"]) {
		const value = body.get(key);
		if (typeof value === "string" && value) carried.searchParams.set(key, value);
	}

	return carried;
}

export function actionPath(from: ReadonlyURL, name: string): string {
	const kept = [...from.searchParams].filter(([key]) => !key.startsWith("/"));
	const query = new URLSearchParams(kept).toString();

	return query ? `?/${name}&${query}` : `?/${name}`;
}
