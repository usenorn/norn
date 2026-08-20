<script lang="ts">
	import './layout.css';
	import Toaster from '$lib/toast/toaster.svelte';
	import { provideToasts } from '$lib/toast/toasts.svelte';

	let { children } = $props();

	provideToasts();
</script>

{@render children()}

<Toaster />
