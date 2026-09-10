import { showFailure, showToast } from "$lib/toast/toasts";

export const copyRefusedLine = "Your browser would not let us copy that";

export async function copyText(text: string, said: string): Promise<boolean> {
	try {
		await navigator.clipboard.writeText(text);
		showToast(said);

		return true;
	} catch {
		showFailure(copyRefusedLine);

		return false;
	}
}
