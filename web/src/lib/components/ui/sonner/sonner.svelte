<script lang="ts">
	import { Toaster as Sonner, type ToasterProps as SonnerProps } from "svelte-sonner";

	let { ...restProps }: SonnerProps = $props();
</script>

<Sonner
	class="toaster group"
	mobileOffset={{ bottom: "calc(76px + env(safe-area-inset-bottom))", left: "16px", right: "16px" }}
	toastOptions={{ unstyled: true, classes: { toast: "w-full" } }}
	{...restProps}
/>
