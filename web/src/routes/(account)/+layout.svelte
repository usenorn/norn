<script lang="ts">
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import { workspacePath } from "$lib/workspace/navigation";
	import type { LayoutProps } from "./$types";

	let { data, children }: LayoutProps = $props();

	const back = $derived(
		data.workspaces.length > 0 ? workspacePath(data.workspaces[0].slug, "/settings") : "/"
	);
</script>

<div class="flex min-h-dvh flex-col bg-paper-50">
	<header class="flex h-11 flex-none items-center gap-3 border-b border-line-subtle px-4">
		<a
			href={back}
			class="flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-ink-900"
		>
			<ArrowLeft class="size-4" aria-hidden="true" />
			<span>Back to {data.workspaces[0]?.name ?? "Norn"}</span>
		</a>
	</header>

	<main class="flex min-h-0 flex-1 flex-col">
		{@render children()}
	</main>
</div>
