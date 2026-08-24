<script lang="ts">
	import { diffFailedLine, diffTruncatedLine, noDiffLine, type DiffView } from "./executions";

	let { view, download }: { view: DiffView; download: string } = $props();

	const tones = {
		add: "bg-success/10 text-ink-900",
		remove: "bg-destructive/10 text-ink-900",
		context: "text-muted-foreground",
	};

	const marks = { add: "+", remove: "−", context: " " };
</script>

{#if view.kind === "loading"}
	<p class="py-2 text-xs text-muted-foreground">Reading the diff…</p>
{:else if view.kind === "absent"}
	<p class="py-2 text-xs text-muted-foreground">{noDiffLine}</p>
{:else if view.kind === "failed"}
	<p class="py-2 text-xs text-muted-foreground">{view.message || diffFailedLine}</p>
{:else if view.kind === "ready"}
	<div class="flex min-w-0 flex-col gap-3 py-2">
		{#if view.files.length === 0}
			<p class="text-xs text-muted-foreground">The diff is empty.</p>
		{/if}

		{#each view.files as file (file.path)}
			<div class="flex min-w-0 flex-col gap-1">
				<div class="flex min-w-0 flex-wrap items-baseline gap-x-2 gap-y-0.5">
					<span class="min-w-0 font-mono text-xs break-all text-ink-900">{file.path}</span>
					<span class="font-mono text-2xs whitespace-nowrap text-muted-foreground">
						+{file.additions} −{file.deletions}
					</span>
				</div>

				{#if file.binary}
					<p class="text-2xs text-muted-foreground">A binary file, so there is nothing to read here.</p>
				{:else}
					<div class="overflow-x-auto rounded-sm bg-paper-1">
						{#each file.hunks as hunk (hunk.header)}
							<p class="px-2 py-1 font-mono text-2xs whitespace-pre text-muted-foreground">
								{hunk.header}
							</p>
							{#each hunk.lines as line, index (index)}
								<p class={`px-2 font-mono text-2xs whitespace-pre ${tones[line.kind]}`}>
									{marks[line.kind]}{line.text}
								</p>
							{/each}
						{/each}
					</div>
				{/if}
			</div>
		{/each}

		<p class="text-2xs text-muted-foreground">
			{#if view.truncated}{diffTruncatedLine}{/if}
			<a href={download} class="underline underline-offset-2 hover:text-foreground">
				Download the full diff
			</a>
		</p>
	</div>
{/if}
