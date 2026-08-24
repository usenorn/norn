<script lang="ts">
	import { cn } from "$lib/utils.js";
	import { folderLabels, folderTones, type CodebaseState } from "./runners";

	let { state, class: className }: { state: CodebaseState; class?: string } = $props();

	const tone = {
		waiting: "text-status-not-started",
		working: "text-status-complete",
		attention: "text-amber-700 dark:text-amber-400",
		bad: "text-danger",
	};

	const pip = {
		waiting: "bg-status-not-started",
		working: "bg-status-complete",
		attention: "bg-amber-700 dark:bg-amber-400",
		bad: "bg-danger",
	};
</script>

<span
	class={cn("inline-flex min-w-0 items-center gap-1.5 text-sm", tone[folderTones[state]], className)}
>
	<span aria-hidden="true" class={cn("size-1.5 flex-none rounded-full", pip[folderTones[state]])}
	></span>
	<span class="truncate">{folderLabels[state]}</span>
</span>
