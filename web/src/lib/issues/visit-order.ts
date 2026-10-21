const storageKey = "norn.visit-order";

type VisitOrder = { list: string; references: string[] };

let remembered: VisitOrder | null = null;

function stored(): VisitOrder | null {
	try {
		const raw = sessionStorage.getItem(storageKey);

		if (!raw) return null;

		const parsed: unknown = JSON.parse(raw);

		if (
			typeof parsed === "object" &&
			parsed !== null &&
			"list" in parsed &&
			"references" in parsed &&
			typeof parsed.list === "string" &&
			Array.isArray(parsed.references) &&
			parsed.references.every((reference) => typeof reference === "string")
		) {
			return { list: parsed.list, references: parsed.references };
		}
	} catch {
		return null;
	}

	return null;
}

export function rememberOrder(list: string, references: string[]) {
	remembered = { list, references };

	try {
		sessionStorage.setItem(storageKey, JSON.stringify(remembered));
	} catch {
		return;
	}
}

export function orderFor(list: string): string[] | null {
	remembered ??= stored();

	return remembered?.list === list ? remembered.references : null;
}

export type Neighbours = {
	position: number;
	total: number;
	previous: string | null;
	next: string | null;
};

export function neighboursIn(references: string[] | null, reference: string): Neighbours | null {
	if (!references) return null;

	const position = references.indexOf(reference);

	if (position < 0) return null;

	return {
		position,
		total: references.length,
		previous: position > 0 ? references[position - 1] : null,
		next: position < references.length - 1 ? references[position + 1] : null,
	};
}
