import { toast } from "svelte-sonner";
import Toast from "$lib/components/norn/toast.svelte";

export type Raised = {
	href?: string;
	action?: string;
	onaction?: () => void;
};

let showing: number | string | undefined;

export function showToast(message: string, options: Raised = {}) {
	if (showing !== undefined) toast.dismiss(showing);

	const id = toast.custom(Toast, {
		unstyled: true,
		componentProps: {
			message,
			href: options.href,
			action: options.action,
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
