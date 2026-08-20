<script lang="ts">
	import './layout.css';
	import { Toaster } from '$lib/components/ui/sonner/index.js';

	let { children } = $props();
</script>

{@render children()}

<Toaster />
