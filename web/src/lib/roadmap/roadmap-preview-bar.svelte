<script lang="ts">
	import ArrowLeft from "@lucide/svelte/icons/arrow-left";
	import EyeOff from "@lucide/svelte/icons/eye-off";
	import Send from "@lucide/svelte/icons/send";
	import Pill from "$lib/components/norn/pill.svelte";
	import { Button } from "$lib/components/ui/button/index.js";
	import { roadmapReviewPath, roadmapSettingsPath } from "./admin";

	let { workspace }: { workspace: string } = $props();
</script>

<nav
	aria-label="Draft preview"
	class="sticky top-0 z-20 flex items-center gap-2 border-b border-line-default bg-background px-4 py-2 sm:gap-3 sm:px-6"
>
	<Button size="sm" variant="ghost" href={roadmapSettingsPath(workspace)} aria-label="Back to roadmap settings">
		<ArrowLeft aria-hidden="true" />
		<span class="hidden sm:inline">Roadmap settings</span>
	</Button>

	<Pill tone="warning" class="min-w-0">
		<EyeOff aria-hidden="true" />
		<span class="truncate">Draft preview · not public</span>
	</Pill>

	<Button size="sm" href={roadmapReviewPath(workspace)} class="ml-auto">
		<Send aria-hidden="true" />
		<span class="sm:hidden">Review</span>
		<span class="hidden sm:inline">Review and publish</span>
	</Button>
</nav>
