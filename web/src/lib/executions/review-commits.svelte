<script lang="ts">
	import GitCommitHorizontal from "@lucide/svelte/icons/git-commit-horizontal";
	import { commitsLine, shortSha, type ReviewedRepository } from "./review";

	let { repository }: { repository: ReviewedRepository } = $props();
</script>

<details class="group min-w-0 text-xs">
	<summary class="cursor-pointer font-mono text-2xs text-muted-foreground hover:text-foreground">
		{shortSha(repository.baseSha)}..{shortSha(repository.headSha)} · {commitsLine(repository.commits)}
	</summary>
	<ol class="mt-1.5 flex min-w-0 flex-col gap-1">
		{#each repository.commits as commit (commit.sha)}
			<li class="flex min-w-0 items-baseline gap-2">
				<GitCommitHorizontal aria-hidden="true" class="size-3 shrink-0 self-center text-muted-foreground" />
				<span class="shrink-0 font-mono text-2xs text-muted-foreground">{shortSha(commit.sha)}</span>
				<span class="min-w-0 break-words text-ink-900">{commit.subject}</span>
			</li>
		{/each}
	</ol>
</details>
