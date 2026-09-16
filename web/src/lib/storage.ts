export function readStored(key: string): string | null {
	try {
		return localStorage.getItem(key);
	} catch {
		return null;
	}
}

export function store(key: string, value: string) {
	try {
		localStorage.setItem(key, value);
	} catch {
		return;
	}
}
