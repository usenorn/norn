import { toast } from "svelte-sonner";
import Toast, { type ToastTone } from "$lib/components/norn/toast.svelte";

export type Raised = {
	href?: string;
	action?: string;
	tone?: ToastTone;
	duration?: number;
	onaction?: () => void;
};

const failureFor = 8000;

let showing: number | string | undefined;

export function showToast(message: string, options: Raised = {}) {
	if (showing !== undefined) toast.dismiss(showing);

	const id = toast.custom(Toast, {
		unstyled: true,
		duration: options.duration,
		componentProps: {
			message,
			href: options.href,
			action: options.action,
			tone: options.tone,
			onaction:
				options.onaction &&
				(() => {
					toast.dismiss(id);
					options.onaction?.();
				}),
			onnavigate: () => toast.dismiss(id),
		},
		onDismiss: () => (showing = undefined),
		onAutoClose: () => (showing = undefined),
	});

	showing = id;

	return id;
}

export function showFailure(message: string, options: Omit<Raised, "tone"> = {}) {
	return showToast(message, { ...options, tone: "failure", duration: failureFor });
}
