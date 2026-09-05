<script lang="ts">
	import X from "@lucide/svelte/icons/x";
	import { Button } from "$lib/components/ui/button/index.js";
	import { Progress } from "$lib/components/ui/progress/index.js";
	import { attachmentFailureMessage, formatBytes } from "$lib/attachments/attachments";
	import { settled, type UploadTask } from "$lib/attachments/upload";

	let {
		uploads,
		oncancel,
		onretry,
		ondismiss,
	}: {
		uploads: UploadTask[];
		oncancel?: (id: string) => void;
		onretry?: (id: string) => void;
		ondismiss?: (id: string) => void;
	} = $props();

	const labels: Record<UploadTask["state"], string> = {
		reserving: "Preparing",
		sending: "Sending",
		finalizing: "Checking",
		done: "Attached",
		failed: "Did not upload",
		cancelled: "Stopped",
	};
</script>

{#if uploads.length > 0}
	<ul class="flex flex-col gap-2">
		{#each uploads as task (task.id)}
			<li class="flex flex-col gap-1 rounded-lg border border-line-subtle p-2">
				<div class="flex items-center gap-2">
					<span class="min-w-0 flex-1 truncate text-sm text-ink-900">{task.name}</span>
					<span class="font-mono text-xs text-muted-foreground">
						{labels[task.state]}
					</span>
					{#if task.state === "failed" && onretry}
						<Button variant="ghost" size="sm" onclick={() => onretry(task.id)}>Retry</Button>
					{/if}
					{#if settled(task) ? ondismiss : oncancel}
						<Button
							variant="ghost"
							size="sm"
							aria-label={settled(task) ? `Dismiss ${task.name}` : `Stop uploading ${task.name}`}
							onclick={() => (settled(task) ? ondismiss?.(task.id) : oncancel?.(task.id))}
						>
							<X aria-hidden="true" />
						</Button>
					{/if}
				</div>

				{#if task.state === "sending"}
					<Progress value={task.size > 0 ? (100 * task.sent) / task.size : 0} max={100} />
					<p class="font-mono text-2xs text-muted-foreground">
						{formatBytes(task.sent)} of {formatBytes(task.size)}
					</p>
				{:else if task.state === "reserving" || task.state === "finalizing"}
					<Progress max={100} />
				{:else if task.failure}
					<p class="text-xs text-destructive">{attachmentFailureMessage(task.failure)}</p>
				{/if}
			</li>
		{/each}
	</ul>
{/if}
